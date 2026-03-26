package app_test

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WAY29/SimplePool/internal/app"
	"github.com/WAY29/SimplePool/internal/config"
)

func TestAppBootstrapCreatesDirsAndServesHealth(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{
		HTTPAddr: "127.0.0.1:0",
		LogLevel: "debug",
		Paths: config.Paths{
			DataDir:    filepath.Join(root, "data"),
			RuntimeDir: filepath.Join(root, "runtime"),
			TempDir:    filepath.Join(root, "tmp"),
			DBPath:     filepath.Join(root, "data", "simplepool.db"),
		},
		Admin: config.Admin{
			Username: "admin",
			Password: "super-secret",
		},
		Security: config.Security{
			MasterKey: bytes.Repeat([]byte{1}, 32),
		},
	}

	ctx := context.Background()
	instance, err := app.New(ctx, cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := instance.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + instance.Address() + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz error = %v", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	for _, dir := range []string{cfg.Paths.DataDir, cfg.Paths.RuntimeDir, cfg.Paths.TempDir} {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("Stat(%q) error = %v", dir, err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := instance.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}
