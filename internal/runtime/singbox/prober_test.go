package singbox

import (
	"encoding/json"
	"testing"

	"github.com/WAY29/SimplePool/internal/node"
)

func TestProberBuildConfigUsesNormalizedLogLevel(t *testing.T) {
	prober := NewProber("https://cloudflare.com/cdn-cgi/trace", 0, "warning")

	configJSON, err := prober.buildConfig(node.ProbeTarget{
		ID:             "node-1",
		Name:           "HK-A",
		Protocol:       "trojan",
		Server:         "1.1.1.1",
		ServerPort:     443,
		Credential:     []byte(`{"password":"secret"}`),
		TransportJSON:  `{}`,
		TLSJSON:        `{"enabled":true,"server_name":"hk.example.com"}`,
		RawPayloadJSON: `{}`,
	}, 18080)
	if err != nil {
		t.Fatalf("buildConfig() error = %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(configJSON, &config); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	logConfig, ok := config["log"].(map[string]any)
	if !ok {
		t.Fatal("log config missing")
	}
	if logConfig["level"] != "warn" {
		t.Fatalf("log.level = %v, want warn", logConfig["level"])
	}
}

func TestProberBuildConfigDefaultsLogLevelToInfo(t *testing.T) {
	prober := NewProber("https://cloudflare.com/cdn-cgi/trace", 0, "")

	configJSON, err := prober.buildConfig(node.ProbeTarget{
		ID:             "node-1",
		Name:           "HK-A",
		Protocol:       "trojan",
		Server:         "1.1.1.1",
		ServerPort:     443,
		Credential:     []byte(`{"password":"secret"}`),
		TransportJSON:  `{}`,
		TLSJSON:        `{"enabled":true,"server_name":"hk.example.com"}`,
		RawPayloadJSON: `{}`,
	}, 18080)
	if err != nil {
		t.Fatalf("buildConfig() error = %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(configJSON, &config); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	logConfig := config["log"].(map[string]any)
	if logConfig["level"] != "info" {
		t.Fatalf("log.level = %v, want info", logConfig["level"])
	}
}
