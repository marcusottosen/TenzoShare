package backends

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
)

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
	input := &s3.PutObjectInput{
		Bucket:      aws.String(b.bucket),
		Key:         aws.String(key),
		Body:        data,
		ContentType: aws.String(contentType),
	}
	if size >= 0 {
		input.ContentLength = aws.Int64(size)
	}
	_, err := b.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("minio: upload %q: %w", key, err)
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
	disposition := fmt.Sprintf(`attachment; filename="%s"`, filename)
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
