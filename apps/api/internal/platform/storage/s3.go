package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

type s3Storage struct {
	client *s3.Client
	bucket string
}

func newS3(ctx context.Context, cfg config.Storage, createBucket bool) (*s3Storage, error) {
	if cfg.S3Bucket == "" || cfg.S3AccessKey == "" || cfg.S3SecretKey == "" {
		return nil, errors.New("storage: s3 driver needs S3_BUCKET, S3_ACCESS_KEY and S3_SECRET_KEY")
	}

	client := s3.NewFromConfig(aws.Config{
		Region:      cfg.S3Region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, ""),
	}, func(o *s3.Options) {
		if cfg.S3Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
		}
		o.UsePathStyle = cfg.S3PathStyle
	})

	st := &s3Storage{client: client, bucket: cfg.S3Bucket}
	if createBucket {
		if err := st.ensureBucket(ctx); err != nil {
			return nil, err
		}
	}
	return st, nil
}

func (s *s3Storage) ensureBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err == nil {
		return nil
	}
	var nf *types.NotFound
	if !errors.As(err, &nf) {
		return fmt.Errorf("storage: check bucket %q: %w", s.bucket, err)
	}
	if _, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(s.bucket)}); err != nil {
		return fmt.Errorf("storage: create bucket %q: %w", s.bucket, err)
	}
	return nil
}

func (s *s3Storage) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	// The SDK seeks the body to hash it and to retry, so a plain reader is read into memory first.
	seekable, ok := body.(io.ReadSeeker)
	if !ok {
		b, err := io.ReadAll(body)
		if err != nil {
			return fmt.Errorf("storage: read %q: %w", key, err)
		}
		seekable = bytes.NewReader(b)
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          seekable,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("storage: put %q: %w", key, err)
	}
	return nil
}

func (s *s3Storage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		var nk *types.NoSuchKey
		if errors.As(err, &nk) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("storage: get %q: %w", key, err)
	}
	return out.Body, nil
}

func (s *s3Storage) Delete(ctx context.Context, key string) error {
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)}); err != nil {
		return fmt.Errorf("storage: delete %q: %w", key, err)
	}
	return nil
}
