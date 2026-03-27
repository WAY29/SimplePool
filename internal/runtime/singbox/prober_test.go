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

func TestProberBuildConfigUsesLocalResolverForDomainOutbounds(t *testing.T) {
	prober := NewProber("https://cloudflare.com/cdn-cgi/trace", 0, "info")

	configJSON, err := prober.buildConfig(node.ProbeTarget{
		ID:             "node-1",
		Name:           "HK-A",
		Protocol:       "vless",
		Server:         "downloadcfpro.example.com",
		ServerPort:     443,
		Credential:     []byte(`{"uuid":"u-1"}`),
		TransportJSON:  `{"type":"ws","host":"hk.example.com","path":"/ws"}`,
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

	dnsConfig := config["dns"].(map[string]any)
	servers := dnsConfig["servers"].([]any)
	server := servers[0].(map[string]any)
	if _, exists := server["detour"]; exists {
		t.Fatalf("dns server detour should be omitted, got %+v", server)
	}

	outbounds := config["outbounds"].([]any)
	firstOutbound := outbounds[0].(map[string]any)
	if firstOutbound["domain_resolver"] != "local" {
		t.Fatalf("outbound.domain_resolver = %v, want local", firstOutbound["domain_resolver"])
	}

	routeConfig := config["route"].(map[string]any)
	if routeConfig["default_domain_resolver"] != "local" {
		t.Fatalf("route.default_domain_resolver = %v, want local", routeConfig["default_domain_resolver"])
	}
}
