package services

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Storage struct {
	bucket        string
	client        *s3.Client
	presignClient *s3.PresignClient
	uploader      *manager.Uploader
}

type S3StorageConfig struct {
	BaseURL        string
	Endpoint       string
	PublicEndpoint string
	Region         string
	AccessKey      string
	SecretKey      string
	Bucket         string
	UsePathStyle   bool
}

func NewS3Storage(ctx context.Context, cfg S3StorageConfig) (*S3Storage, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("s3 endpoint is required")
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("s3 credentials are required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}

	client, err := newS3Client(cfg.Endpoint, cfg.Region, cfg.AccessKey, cfg.SecretKey, cfg.UsePathStyle)
	if err != nil {
		return nil, err
	}

	publicEndpoint := cfg.PublicEndpoint
	if publicEndpoint == "" {
		publicEndpoint = cfg.Endpoint
	}
	presignClientBase, err := newS3Client(publicEndpoint, cfg.Region, cfg.AccessKey, cfg.SecretKey, cfg.UsePathStyle)
	if err != nil {
		return nil, err
	}

	storage := &S3Storage{
		bucket:        cfg.Bucket,
		client:        client,
		presignClient: s3.NewPresignClient(presignClientBase),
		uploader:      manager.NewUploader(client),
	}
	if err := storage.ensureBucket(ctx); err != nil {
		return nil, err
	}
	return storage, nil
}

func newS3Client(endpoint, region, accessKey, secretKey string, usePathStyle bool) (*s3.Client, error) {
	resolver := s3.EndpointResolverFromURL(endpoint)
	cfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = usePathStyle
		o.EndpointResolver = resolver
	}), nil
}

func (s *S3Storage) ensureBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err == nil {
		return nil
	}
	_, createErr := s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(s.bucket)})
	if createErr != nil {
		return fmt.Errorf("ensure bucket %q: head=%v create=%w", s.bucket, err, createErr)
	}
	return nil
}

func (s *S3Storage) Upload(ctx context.Context, key string, r io.Reader, contentType string) error {
	clean, err := CleanStorageKey(key)
	if err != nil {
		return err
	}
	_, err = s.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(clean),
		Body:        r,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("put s3 object: %w", err)
	}
	return nil
}

func (s *S3Storage) Download(ctx context.Context, key string) ([]byte, error) {
	clean, err := CleanStorageKey(key)
	if err != nil {
		return nil, err
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(clean),
	})
	if err != nil {
		return nil, fmt.Errorf("get s3 object: %w", err)
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func (s *S3Storage) GetURL(ctx context.Context, key string) (string, error) {
	clean, err := CleanStorageKey(key)
	if err != nil {
		return "", err
	}
	presigned, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucket),
		Key:                        aws.String(clean),
		ResponseContentDisposition: aws.String(`attachment; filename="` + pathBase(clean) + `"`),
	}, func(o *s3.PresignOptions) {
		o.Expires = 24 * time.Hour
	})
	if err != nil {
		return "", fmt.Errorf("presign get object: %w", err)
	}
	return presigned.URL, nil
}

func (s *S3Storage) Delete(ctx context.Context, key string) error {
	clean, err := CleanStorageKey(key)
	if err != nil {
		return err
	}
	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(clean),
	})
	if err != nil {
		return fmt.Errorf("delete s3 object: %w", err)
	}
	return nil
}

func pathBase(key string) string {
	if i := strings.LastIndexByte(key, '/'); i >= 0 {
		return key[i+1:]
	}
	return key
}
