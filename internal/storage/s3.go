package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// S3Backend implements StorageBackend using AWS S3 or S3-compatible storage
type S3Backend struct {
	config   S3Config
	client   *s3.Client
	uploader *manager.Uploader
}

// NewS3Backend creates a new S3 storage backend
func NewS3Backend(s3Config S3Config) (*S3Backend, error) {
	// Create AWS config
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(s3Config.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			s3Config.AccessKeyID,
			s3Config.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, &StorageError{
			Op:      "create s3 backend",
			Backend: "s3",
			Err:     fmt.Errorf("failed to load AWS config: %w", err),
		}
	}

	// Create S3 client with custom endpoint if specified
	var client *s3.Client
	if s3Config.Endpoint != "" {
		client = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(s3Config.Endpoint)
			o.UsePathStyle = s3Config.ForcePathStyle
		})
	} else {
		client = s3.NewFromConfig(cfg)
	}

	// Create uploader
	uploader := manager.NewUploader(client)

	backend := &S3Backend{
		config:   s3Config,
		client:   client,
		uploader: uploader,
	}

	// Test connection by checking if bucket exists
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := backend.testConnection(ctx); err != nil {
		return nil, &StorageError{
			Op:      "test s3 connection",
			Backend: "s3",
			Err:     err,
		}
	}

	return backend, nil
}

// testConnection tests if we can access the S3 bucket
func (s *S3Backend) testConnection(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "s3.testConnection")
	defer span.End()

	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.config.Bucket),
	})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to access S3 bucket %s: %w", s.config.Bucket, err)
	}

	return nil
}

// getObjectKey returns the full object key with path prefix
func (s *S3Backend) getObjectKey(path string) string {
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")

	if s.config.PathPrefix != "" {
		prefix := strings.TrimSuffix(s.config.PathPrefix, "/")
		return prefix + "/" + path
	}

	return path
}

// Upload uploads a file to S3
func (s *S3Backend) Upload(ctx context.Context, path string, reader io.Reader, size int64, contentType string) error {
	ctx, span := tracer.Start(ctx, "s3.Upload",
		trace.WithAttributes(
			attribute.String("storage.path", path),
			attribute.Int64("storage.size", size),
			attribute.String("storage.content_type", contentType),
		))
	defer span.End()

	key := s.getObjectKey(path)

	uploadInput := &s3.PutObjectInput{
		Bucket:        aws.String(s.config.Bucket),
		Key:           aws.String(key),
		Body:          reader,
		ContentLength: aws.Int64(size),
	}

	if contentType != "" {
		uploadInput.ContentType = aws.String(contentType)
	}

	_, err := s.uploader.Upload(ctx, uploadInput)
	if err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "upload",
			Path:    path,
			Backend: "s3",
			Err:     fmt.Errorf("failed to upload to S3: %w", err),
		}
	}

	return nil
}

// UploadBytes uploads byte data to S3
func (s *S3Backend) UploadBytes(ctx context.Context, path string, data []byte, contentType string) error {
	ctx, span := tracer.Start(ctx, "s3.UploadBytes",
		trace.WithAttributes(
			attribute.String("storage.path", path),
			attribute.String("storage.content_type", contentType),
			attribute.Int("storage.size", len(data)),
		))
	defer span.End()

	key := s.getObjectKey(path)

	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.config.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	}

	// Note: Server-side encryption can be added here if needed in the config

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "upload bytes",
			Path:    path,
			Backend: "s3",
			Err:     fmt.Errorf("failed to upload to S3: %w", err),
		}
	}

	return nil
}

// Download downloads a file from S3
func (s *S3Backend) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	ctx, span := tracer.Start(ctx, "s3.Download",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	key := s.getObjectKey(path)

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		span.RecordError(err)
		return nil, &StorageError{
			Op:      "download",
			Path:    path,
			Backend: "s3",
			Err:     fmt.Errorf("failed to download from S3: %w", err),
		}
	}

	return result.Body, nil
}

// Delete deletes a file from S3
func (s *S3Backend) Delete(ctx context.Context, path string) error {
	ctx, span := tracer.Start(ctx, "s3.Delete",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	key := s.getObjectKey(path)

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "delete",
			Path:    path,
			Backend: "s3",
			Err:     fmt.Errorf("failed to delete from S3: %w", err),
		}
	}

	return nil
}

// Exists checks if a file exists in S3
func (s *S3Backend) Exists(ctx context.Context, path string) (bool, error) {
	ctx, span := tracer.Start(ctx, "s3.Exists",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	key := s.getObjectKey(path)

	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// Check if it's a "not found" error
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}

		span.RecordError(err)
		return false, &StorageError{
			Op:      "exists",
			Path:    path,
			Backend: "s3",
			Err:     fmt.Errorf("failed to check if object exists: %w", err),
		}
	}

	return true, nil
}

// GetSize returns the size of a file in S3
func (s *S3Backend) GetSize(ctx context.Context, path string) (int64, error) {
	ctx, span := tracer.Start(ctx, "s3.GetSize",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	key := s.getObjectKey(path)

	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		span.RecordError(err)
		return 0, &StorageError{
			Op:      "get size",
			Path:    path,
			Backend: "s3",
			Err:     fmt.Errorf("failed to get object metadata: %w", err),
		}
	}

	return aws.ToInt64(result.ContentLength), nil
}

// GetPresignedUploadURL generates a pre-signed URL for uploading to S3
func (s *S3Backend) GetPresignedUploadURL(ctx context.Context, path string, contentType string, expiry time.Duration) (*PresignedURL, error) {
	ctx, span := tracer.Start(ctx, "s3.GetPresignedUploadURL",
		trace.WithAttributes(
			attribute.String("storage.path", path),
			attribute.String("storage.content_type", contentType),
			attribute.String("storage.expiry", expiry.String()),
		))
	defer span.End()

	key := s.getObjectKey(path)

	presigner := s3.NewPresignClient(s.client)

	putObjectInput := &s3.PutObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(key),
	}

	if contentType != "" {
		putObjectInput.ContentType = aws.String(contentType)
	}

	request, err := presigner.PresignPutObject(ctx, putObjectInput, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		span.RecordError(err)
		return nil, &StorageError{
			Op:      "get presigned upload URL",
			Path:    path,
			Backend: "s3",
			Err:     fmt.Errorf("failed to generate presigned upload URL: %w", err),
		}
	}

	headers := make(map[string]string)
	for k, v := range request.SignedHeader {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return &PresignedURL{
		URL:       request.URL,
		Method:    request.Method,
		Headers:   headers,
		ExpiresAt: time.Now().Add(expiry),
	}, nil
}

// GetPresignedDownloadURL generates a pre-signed URL for downloading from S3
func (s *S3Backend) GetPresignedDownloadURL(ctx context.Context, path string, expiry time.Duration) (*PresignedURL, error) {
	ctx, span := tracer.Start(ctx, "s3.GetPresignedDownloadURL",
		trace.WithAttributes(
			attribute.String("storage.path", path),
			attribute.String("storage.expiry", expiry.String()),
		))
	defer span.End()

	key := s.getObjectKey(path)

	presigner := s3.NewPresignClient(s.client)

	request, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		span.RecordError(err)
		return nil, &StorageError{
			Op:      "get presigned download URL",
			Path:    path,
			Backend: "s3",
			Err:     fmt.Errorf("failed to generate presigned download URL: %w", err),
		}
	}

	return &PresignedURL{
		URL:       request.URL,
		Method:    request.Method,
		ExpiresAt: time.Now().Add(expiry),
	}, nil
}

// SupportsPresignedURLs returns true for S3
func (s *S3Backend) SupportsPresignedURLs() bool {
	return true
}

// GetPublicURL returns a public URL for accessing the file (if bucket is public)
func (s *S3Backend) GetPublicURL(ctx context.Context, path string) (string, error) {
	key := s.getObjectKey(path)

	// Construct public URL
	var url string
	if s.config.Endpoint != "" {
		// Custom endpoint (e.g., MinIO)
		scheme := "https"
		if !s.config.UseSSL {
			scheme = "http"
		}

		if s.config.ForcePathStyle {
			url = fmt.Sprintf("%s://%s/%s/%s", scheme, s.config.Endpoint, s.config.Bucket, key)
		} else {
			url = fmt.Sprintf("%s://%s.%s/%s", scheme, s.config.Bucket, s.config.Endpoint, key)
		}
	} else {
		// AWS S3
		url = fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.config.Bucket, s.config.Region, key)
	}

	return url, nil
}

// Copy copies a file within S3
func (s *S3Backend) Copy(ctx context.Context, srcPath, dstPath string) error {
	ctx, span := tracer.Start(ctx, "s3.Copy",
		trace.WithAttributes(
			attribute.String("storage.src_path", srcPath),
			attribute.String("storage.dst_path", dstPath),
		))
	defer span.End()

	srcKey := s.getObjectKey(srcPath)
	dstKey := s.getObjectKey(dstPath)

	copySource := fmt.Sprintf("%s/%s", s.config.Bucket, srcKey)

	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.config.Bucket),
		Key:        aws.String(dstKey),
		CopySource: aws.String(copySource),
	})
	if err != nil {
		span.RecordError(err)
		return &StorageError{
			Op:      "copy",
			Path:    srcPath,
			Backend: "s3",
			Err:     fmt.Errorf("failed to copy object in S3: %w", err),
		}
	}

	return nil
}

// Move moves a file within S3 (copy + delete)
func (s *S3Backend) Move(ctx context.Context, srcPath, dstPath string) error {
	ctx, span := tracer.Start(ctx, "s3.Move",
		trace.WithAttributes(
			attribute.String("storage.src_path", srcPath),
			attribute.String("storage.dst_path", dstPath),
		))
	defer span.End()

	// Copy the object
	if err := s.Copy(ctx, srcPath, dstPath); err != nil {
		span.RecordError(err)
		return err
	}

	// Delete the source object
	if err := s.Delete(ctx, srcPath); err != nil {
		span.RecordError(err)
		return err
	}

	return nil
}

// List lists files in S3 with optional prefix filtering
func (s *S3Backend) List(ctx context.Context, prefix string, recursive bool) ([]FileInfo, error) {
	ctx, span := tracer.Start(ctx, "s3.List",
		trace.WithAttributes(
			attribute.String("storage.prefix", prefix),
			attribute.Bool("storage.recursive", recursive),
		))
	defer span.End()

	keyPrefix := s.getObjectKey(prefix)

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.config.Bucket),
		Prefix: aws.String(keyPrefix),
	}

	if !recursive {
		// Use delimiter to only get immediate children
		input.Delimiter = aws.String("/")
	}

	var files []FileInfo

	paginator := s3.NewListObjectsV2Paginator(s.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			span.RecordError(err)
			return nil, &StorageError{
				Op:      "list",
				Path:    prefix,
				Backend: "s3",
				Err:     fmt.Errorf("failed to list objects: %w", err),
			}
		}

		// Add objects
		for _, obj := range page.Contents {
			// Remove path prefix to get relative path
			path := aws.ToString(obj.Key)
			if s.config.PathPrefix != "" {
				path = strings.TrimPrefix(path, strings.TrimSuffix(s.config.PathPrefix, "/")+"/")
			}

			files = append(files, FileInfo{
				Path:    path,
				Size:    aws.ToInt64(obj.Size),
				ModTime: aws.ToTime(obj.LastModified),
				IsDir:   false,
				ETag:    strings.Trim(aws.ToString(obj.ETag), "\""),
			})
		}

		// Add common prefixes (directories) if not recursive
		if !recursive {
			for _, prefix := range page.CommonPrefixes {
				// Remove path prefix to get relative path
				path := aws.ToString(prefix.Prefix)
				if s.config.PathPrefix != "" {
					path = strings.TrimPrefix(path, strings.TrimSuffix(s.config.PathPrefix, "/")+"/")
				}

				// Remove trailing slash
				path = strings.TrimSuffix(path, "/")

				files = append(files, FileInfo{
					Path:  path,
					IsDir: true,
				})
			}
		}
	}

	return files, nil
}

// GetMetadata returns metadata about a file in S3
func (s *S3Backend) GetMetadata(ctx context.Context, path string) (*FileMetadata, error) {
	ctx, span := tracer.Start(ctx, "s3.GetMetadata",
		trace.WithAttributes(attribute.String("storage.path", path)))
	defer span.End()

	key := s.getObjectKey(path)

	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		span.RecordError(err)
		return nil, &StorageError{
			Op:      "get metadata",
			Path:    path,
			Backend: "s3",
			Err:     fmt.Errorf("failed to get object metadata: %w", err),
		}
	}

	metadata := &FileMetadata{
		Path:        path,
		Size:        aws.ToInt64(result.ContentLength),
		ModTime:     aws.ToTime(result.LastModified),
		ContentType: aws.ToString(result.ContentType),
		ETag:        strings.Trim(aws.ToString(result.ETag), "\""),
		Metadata:    make(map[string]string),
	}

	// Add user metadata
	for k, v := range result.Metadata {
		metadata.Metadata[k] = v
	}

	return metadata, nil
}

// Close closes the S3 backend
func (s *S3Backend) Close() error {
	// Nothing to close for S3
	return nil
}
