package admin

import (
	"testing"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/stretchr/testify/assert"
)

func TestValidBackupFilename(t *testing.T) {
	assert.True(t, validBackupFilename("backup-2026-07-04.sql.gz"))
	assert.False(t, validBackupFilename(""))
	assert.False(t, validBackupFilename("../etc/passwd"))
	assert.False(t, validBackupFilename("a/b.sql.gz"))
	assert.False(t, validBackupFilename(`a\b.sql.gz`))
}

func TestStorageFolderToProto(t *testing.T) {
	cases := map[string]immichv1.StorageFolder{
		"encoded-video": immichv1.StorageFolder_STORAGE_FOLDER_ENCODED_VIDEO,
		"library":       immichv1.StorageFolder_STORAGE_FOLDER_LIBRARY,
		"upload":        immichv1.StorageFolder_STORAGE_FOLDER_UPLOAD,
		"profile":       immichv1.StorageFolder_STORAGE_FOLDER_PROFILE,
		"thumbs":        immichv1.StorageFolder_STORAGE_FOLDER_THUMBS,
		"backups":       immichv1.StorageFolder_STORAGE_FOLDER_BACKUPS,
		"other":         immichv1.StorageFolder_STORAGE_FOLDER_UNSPECIFIED,
	}
	for name, want := range cases {
		assert.Equal(t, want, storageFolderToProto(name), name)
	}
}
