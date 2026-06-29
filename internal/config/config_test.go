package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadSecretPrefersFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pw")
	if err := os.WriteFile(path, []byte("  filesecret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DB_PASSWORD", "envsecret")
	t.Setenv("DB_PASSWORD_FILE", path)

	got, err := readSecret("DB_PASSWORD")
	if err != nil {
		t.Fatalf("readSecret: %v", err)
	}
	if got != "filesecret" {
		t.Fatalf("want trimmed file contents %q, got %q", "filesecret", got)
	}
}

func TestReadSecretFallsBackToEnv(t *testing.T) {
	t.Setenv("REDIS_PASSWORD", "token")
	os.Unsetenv("REDIS_PASSWORD_FILE")

	got, err := readSecret("REDIS_PASSWORD")
	if err != nil {
		t.Fatalf("readSecret: %v", err)
	}
	if got != "token" {
		t.Fatalf("want %q, got %q", "token", got)
	}
}

func TestLoadRejectsBadMode(t *testing.T) {
	t.Setenv("APP_MODE", "nonsense")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid APP_MODE, got nil")
	}
}

func TestLoadDefaults(t *testing.T) {
	for _, k := range []string{"APP_MODE", "PORT", "DB_PORT", "DB_NAME", "REDIS_QUEUE_KEY"} {
		os.Unsetenv(k)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Mode != ModeServer {
		t.Errorf("default mode: want %q, got %q", ModeServer, cfg.Mode)
	}
	if cfg.Port != "8080" {
		t.Errorf("default port: want 8080, got %q", cfg.Port)
	}
	if cfg.Redis.QueueKey != "demo:jobs" {
		t.Errorf("default queue key: want demo:jobs, got %q", cfg.Redis.QueueKey)
	}
}
