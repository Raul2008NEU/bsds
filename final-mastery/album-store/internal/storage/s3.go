package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Client struct {
	s3       *s3.Client
	uploader *manager.Uploader
	presign  *s3.PresignClient
}

func NewClient(ctx context.Context) (*Client, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(getEnvOrDefault("AWS_REGION", "us-east-1")),
	}

	// Support localstack / custom endpoint (set AWS_ENDPOINT_URL for local dev)
	if endpoint := os.Getenv("AWS_ENDPOINT_URL"); endpoint != "" {
		opts = append(opts,
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
		)
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	var s3Opts []func(*s3.Options)
	if endpoint := os.Getenv("AWS_ENDPOINT_URL"); endpoint != "" {
		s3Opts = append(s3Opts,
			func(o *s3.Options) {
				o.BaseEndpoint = aws.String(endpoint)
				o.UsePathStyle = true // required for localstack
			},
		)
	}

	client := s3.NewFromConfig(cfg, s3Opts...)

	return &Client{
		s3: client,
		uploader: manager.NewUploader(client, func(u *manager.Uploader) {
			u.Concurrency = 20
		}),
		presign: s3.NewPresignClient(client),
	}, nil
}

// UploadPhoto streams a reader to S3 at the given key and returns the S3 location URL.
func (c *Client) UploadPhoto(ctx context.Context, bucket, key string, body io.Reader, contentType string) (string, error) {
	result, err := c.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("UploadPhoto: %w", err)
	}
	return result.Location, nil
}

// DeleteObject removes an object from S3.
func (c *Client) DeleteObject(ctx context.Context, bucket, key string) error {
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("DeleteObject: %w", err)
	}
	return nil
}

// PresignedURL generates a pre-signed GET URL valid for the given duration.
func (c *Client) PresignedURL(ctx context.Context, bucket, key string, duration time.Duration) (string, error) {
	req, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(duration))
	if err != nil {
		return "", fmt.Errorf("PresignedURL: %w", err)
	}
	return req.URL, nil
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
