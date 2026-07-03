package storage

import (
	"fmt"

	"github.com/google/uuid"
)

// AssetFallbackPath returns the fallback storage path for an asset when
// OriginalPath is empty. It constructs the path from the asset ID and
// original file name.
func AssetFallbackPath(assetID uuid.UUID, originalFileName string) string {
	return fmt.Sprintf("%s/%s", assetID.String(), originalFileName)
}
