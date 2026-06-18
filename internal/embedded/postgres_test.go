package embedded

import (
	"strings"
	"testing"
)

func TestIsEnabled_DefaultsFalse(t *testing.T) {
	t.Setenv("IMMICH_EMBEDDED_DB", "")
	if IsEnabled() {
		t.Fatal("IsEnabled should be false when env var is empty")
	}
}

func TestIsEnabled_Truthy(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "True", "yes", "YES", "Yes"} {
		t.Setenv("IMMICH_EMBEDDED_DB", v)
		if !IsEnabled() {
			t.Fatalf("IsEnabled() should be true for %q", v)
		}
	}
}

func TestIsEnabled_FalsyButPresent(t *testing.T) {
	for _, v := range []string{"0", "false", "no", "off", "anything"} {
		t.Setenv("IMMICH_EMBEDDED_DB", v)
		if IsEnabled() {
			t.Fatalf("IsEnabled() should be false for %q", v)
		}
	}
}

func TestRedactDSN(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{
			"postgres://immich:supersecret@127.0.0.1:5432/immich?sslmode=disable",
			"postgres://immich:***@127.0.0.1:5432/immich?sslmode=disable",
		},
		{
			"postgres://u:p@h:1/d",
			"postgres://u:***@h:1/d",
		},
		// No password present — return as-is.
		{
			"postgres://h:1/d",
			"postgres://h:1/d",
		},
		// Garbage without scheme — return as-is.
		{
			"not a dsn",
			"not a dsn",
		},
	}
	for _, c := range cases {
		got := redactDSN(c.in)
		if got != c.want {
			t.Errorf("redactDSN(%q) = %q, want %q", c.in, got, c.want)
		}
		if strings.Contains(got, "supersecret") {
			t.Errorf("redactDSN(%q) leaked the password: %q", c.in, got)
		}
	}
}

func TestFillDefaults_OverlaysOnlyMissing(t *testing.T) {
	t.Setenv("IMMICH_EMBEDDED_PG_DATA", "")
	t.Setenv("IMMICH_EMBEDDED_PG_BIN", "")
	got := fillDefaults(Config{Port: 6000, User: "alice"})
	if got.Port != 6000 {
		t.Errorf("Port was overridden: got %d", got.Port)
	}
	if got.User != "alice" {
		t.Errorf("User was overridden: got %q", got.User)
	}
	if got.DataPath == "" || got.BinariesPath == "" {
		t.Errorf("defaults not applied: data=%q bin=%q", got.DataPath, got.BinariesPath)
	}
	if got.Database != "immich" || got.Password != "immich" {
		t.Errorf("default user/db/password missing: %+v", got)
	}
	if got.StartTimeout <= 0 {
		t.Errorf("StartTimeout default missing: %v", got.StartTimeout)
	}
}
