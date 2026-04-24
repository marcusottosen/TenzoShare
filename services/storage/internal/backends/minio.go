package backends

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/tenzoshare/tenzoshare/shared/pkg/config"
)

// MinIOBackend implements shared/pkg/storage.Backend using the AWS SDK v2 against MinIO.
type MinIOBackend struct {
	client *s3.Client
	bucket string
}

func NewMinIO(ctx context.Context, cfg *config.Config) (*MinIOBackend, error) {
	endpoint := cfg.S3.Endpoint
	if cfg.S3.UseSSL && len(endpoint) >= 7 && endpoint[:7] == "http://" {
		endpoint = "https://" + endpoint[7:]
	} else if !cfg.S3.UseSSL && len(endpoint) >= 8 && endpoint[:8] == "https://" {
		endpoint = "http://" + endpoint[8:]
	}

	customResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...any) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               endpoint,
				SigningRegion:     cfg.S3.Region,
				HostnameImmutable: true, // required for MinIO path-style URLs
			}, nil
		},
	)

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.S3.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.S3.AccessKey, cfg.S3.SecretKey, "",
		)),
		awsconfig.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return nil, fmt.Errorf("minio: load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true // MinIO requires path-style (not virtual-hosted)
	})

	b := &MinIOBackend{client: client, bucket: cfg.S3.Bucket}
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

func (b *MinIOBackend) GetPresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	presign := s3.NewPresignClient(b.client)
	req, err := presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
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
