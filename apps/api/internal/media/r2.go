package media

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// R2 stores objects in a Cloudflare R2 bucket through its S3 API.
type R2 struct {
	client *s3.Client
	bucket string
}

// NewR2 returns an R2 store, or nil when any credential is missing so callers
// can treat "not configured" explicitly.
func NewR2(accountID, accessKeyID, secretAccessKey, bucket string) *R2 {
	if accountID == "" || accessKeyID == "" || secretAccessKey == "" || bucket == "" {
		return nil
	}
	return newR2WithEndpoint("https://"+accountID+".r2.cloudflarestorage.com", accessKeyID, secretAccessKey, bucket)
}

func newR2WithEndpoint(endpoint, accessKeyID, secretAccessKey, bucket string) *R2 {
	client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		UsePathStyle: true,
		// R2 does not implement the newer default CRC checksums.
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
	return &R2{client: client, bucket: bucket}
}

// Put uploads body under key.
func (r *R2) Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error {
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(r.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
		CacheControl:  aws.String("public, max-age=31536000, immutable"),
	})
	if err != nil {
		return fmt.Errorf("r2 put %s: %w", key, err)
	}
	return nil
}

// Delete removes key. S3 DeleteObject succeeds for missing keys.
func (r *R2) Delete(ctx context.Context, key string) error {
	if _, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("r2 delete %s: %w", key, err)
	}
	return nil
}
