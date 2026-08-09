package db

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const immichPostgresImagePrefix = "ghcr.io/immich-app/postgres:"

type composeConfig struct {
	Services map[string]struct {
		Image       string `yaml:"image"`
		Healthcheck struct {
			Test []string `yaml:"test"`
		} `yaml:"healthcheck"`
	} `yaml:"services"`
}

// TestComposeDatabaseSupportsMigrationExtensions is an offline guard for the
// local quickstart. It catches regressions to a stock Postgres image and keeps
// the runtime health check synchronized with extensions created by migrations.
func TestComposeDatabaseSupportsMigrationExtensions(t *testing.T) {
	composeYAML, err := os.ReadFile(filepath.Join("..", "..", "docker-compose.yml"))
	require.NoError(t, err)

	var compose composeConfig
	require.NoError(t, yaml.Unmarshal(composeYAML, &compose))
	postgres, ok := compose.Services["postgres"]
	require.True(t, ok, "docker-compose.yml must define a postgres service")

	assert.True(t, strings.HasPrefix(postgres.Image, immichPostgresImagePrefix),
		"the stock Postgres image does not include VectorChord; use the Immich PostgreSQL image")
	assert.Regexp(t, regexp.MustCompile(`vectorchord[^@]*@sha256:[0-9a-f]{64}$`), postgres.Image,
		"the PostgreSQL image must identify VectorChord and be pinned by digest")

	required := migrationExtensions(t)
	assert.Equal(t,
		[]string{"cube", "earthdistance", "pg_trgm", "unaccent", "uuid-ossp", "vchord", "vector"},
		required,
		"update the Compose image and health check when migration extension requirements change",
	)

	healthcheck := strings.Join(postgres.Healthcheck.Test, " ")
	for _, extension := range required {
		assert.Contains(t, healthcheck, "'"+extension+"'",
			"the Compose health check must verify that %s is available", extension)
	}
}

func migrationExtensions(t *testing.T) []string {
	t.Helper()

	migrations, err := loadMigrations()
	require.NoError(t, err)

	extensionPattern := regexp.MustCompile(`(?i)CREATE\s+EXTENSION\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:"([^"]+)"|([a-z0-9_-]+))`)
	seen := make(map[string]struct{})
	for _, migration := range migrations {
		for _, match := range extensionPattern.FindAllStringSubmatch(migration.SQL, -1) {
			name := match[1]
			if name == "" {
				name = match[2]
			}
			seen[strings.ToLower(name)] = struct{}{}
		}
	}

	extensions := make([]string, 0, len(seen))
	for extension := range seen {
		extensions = append(extensions, extension)
	}
	sort.Strings(extensions)
	return extensions
}
