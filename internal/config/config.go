package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/WAY29/SimplePool/internal/apperr"
)

const (
	defaultDatabaseName = "simplepool.db"
	defaultHTTPAddr     = "127.0.0.1:7891"
	defaultDataDir      = ".simplepool/data"
	defaultRuntimeDir   = ".simplepool/runtime"
	defaultTempDir      = ".simplepool/tmp"
)

type Config struct {
	HTTPAddr string
	LogLevel string
	Paths    Paths
	Admin    Admin
	Security Security
}

type Paths struct {
	DataDir    string
	RuntimeDir string
	TempDir    string
	DBPath     string
}

type Admin struct {
	Username string
	Password string
}

type Security struct {
	MasterKey []byte
}

func Load() (Config, error) {
	const op = "config.Load"

	dataDir, err := resolvePath(envOrDefault("SIMPLEPOOL_DATA_DIR", defaultDataDir))
	if err != nil {
		return Config{}, apperr.Wrap(apperr.CodeConfig, op, fmt.Errorf("resolve data dir: %w", err))
	}

	runtimeDir, err := resolvePath(envOrDefault("SIMPLEPOOL_RUNTIME_DIR", defaultRuntimeDir))
	if err != nil {
		return Config{}, apperr.Wrap(apperr.CodeConfig, op, fmt.Errorf("resolve runtime dir: %w", err))
	}

	tempDir, err := resolvePath(envOrDefault("SIMPLEPOOL_TEMP_DIR", defaultTempDir))
	if err != nil {
		return Config{}, apperr.Wrap(apperr.CodeConfig, op, fmt.Errorf("resolve temp dir: %w", err))
	}

	dbPath, err := resolvePath(envOrDefault("SIMPLEPOOL_DB_PATH", filepath.Join(dataDir, defaultDatabaseName)))
	if err != nil {
		return Config{}, apperr.Wrap(apperr.CodeConfig, op, fmt.Errorf("resolve db path: %w", err))
	}

	adminUsername := envOrDefault("SIMPLEPOOL_ADMIN_USERNAME", "admin")
	adminPassword := strings.TrimSpace(os.Getenv("SIMPLEPOOL_ADMIN_PASSWORD"))
	if adminPassword == "" {
		return Config{}, apperr.New(apperr.CodeConfig, op, "admin password is required")
	}

	masterKey, err := loadMasterKey()
	if err != nil {
		return Config{}, apperr.Wrap(apperr.CodeConfig, op, err)
	}

	return Config{
		HTTPAddr: envOrDefault("SIMPLEPOOL_HTTP_ADDR", defaultHTTPAddr),
		LogLevel: envOrDefault("SIMPLEPOOL_LOG_LEVEL", "info"),
		Paths: Paths{
			DataDir:    dataDir,
			RuntimeDir: runtimeDir,
			TempDir:    tempDir,
			DBPath:     dbPath,
		},
		Admin: Admin{
			Username: adminUsername,
			Password: adminPassword,
		},
		Security: Security{
			MasterKey: masterKey,
		},
	}, nil
}

func loadMasterKey() ([]byte, error) {
	const op = "config.loadMasterKey"

	inlineKey := strings.TrimSpace(os.Getenv("SIMPLEPOOL_MASTER_KEY"))
	filePath := strings.TrimSpace(os.Getenv("SIMPLEPOOL_MASTER_KEY_FILE"))

	if inlineKey != "" && filePath != "" {
		return nil, apperr.New(apperr.CodeConfig, op, "master key and master key file cannot both be set")
	}

	switch {
	case inlineKey != "":
		return decodeMasterKey(inlineKey)
	case filePath != "":
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read master key file: %w", err)
		}

		return decodeMasterKey(strings.TrimSpace(string(content)))
	default:
		return nil, apperr.New(apperr.CodeConfig, op, "master key is required")
	}
}

func decodeMasterKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode master key: %w", err)
	}

	if len(key) != 32 {
		return nil, fmt.Errorf("master key must decode to 32 bytes, got %d", len(key))
	}

	return key, nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func resolvePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}

	return filepath.Abs(path)
}
