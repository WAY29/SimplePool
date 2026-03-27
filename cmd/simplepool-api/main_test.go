package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseArgsAcceptsConfigPath(t *testing.T) {
	opts, err := parseArgs([]string{"--config", "runtime/simplepool.env"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}

	if got, want := opts.ConfigPath, "runtime/simplepool.env"; got != want {
		t.Fatalf("ConfigPath = %q, want %q", got, want)
	}

	if opts.Debug {
		t.Fatal("Debug = true, want false")
	}
}

func TestParseArgsAcceptsDebugFlag(t *testing.T) {
	opts, err := parseArgs([]string{"-debug"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}

	if !opts.Debug {
		t.Fatal("Debug = false, want true")
	}
}

func TestParseArgsHelpReturnsFlagErrHelp(t *testing.T) {
	_, err := parseArgs([]string{"-h"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("parseArgs() error = %v, want flag.ErrHelp", err)
	}
}

func TestUsageTextIncludesSupportedFlags(t *testing.T) {
	text := usageText()
	for _, want := range []string{"Usage of simplepool-api:", "-config", "-debug"} {
		if !strings.Contains(text, want) {
			t.Fatalf("usageText() missing %q in %q", want, text)
		}
	}
}

func TestSetGinModeDefaultsToRelease(t *testing.T) {
	previous := gin.Mode()
	t.Cleanup(func() {
		gin.SetMode(previous)
	})

	setGinMode(false)
	if got, want := gin.Mode(), gin.ReleaseMode; got != want {
		t.Fatalf("gin.Mode() = %q, want %q", got, want)
	}

	setGinMode(true)
	if got, want := gin.Mode(), gin.DebugMode; got != want {
		t.Fatalf("gin.Mode() = %q, want %q", got, want)
	}
}

func TestLoadConfigUsesConfigFile(t *testing.T) {
	rootDir := t.TempDir()
	configPath := filepath.Join(rootDir, "simplepool.env")
	masterKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32))

	content := []byte("" +
		"# SimplePool 示例配置\n" +
		"SIMPLEPOOL_DATA_DIR=" + filepath.Join(rootDir, "data") + "\n" +
		"SIMPLEPOOL_RUNTIME_DIR=" + filepath.Join(rootDir, "runtime") + "\n" +
		"SIMPLEPOOL_TEMP_DIR=" + filepath.Join(rootDir, "tmp") + "\n" +
		"SIMPLEPOOL_DB_PATH=" + filepath.Join(rootDir, "data", "simplepool.db") + "\n" +
		"SIMPLEPOOL_HTTP_ADDR=127.0.0.1:18080\n" +
		"SIMPLEPOOL_LOG_LEVEL=debug\n" +
		"SIMPLEPOOL_ADMIN_USERNAME=operator\n" +
		"SIMPLEPOOL_ADMIN_PASSWORD=super-secret\n" +
		"SIMPLEPOOL_MASTER_KEY=" + masterKey + "\n")
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := loadConfig(options{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if got, want := cfg.HTTPAddr, "127.0.0.1:18080"; got != want {
		t.Fatalf("HTTPAddr = %q, want %q", got, want)
	}

	if got, want := cfg.Paths.DataDir, filepath.Join(rootDir, "data"); got != want {
		t.Fatalf("DataDir = %q, want %q", got, want)
	}

	if got, want := cfg.LogLevel, "debug"; got != want {
		t.Fatalf("LogLevel = %q, want %q", got, want)
	}

	if got, want := cfg.Admin.Username, "operator"; got != want {
		t.Fatalf("Admin.Username = %q, want %q", got, want)
	}

	if got := len(cfg.Security.MasterKey); got != 32 {
		t.Fatalf("len(MasterKey) = %d, want 32", got)
	}
}

func TestLoadConfigEnablesDebugFromCLIFlag(t *testing.T) {
	rootDir := t.TempDir()
	configPath := filepath.Join(rootDir, "simplepool.env")
	masterKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))

	content := []byte("" +
		"SIMPLEPOOL_ADMIN_PASSWORD=super-secret\n" +
		"SIMPLEPOOL_MASTER_KEY=" + masterKey + "\n")
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := loadConfig(options{ConfigPath: configPath, Debug: true})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if !cfg.Debug {
		t.Fatal("Debug = false, want true")
	}
}
