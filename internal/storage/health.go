package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

type backendHealthChecker interface {
	HealthCheck(context.Context) error
}

// HealthCheck verifies that the configured backend is reachable without
// relying on a cached startup result. Built-in backends use a lightweight,
// read-only operation; custom backends fall back to listing their root.
func (s *Service) HealthCheck(ctx context.Context) error {
	if checker, ok := s.backend.(backendHealthChecker); ok {
		return checker.HealthCheck(ctx)
	}
	_, err := s.backend.List(ctx, "", false)
	return err
}

// HealthCheck verifies that the local root still exists and can be read.
func (l *LocalBackend) HealthCheck(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	directory, err := os.Open(l.rootPath)
	if err != nil {
		return fmt.Errorf("open storage root: %w", err)
	}
	defer directory.Close()

	_, err = directory.Readdirnames(1)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read storage root: %w", err)
	}
	return nil
}

// HealthCheck verifies that the configured bucket is reachable.
func (s *S3Backend) HealthCheck(ctx context.Context) error {
	return s.testConnection(ctx)
}

// HealthCheck verifies that the configured rclone remote is reachable.
func (r *RcloneBackend) HealthCheck(ctx context.Context) error {
	return r.testConnection(ctx)
}
