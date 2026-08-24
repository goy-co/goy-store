package goystore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// S3BlobStore implements the BlobStore interface using AWS S3 / MinIO.
type S3BlobStore struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
}

// NewS3BlobStore creates a new S3BlobStore instance.
func NewS3BlobStore(ctx context.Context, cfg BlobConfig) (*S3BlobStore, error) {
	bucket := cfg.Bucket
	if bucket == "" {
		bucket = "goy-store"
	}
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	var optFns []func(*config.LoadOptions) error
	optFns = append(optFns, config.WithRegion(region))

	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		creds := credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")
		optFns = append(optFns, config.WithCredentialsProvider(creds))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		if cfg.ForcePathStyle {
			o.UsePathStyle = true
		}
	})

	presignClient := s3.NewPresignClient(client)

	return &S3BlobStore{
		client:        client,
		presignClient: presignClient,
		bucket:        bucket,
	}, nil
}

// Client returns the underlying *s3.Client.
func (s *S3BlobStore) Client() *s3.Client {
	return s.client
}

// Put writes an object to S3 with optional metadata.
func (s *S3BlobStore) Put(ctx context.Context, key string, data []byte, metadata *Metadata) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	}

	if metadata != nil {
		if metadata.ContentType != "" {
			input.ContentType = aws.String(metadata.ContentType)
		}
		if len(metadata.Custom) > 0 {
			input.Metadata = make(map[string]string, len(metadata.Custom))
			for k, v := range metadata.Custom {
				input.Metadata[k] = v
			}
		}
	}

	_, err := s.client.PutObject(ctx, input)
	return err
}

// Get retrieves an object and its metadata from S3.
func (s *S3BlobStore) Get(ctx context.Context, key string) ([]byte, *Metadata, bool, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	output, err := s.client.GetObject(ctx, input)
	if err != nil {
		var nsk *types.NoSuchKey
		var apiErr smithy.APIError
		if errors.As(err, &nsk) || (errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchKey")) {
			return nil, nil, false, nil
		}
		return nil, nil, false, err
	}
	defer output.Body.Close()

	data, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to read object body: %w", err)
	}

	meta := &Metadata{
		Custom: output.Metadata,
	}
	if output.ContentType != nil {
		meta.ContentType = *output.ContentType
	}

	return data, meta, true, nil
}

// Delete removes an object from S3.
func (s *S3BlobStore) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

// List lists object keys matching an optional prefix.
func (s *S3BlobStore) List(ctx context.Context, prefix *string) ([]string, error) {
	var results []string
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: prefix,
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}
		for _, obj := range page.Contents {
			if obj.Key != nil {
				results = append(results, *obj.Key)
			}
		}
	}

	sort.Strings(results)
	return results, nil
}

// PresignURL generates a pre-signed URL for GET operations.
func (s *S3BlobStore) PresignURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	req, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("failed to presign get object: %w", err)
	}
	return req.URL, nil
}

// IsHealthy performs a lightweight HeadBucket check.
func (s *S3BlobStore) IsHealthy(ctx context.Context) (*HealthStatus, error) {
	start := time.Now()
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	_, err := s.client.HeadBucket(checkCtx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return &HealthStatus{
			Contract:  "blob",
			Backend:   "s3",
			State:     HealthUnhealthy,
			Message:   err.Error(),
			LatencyMS: latency,
		}, nil
	}

	return &HealthStatus{
		Contract:  "blob",
		Backend:   "s3",
		State:     HealthHealthy,
		LatencyMS: latency,
	}, nil
}
