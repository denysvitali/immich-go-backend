package admin

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/grpcutil"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

// storageFolderToProto maps a storage folder name to the proto enum.
func storageFolderToProto(folder string) immichv1.StorageFolder {
	switch folder {
	case "encoded-video":
		return immichv1.StorageFolder_STORAGE_FOLDER_ENCODED_VIDEO
	case "library":
		return immichv1.StorageFolder_STORAGE_FOLDER_LIBRARY
	case "upload":
		return immichv1.StorageFolder_STORAGE_FOLDER_UPLOAD
	case "profile":
		return immichv1.StorageFolder_STORAGE_FOLDER_PROFILE
	case "thumbs":
		return immichv1.StorageFolder_STORAGE_FOLDER_THUMBS
	case "backups":
		return immichv1.StorageFolder_STORAGE_FOLDER_BACKUPS
	default:
		return immichv1.StorageFolder_STORAGE_FOLDER_UNSPECIFIED
	}
}

// DetectPriorInstall reports the state of the standard storage folders so an
// installer can detect artifacts from a prior installation.
func (s *Server) DetectPriorInstall(ctx context.Context, _ *emptypb.Empty) (*immichv1.MaintenanceDetectInstallResponseDto, error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	statuses, err := s.service.DetectPriorInstall(ctx)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to detect prior install", err)
	}

	storage := make([]*immichv1.MaintenanceDetectInstallStorageFolderDto, 0, len(statuses))
	for _, st := range statuses {
		storage = append(storage, &immichv1.MaintenanceDetectInstallStorageFolderDto{
			Folder:   storageFolderToProto(st.Folder),
			Files:    st.Files,
			Readable: st.Readable,
			Writable: st.Writable,
		})
	}

	return &immichv1.MaintenanceDetectInstallResponseDto{Storage: storage}, nil
}

// ListDatabaseBackups lists all recorded database backups.
func (s *Server) ListDatabaseBackups(ctx context.Context, _ *emptypb.Empty) (*immichv1.DatabaseBackupListResponseDto, error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	backups, err := s.service.ListDatabaseBackups(ctx)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to list database backups", err)
	}

	dtos := make([]*immichv1.DatabaseBackupDto, 0, len(backups))
	for _, backup := range backups {
		dtos = append(dtos, &immichv1.DatabaseBackupDto{
			Filename: backup.Filename,
			Filesize: backup.Filesize,
			Timezone: backup.Timezone,
		})
	}

	return &immichv1.DatabaseBackupListResponseDto{Backups: dtos}, nil
}

// DeleteDatabaseBackup deletes the given database backups.
func (s *Server) DeleteDatabaseBackup(ctx context.Context, request *immichv1.DatabaseBackupDeleteDto) (*emptypb.Empty, error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	if err := s.service.DeleteDatabaseBackups(ctx, request.GetBackups()); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, "backup not found")
		}
		if strings.Contains(err.Error(), "invalid") {
			return nil, status.Error(codes.InvalidArgument, "invalid backup filename")
		}
		return nil, grpcutil.SanitizedInternal(ctx, "failed to delete database backups", err)
	}

	return &emptypb.Empty{}, nil
}

// UploadDatabaseBackup stores an uploaded database backup.
func (s *Server) UploadDatabaseBackup(ctx context.Context, request *immichv1.DatabaseBackupUploadDto) (*emptypb.Empty, error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	if err := s.service.UploadDatabaseBackup(ctx, request.GetFilename(), request.GetData()); err != nil {
		if strings.Contains(err.Error(), "invalid") {
			return nil, status.Error(codes.InvalidArgument, "invalid backup filename")
		}
		return nil, grpcutil.SanitizedInternal(ctx, "failed to upload database backup", err)
	}

	return &emptypb.Empty{}, nil
}

// DownloadDatabaseBackup returns the contents of a stored database backup.
func (s *Server) DownloadDatabaseBackup(ctx context.Context, request *immichv1.DownloadDatabaseBackupRequest) (*immichv1.DownloadDatabaseBackupResponse, error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	data, err := s.service.DownloadDatabaseBackup(ctx, request.GetFilename())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, "backup not found")
		}
		if strings.Contains(err.Error(), "invalid") {
			return nil, status.Error(codes.InvalidArgument, "invalid backup filename")
		}
		return nil, grpcutil.SanitizedInternal(ctx, "failed to download database backup", err)
	}

	return &immichv1.DownloadDatabaseBackupResponse{
		Data:        data,
		ContentType: "application/gzip",
		Filename:    request.GetFilename(),
	}, nil
}

// StartDatabaseRestoreFlow records a pending database restore request.
func (s *Server) StartDatabaseRestoreFlow(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	if err := s.service.StartDatabaseRestoreFlow(ctx); err != nil {
		if strings.Contains(err.Error(), "no database backups") {
			return nil, status.Error(codes.FailedPrecondition, "no database backups available to restore")
		}
		return nil, grpcutil.SanitizedInternal(ctx, "failed to start database restore flow", err)
	}

	return &emptypb.Empty{}, nil
}
