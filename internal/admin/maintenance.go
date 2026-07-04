package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
)

// storageFolderNames are the upstream Immich storage folders that a prior
// install would have created under the storage root.
var storageFolderNames = []string{
	"encoded-video",
	"library",
	"upload",
	"profile",
	"thumbs",
	"backups",
}

// StorageFolderStatus describes one storage folder inspected by
// DetectPriorInstall.
type StorageFolderStatus struct {
	Folder   string
	Files    int64
	Readable bool
	Writable bool
}

// DatabaseBackupInfo describes a stored database backup.
type DatabaseBackupInfo struct {
	Filename  string
	Filesize  int64
	Timezone  string
	CreatedAt time.Time
}

// backupsDir returns the directory where database backups are stored.
func (s *Service) backupStoragePath(filename string) string {
	return path.Join("backups", filename)
}

func validBackupFilename(filename string) bool {
	return filename != "" &&
		filename == path.Base(filename) &&
		!strings.ContainsAny(filename, `/\`)
}

// DetectPriorInstall inspects the configured storage root for the standard
// Immich storage folders and reports file counts and access permissions,
// which the web installer uses to detect a prior installation.
func (s *Service) DetectPriorInstall(ctx context.Context) ([]StorageFolderStatus, error) {
	_, span := tracer.Start(ctx, "admin.detect_prior_install")
	defer span.End()

	statuses := make([]StorageFolderStatus, 0, len(storageFolderNames))
	for _, folder := range storageFolderNames {
		st := StorageFolderStatus{Folder: folder}

		entries, err := s.storage.List(ctx, folder, true)
		if err == nil {
			st.Readable = true
			st.Files = int64(len(entries))
		}

		probePath := path.Join(folder, ".immich-go-write-check")
		if err := s.storage.UploadBytes(ctx, probePath, []byte("ok"), "text/plain"); err == nil {
			st.Writable = true
			_ = s.storage.Delete(ctx, probePath)
		}

		statuses = append(statuses, st)
	}

	return statuses, nil
}

// ListDatabaseBackups returns all recorded database backups.
func (s *Service) ListDatabaseBackups(ctx context.Context) ([]DatabaseBackupInfo, error) {
	ctx, span := tracer.Start(ctx, "admin.list_database_backups")
	defer span.End()

	rows, err := s.db.ListDatabaseBackups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list database backups: %w", err)
	}

	backups := make([]DatabaseBackupInfo, 0, len(rows))
	for _, row := range rows {
		backups = append(backups, DatabaseBackupInfo{
			Filename:  row.Filename,
			Filesize:  row.Filesize,
			Timezone:  row.Timezone,
			CreatedAt: row.CreatedAt.Time,
		})
	}
	return backups, nil
}

// UploadDatabaseBackup stores an uploaded backup file on disk and records it
// in the database.
func (s *Service) UploadDatabaseBackup(ctx context.Context, filename string, data []byte) error {
	ctx, span := tracer.Start(ctx, "admin.upload_database_backup")
	defer span.End()

	if !validBackupFilename(filename) {
		return fmt.Errorf("invalid backup filename")
	}

	storagePath := s.backupStoragePath(filename)
	if err := s.storage.UploadBytes(ctx, storagePath, data, "application/gzip"); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	_, err := s.db.UpsertDatabaseBackup(ctx, sqlc.UpsertDatabaseBackupParams{
		Filename: filename,
		Path:     storagePath,
		Filesize: int64(len(data)),
		Timezone: time.Now().Location().String(),
	})
	if err != nil {
		return fmt.Errorf("failed to record backup: %w", err)
	}
	return nil
}

// DownloadDatabaseBackup reads a stored backup file from disk.
func (s *Service) DownloadDatabaseBackup(ctx context.Context, filename string) ([]byte, error) {
	ctx, span := tracer.Start(ctx, "admin.download_database_backup")
	defer span.End()

	if !validBackupFilename(filename) {
		return nil, fmt.Errorf("invalid backup filename")
	}

	backup, err := s.db.GetDatabaseBackup(ctx, filename)
	if err != nil {
		return nil, fmt.Errorf("backup not found: %w", err)
	}

	reader, err := s.storage.Download(ctx, backup.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup file: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup file: %w", err)
	}
	return data, nil
}

// DeleteDatabaseBackups deletes the given backups from disk and the database.
func (s *Service) DeleteDatabaseBackups(ctx context.Context, filenames []string) error {
	ctx, span := tracer.Start(ctx, "admin.delete_database_backups")
	defer span.End()

	for _, filename := range filenames {
		if !validBackupFilename(filename) {
			return fmt.Errorf("invalid backup filename %q", filename)
		}

		backup, err := s.db.GetDatabaseBackup(ctx, filename)
		if err != nil {
			return fmt.Errorf("backup %q not found: %w", filename, err)
		}

		if err := s.storage.Delete(ctx, backup.Path); err != nil {
			return fmt.Errorf("failed to delete backup file %q: %w", filename, err)
		}

		if err := s.db.DeleteDatabaseBackup(ctx, filename); err != nil {
			return fmt.Errorf("failed to delete backup record %q: %w", filename, err)
		}
	}
	return nil
}

// StartDatabaseRestoreFlow records a pending database restore request in the
// system metadata table. The actual restore is performed on the next server
// start in maintenance mode, mirroring upstream's restart-based flow.
func (s *Service) StartDatabaseRestoreFlow(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "admin.start_database_restore_flow")
	defer span.End()

	backups, err := s.db.ListDatabaseBackups(ctx)
	if err != nil {
		return fmt.Errorf("failed to list database backups: %w", err)
	}
	if len(backups) == 0 {
		return fmt.Errorf("no database backups available to restore")
	}

	value, err := json.Marshal(map[string]any{
		"requestedAt": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("failed to encode restore request: %w", err)
	}

	if _, err := s.db.SetSystemMetadata(ctx, sqlc.SetSystemMetadataParams{
		Key:   "database-restore-request",
		Value: value,
	}); err != nil {
		return fmt.Errorf("failed to record restore request: %w", err)
	}
	return nil
}
