package singbox

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/WAY29/SimplePool/internal/node"
	"github.com/WAY29/SimplePool/internal/settings"
)

func TestProberUsesDefaultTestURLWhenResolverReturnsEmpty(t *testing.T) {
	prober := NewDynamicProber(func(context.Context) string {
		return ""
	}, 0, "info")

	if got := prober.testURL(context.Background()); got != settings.DefaultProbeTestURL {
		t.Fatalf("testURL() = %q, want %q", got, settings.DefaultProbeTestURL)
	}
}

func TestProberUsesDynamicResolvedTestURL(t *testing.T) {
	prober := NewDynamicProber(func(context.Context) string {
		return "https://example.com/generate_204"
	}, 0, "info")

	if got := prober.testURL(context.Background()); got != "https://example.com/generate_204" {
		t.Fatalf("testURL() = %q, want dynamic URL", got)
	}
}

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

func TestProberBuildConfigRoutesOutboundThroughUpstreamHTTPProxy(t *testing.T) {
	prober := NewProber("https://cloudflare.com/cdn-cgi/trace", 0, "info", "http://user-1:pass-1@proxy.example.com:8080")

	configJSON, err := prober.buildConfig(node.ProbeTarget{
		ID:             "node-1",
		Name:           "HK-A",
		Protocol:       "trojan",
		Server:         "downloadcfpro.example.com",
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

	outbounds := config["outbounds"].([]any)
	if len(outbounds) != 2 {
		t.Fatalf("len(outbounds) = %d, want 2", len(outbounds))
	}

	probeOutbound := outbounds[0].(map[string]any)
	if probeOutbound["tag"] != "probe-out" {
		t.Fatalf("probe outbound tag = %v, want probe-out", probeOutbound["tag"])
	}
	if probeOutbound["detour"] != "upstream-http-proxy" {
		t.Fatalf("probe outbound detour = %v, want upstream-http-proxy", probeOutbound["detour"])
	}

	upstream := outbounds[1].(map[string]any)
	if upstream["tag"] != "upstream-http-proxy" {
		t.Fatalf("upstream tag = %v, want upstream-http-proxy", upstream["tag"])
	}
	if upstream["type"] != "http" {
		t.Fatalf("upstream type = %v, want http", upstream["type"])
	}
	if upstream["server"] != "proxy.example.com" {
		t.Fatalf("upstream server = %v, want proxy.example.com", upstream["server"])
	}
	if int(upstream["server_port"].(float64)) != 8080 {
		t.Fatalf("upstream server_port = %v, want 8080", upstream["server_port"])
	}
	if upstream["username"] != "user-1" || upstream["password"] != "pass-1" {
		t.Fatalf("upstream auth = %+v, want configured username/password", upstream)
	}
}
