package config

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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
	Debug                bool
	HTTPAddr             string
	LogLevel             string
	UpstreamHTTPProxyURL string
	Paths                Paths
	Admin                Admin
	Security             Security
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

	upstreamHTTPProxyURL := strings.TrimSpace(os.Getenv("SIMPLEPOOL_UPSTREAM_HTTP_PROXY_URL"))
	if err := validateUpstreamHTTPProxyURL(upstreamHTTPProxyURL); err != nil {
		return Config{}, apperr.Wrap(apperr.CodeConfig, op, err)
	}

	return Config{
		Debug:                parseBoolEnv("SIMPLEPOOL_DEBUG"),
		HTTPAddr:             envOrDefault("SIMPLEPOOL_HTTP_ADDR", defaultHTTPAddr),
		LogLevel:             envOrDefault("SIMPLEPOOL_LOG_LEVEL", "info"),
		UpstreamHTTPProxyURL: upstreamHTTPProxyURL,
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

func parseBoolEnv(key string) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func resolvePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}

	return filepath.Abs(path)
}

func validateUpstreamHTTPProxyURL(raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("upstream http proxy url is invalid: %w", err)
	}
	if parsed.Scheme != "http" {
		return fmt.Errorf("upstream http proxy url must use http scheme")
	}
	if parsed.Hostname() == "" {
		return fmt.Errorf("upstream http proxy url host is required")
	}
	if parsed.Port() != "" {
		port, err := strconv.Atoi(parsed.Port())
		if err != nil || port <= 0 || port > 65535 {
			return fmt.Errorf("upstream http proxy url port is invalid")
		}
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return fmt.Errorf("upstream http proxy url path is not supported")
	}

	return nil
}
