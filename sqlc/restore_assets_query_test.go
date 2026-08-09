package sqlc_test

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestoreAssetsQueryIsDeletionSafeCompareAndSet(t *testing.T) {
	queries, err := os.ReadFile("queries.sql")
	require.NoError(t, err)

	restoreQuery := regexp.MustCompile(`(?s)-- name: RestoreAssets :execrows\s+(.*?)\s+-- name: PermanentlyDeleteAssets`).FindSubmatch(queries)
	require.Len(t, restoreQuery, 2, "RestoreAssets must report the rows changed by its compare-and-set")

	assert.Regexp(t, regexp.MustCompile(`(?s)WHERE\s+id\s*=\s*ANY\(\$1::uuid\[\]\).*"ownerId"\s*=\s*\$2`), string(restoreQuery[1]))
	assert.Regexp(t, regexp.MustCompile(`"deletedAt"\s+IS\s+NULL`), string(restoreQuery[1]))
	assert.Regexp(t, regexp.MustCompile(`status\s*=\s*'trashed'::assets_status_enum`), string(restoreQuery[1]))
	assert.NotRegexp(t, regexp.MustCompile(`status\s*!=\s*'deleted'`), string(restoreQuery[1]),
		"restore must positively compare-and-set from trashed, not merely exclude one terminal state")
}
