package settings_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/WAY29/SimplePool/internal/settings"
	"github.com/WAY29/SimplePool/internal/store/sqlite"
)

func TestServiceGetProbeConfigReturnsDefaultWhenUnset(t *testing.T) {
	service := newSettingsService(t)

	view, err := service.GetProbeConfig(context.Background())
	if err != nil {
		t.Fatalf("GetProbeConfig() error = %v", err)
	}
	if view.TestURL != settings.DefaultProbeTestURL {
		t.Fatalf("TestURL = %q, want %q", view.TestURL, settings.DefaultProbeTestURL)
	}
	if len(view.PresetURLs) != 2 {
		t.Fatalf("len(PresetURLs) = %d, want 2", len(view.PresetURLs))
	}
	if view.UpdatedAt != nil {
		t.Fatalf("UpdatedAt = %v, want nil", view.UpdatedAt)
	}
}

func TestServiceSetProbeTestURLPersistsCustomURL(t *testing.T) {
	service := newSettingsService(t)
	ctx := context.Background()

	saved, err := service.SetProbeTestURL(ctx, " https://example.com/generate_204 ")
	if err != nil {
		t.Fatalf("SetProbeTestURL() error = %v", err)
	}
	if saved.TestURL != "https://example.com/generate_204" {
		t.Fatalf("saved TestURL = %q, want trimmed custom URL", saved.TestURL)
	}
	if saved.UpdatedAt == nil {
		t.Fatal("saved UpdatedAt = nil, want value")
	}

	loaded, err := service.GetProbeConfig(ctx)
	if err != nil {
		t.Fatalf("GetProbeConfig() error = %v", err)
	}
	if loaded.TestURL != "https://example.com/generate_204" {
		t.Fatalf("loaded TestURL = %q, want saved custom URL", loaded.TestURL)
	}
	if loaded.UpdatedAt == nil || !loaded.UpdatedAt.Equal(*saved.UpdatedAt) {
		t.Fatalf("loaded UpdatedAt = %v, want %v", loaded.UpdatedAt, saved.UpdatedAt)
	}
}

func TestServiceSetProbeTestURLRejectsInvalidValue(t *testing.T) {
	service := newSettingsService(t)

	for _, raw := range []string{"", "ftp://example.com", "not-a-url"} {
		if _, err := service.SetProbeTestURL(context.Background(), raw); err == nil {
			t.Fatalf("SetProbeTestURL(%q) error = nil, want invalid url", raw)
		}
	}
}

func newSettingsService(t *testing.T) *settings.Service {
	t.Helper()

	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := sqlite.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	repos := sqlite.NewRepositories(db)
	return settings.NewService(settings.Options{
		AppSettings: repos.AppSettings,
		Now: func() time.Time {
			return time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC)
		},
	})
}
