package config_test

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WAY29/SimplePool/internal/config"
)

func TestLoadDefaultsAndDerivedPaths(t *testing.T) {
	t.Setenv("SIMPLEPOOL_MASTER_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	t.Setenv("SIMPLEPOOL_ADMIN_PASSWORD", "super-secret")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Admin.Username != "admin" {
		t.Fatalf("Admin.Username = %q, want admin", cfg.Admin.Username)
	}

	if cfg.HTTPAddr != "127.0.0.1:7891" {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.HTTPAddr, "127.0.0.1:7891")
	}

	rootDir, err := filepath.Abs(".simplepool")
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}

	if got, want := cfg.Paths.DataDir, filepath.Join(rootDir, "data"); got != want {
		t.Fatalf("DataDir = %q, want %q", got, want)
	}

	if got, want := cfg.Paths.RuntimeDir, filepath.Join(rootDir, "runtime"); got != want {
		t.Fatalf("RuntimeDir = %q, want %q", got, want)
	}

	if got, want := cfg.Paths.TempDir, filepath.Join(rootDir, "tmp"); got != want {
		t.Fatalf("TempDir = %q, want %q", got, want)
	}

	if got, want := cfg.Paths.DBPath, filepath.Join(cfg.Paths.DataDir, "simplepool.db"); got != want {
		t.Fatalf("DBPath = %q, want %q", got, want)
	}

	if got := len(cfg.Security.MasterKey); got != 32 {
		t.Fatalf("len(MasterKey) = %d, want 32", got)
	}
}

func TestLoadParsesDebugFlag(t *testing.T) {
	t.Setenv("SIMPLEPOOL_MASTER_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	t.Setenv("SIMPLEPOOL_ADMIN_PASSWORD", "super-secret")
	t.Setenv("SIMPLEPOOL_DEBUG", "true")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.Debug {
		t.Fatal("Debug = false, want true")
	}
}

func TestLoadParsesUpstreamHTTPProxyURL(t *testing.T) {
	t.Setenv("SIMPLEPOOL_MASTER_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	t.Setenv("SIMPLEPOOL_ADMIN_PASSWORD", "super-secret")
	t.Setenv("SIMPLEPOOL_UPSTREAM_HTTP_PROXY_URL", "http://user-1:pass-1@proxy.example.com:8080")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.UpstreamHTTPProxyURL != "http://user-1:pass-1@proxy.example.com:8080" {
		t.Fatalf("UpstreamHTTPProxyURL = %q, want configured proxy url", cfg.UpstreamHTTPProxyURL)
	}
}

func TestLoadRejectsInvalidUpstreamHTTPProxyURL(t *testing.T) {
	t.Setenv("SIMPLEPOOL_MASTER_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	t.Setenv("SIMPLEPOOL_ADMIN_PASSWORD", "super-secret")
	t.Setenv("SIMPLEPOOL_UPSTREAM_HTTP_PROXY_URL", "socks5://proxy.example.com:1080")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "upstream http proxy") {
		t.Fatalf("Load() error = %v, want upstream http proxy error", err)
	}
}

func TestLoadMasterKeyFromFile(t *testing.T) {
	root := t.TempDir()
	keyFile := filepath.Join(root, "master.key")
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32))
	if err := os.WriteFile(keyFile, []byte(key+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("SIMPLEPOOL_DATA_DIR", filepath.Join(root, "data"))
	t.Setenv("SIMPLEPOOL_RUNTIME_DIR", filepath.Join(root, "runtime"))
	t.Setenv("SIMPLEPOOL_TEMP_DIR", filepath.Join(root, "tmp"))
	t.Setenv("SIMPLEPOOL_DB_PATH", filepath.Join(root, "data", "simplepool.db"))
	t.Setenv("SIMPLEPOOL_MASTER_KEY_FILE", keyFile)
	t.Setenv("SIMPLEPOOL_ADMIN_PASSWORD", "super-secret")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !bytes.Equal(cfg.Security.MasterKey, bytes.Repeat([]byte{9}, 32)) {
		t.Fatal("MasterKey mismatch")
	}
}

func TestLoadRejectsMissingMasterKey(t *testing.T) {
	t.Setenv("SIMPLEPOOL_ADMIN_PASSWORD", "super-secret")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "master key") {
		t.Fatalf("Load() error = %v, want master key error", err)
	}
}

func TestLoadRejectsInvalidMasterKey(t *testing.T) {
	root := t.TempDir()
	keyFile := filepath.Join(root, "master.key")
	if err := os.WriteFile(keyFile, []byte("invalid-base64"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("SIMPLEPOOL_DATA_DIR", filepath.Join(root, "data"))
	t.Setenv("SIMPLEPOOL_RUNTIME_DIR", filepath.Join(root, "runtime"))
	t.Setenv("SIMPLEPOOL_TEMP_DIR", filepath.Join(root, "tmp"))
	t.Setenv("SIMPLEPOOL_DB_PATH", filepath.Join(root, "data", "simplepool.db"))
	t.Setenv("SIMPLEPOOL_MASTER_KEY_FILE", keyFile)
	t.Setenv("SIMPLEPOOL_ADMIN_PASSWORD", "super-secret")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "master key") {
		t.Fatalf("Load() error = %v, want master key error", err)
	}
}
