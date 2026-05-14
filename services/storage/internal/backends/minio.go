package backends

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
)

// multipartPartSize is the size of each part in a multipart upload.
// MinIO/S3 requires every non-final part to be ≥ 5 MiB; the last part may be any size.
// 64 MiB per part × 10,000 parts (S3 maximum) = 640 GiB max object size.
const multipartPartSize = 64 * 1024 * 1024 // 64 MiB

// MinIOBackend implements shared/pkg/storage.Backend using the AWS SDK v2 against MinIO.
type MinIOBackend struct {
	client        *s3.Client
	presignClient *s3.PresignClient // may point at publicURL so signature matches public host
	bucket        string
}

// newS3Client creates an s3.Client pointed at the given endpoint.
func newS3Client(ctx context.Context, endpoint, region, accessKey, secretKey string) (*s3.Client, error) {
	resolver := aws.EndpointResolverWithOptionsFunc(
		func(service, reg string, options ...any) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               endpoint,
				SigningRegion:     region,
				HostnameImmutable: true,
			}, nil
		},
	)
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		awsconfig.WithEndpointResolverWithOptions(resolver),
		// Disable proactive checksum calculation.
		// AWS SDK v2 ≥ v1.32 defaults to RequestChecksumCalculationWhenSupported,
		// which requires either a seekable reader (to pre-compute) or TLS (for
		// trailing checksums sent after the body). MinIO is accessed over plain
		// HTTP inside Docker and we pass unseekable streaming readers, so we
		// must set WhenRequired so checksums are only added when the S3 API
		// explicitly demands them (which PutObject does not).
		awsconfig.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
		awsconfig.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired),
	)
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
		// PutObject over plain HTTP with a non-seekable streaming reader:
		// the SDK's dynamic payload signer falls back to ComputePayloadSHA256
		// which requires seeking the reader. Use UNSIGNED-PAYLOAD instead,
		// which is what the SDK already does on HTTPS — this makes HTTP
		// behaviour match and allows unbuffered streaming uploads.
		o.APIOptions = append(o.APIOptions, v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware)
	}), nil
}

func NewMinIO(ctx context.Context, cfg *config.Config) (*MinIOBackend, error) {
	client, err := newS3Client(ctx, cfg.S3.Endpoint, cfg.S3.Region, cfg.S3.AccessKey, cfg.S3.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("minio: load aws config: %w", err)
	}

	// For presigning, use the public URL if set so the signature is computed
	// against the host that the browser will actually connect to.
	// If S3_PUBLIC_URL is unset, fall back to the internal endpoint.
	presignEndpoint := cfg.S3.Endpoint
	if cfg.S3.PublicURL != "" {
		presignEndpoint = cfg.S3.PublicURL
	}
	presignS3, err := newS3Client(ctx, presignEndpoint, cfg.S3.Region, cfg.S3.AccessKey, cfg.S3.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("minio: load presign aws config: %w", err)
	}

	b := &MinIOBackend{
		client:        client,
		presignClient: s3.NewPresignClient(presignS3),
		bucket:        cfg.S3.Bucket,
	}
	if err := b.ensureBucket(ctx); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *MinIOBackend) Upload(ctx context.Context, key string, data io.Reader, size int64, contentType string) error {
	if size >= 0 {
		// Known size: direct PutObject (caller pre-computed the exact byte count).
		input := &s3.PutObjectInput{
			Bucket:        aws.String(b.bucket),
			Key:           aws.String(key),
			Body:          data,
			ContentType:   aws.String(contentType),
			ContentLength: aws.Int64(size),
		}
		_, err := b.client.PutObject(ctx, input)
		if err != nil {
			return fmt.Errorf("minio: upload %q: %w", key, err)
		}
		return nil
	}
	// Unknown size (streaming): use multipart upload.
	// A single PutObject with no Content-Length keeps one HTTP connection open
	// for the entire duration of the upload, which is fragile for large files
	// (proxy timeouts, transient network errors restart the whole transfer).
	// Multipart upload breaks the stream into 64 MiB parts, each its own short
	// HTTP request, supporting up to 640 GiB with constant ~64 MiB RAM usage.
	return b.multipartUpload(ctx, key, data, contentType)
}

// multipartUpload streams data to MinIO using the S3 multipart upload API.
// Parts are read into a fixed-size in-memory buffer so the overall RAM usage
// is bounded to multipartPartSize bytes regardless of the total object size.
func (b *MinIOBackend) multipartUpload(ctx context.Context, key string, data io.Reader, contentType string) error {
	create, err := b.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(b.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("minio: create multipart upload %q: %w", key, err)
	}
	uploadID := aws.ToString(create.UploadId)

	abort := func() {
		// Use a background context so the abort still fires even if ctx was cancelled.
		_, _ = b.client.AbortMultipartUpload(context.Background(), &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(b.bucket),
			Key:      aws.String(key),
			UploadId: aws.String(uploadID),
		})
	}

	buf := make([]byte, multipartPartSize)
	var parts []s3types.CompletedPart
	partNum := int32(1)

	for {
		// io.ReadFull fills buf completely, or returns:
		//   (n>0, io.ErrUnexpectedEOF) — last partial chunk (end of stream)
		//   (0,   io.EOF)               — stream was already exhausted
		n, readErr := io.ReadFull(data, buf)
		if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
			abort()
			return fmt.Errorf("minio: read part %d for %q: %w", partNum, key, readErr)
		}
		if n == 0 {
			break // nothing left to upload
		}

		// bytes.NewReader is seekable, so the SDK can retry if needed,
		// and ContentLength is explicit so MinIO never falls back to chunked TE.
		up, err := b.client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:        aws.String(b.bucket),
			Key:           aws.String(key),
			UploadId:      aws.String(uploadID),
			PartNumber:    aws.Int32(partNum),
			Body:          bytes.NewReader(buf[:n]),
			ContentLength: aws.Int64(int64(n)),
		})
		if err != nil {
			abort()
			return fmt.Errorf("minio: upload part %d for %q: %w", partNum, key, err)
		}

		parts = append(parts, s3types.CompletedPart{
			ETag:       up.ETag,
			PartNumber: aws.Int32(partNum),
		})
		partNum++

		if readErr == io.ErrUnexpectedEOF || readErr == io.EOF {
			break // last chunk uploaded, done
		}
	}

	if len(parts) == 0 {
		// Empty body: multipart upload requires at least one part, so fall back
		// to a zero-byte PutObject and clean up the abandoned upload ID.
		abort()
		_, err = b.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:        aws.String(b.bucket),
			Key:           aws.String(key),
			ContentType:   aws.String(contentType),
			ContentLength: aws.Int64(0),
		})
		if err != nil {
			return fmt.Errorf("minio: upload empty object %q: %w", key, err)
		}
		return nil
	}

	_, err = b.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(b.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &s3types.CompletedMultipartUpload{
			Parts: parts,
		},
	})
	if err != nil {
		abort()
		return fmt.Errorf("minio: complete multipart upload %q: %w", key, err)
	}
	return nil
}

func (b *MinIOBackend) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := b.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("minio: download %q: %w", key, err)
	}
	return out.Body, nil
}

func (b *MinIOBackend) Delete(ctx context.Context, key string) error {
	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("minio: delete %q: %w", key, err)
	}
	return nil
}

func (b *MinIOBackend) GetPresignedURL(ctx context.Context, key, filename string, expiry time.Duration) (string, error) {
	safeFilename := strings.ReplaceAll(filename, `\`, `\\`)
	safeFilename = strings.ReplaceAll(safeFilename, `"`, `\"`)
	disposition := fmt.Sprintf(`attachment; filename="%s"`, safeFilename)
	req, err := b.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(b.bucket),
		Key:                        aws.String(key),
		ResponseContentDisposition: aws.String(disposition),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("minio: presign %q: %w", key, err)
	}
	return req.URL, nil
}

func (b *MinIOBackend) Exists(ctx context.Context, key string) (bool, error) {
	_, err := b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// AWS SDK returns a NoSuchKey or 404 error — treat as "not exists"
		return false, nil
	}
	return true, nil
}

func (b *MinIOBackend) ensureBucket(ctx context.Context) error {
	_, err := b.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(b.bucket),
	})
	if err == nil {
		return nil // already exists
	}
	_, err = b.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(b.bucket),
	})
	if err != nil {
		return fmt.Errorf("minio: create bucket %q: %w", b.bucket, err)
	}
	return nil
}
