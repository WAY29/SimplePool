package node

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestParseImportPayloadTrojanPreservesTLSAndTransportOptions(t *testing.T) {
	items, err := ParseImportPayload("trojan://pass@example.com:443?allowInsecure=1&sni=hk.example.com&type=ws&host=cdn.example.com&path=%2Fws#TR-WS")
	if err != nil {
		t.Fatalf("ParseImportPayload() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}

	var transport map[string]any
	if err := json.Unmarshal([]byte(items[0].TransportJSON), &transport); err != nil {
		t.Fatalf("json.Unmarshal(transport) error = %v", err)
	}
	if transport["type"] != "ws" {
		t.Fatalf("transport.type = %v, want ws", transport["type"])
	}
	if transport["host"] != "cdn.example.com" {
		t.Fatalf("transport.host = %v, want cdn.example.com", transport["host"])
	}
	if transport["path"] != "/ws" {
		t.Fatalf("transport.path = %v, want /ws", transport["path"])
	}

	var tlsOptions map[string]any
	if err := json.Unmarshal([]byte(items[0].TLSJSON), &tlsOptions); err != nil {
		t.Fatalf("json.Unmarshal(tls) error = %v", err)
	}
	if enabled, _ := tlsOptions["enabled"].(bool); !enabled {
		t.Fatalf("tls.enabled = %v, want true", tlsOptions["enabled"])
	}
	if tlsOptions["server_name"] != "hk.example.com" {
		t.Fatalf("tls.server_name = %v, want hk.example.com", tlsOptions["server_name"])
	}
	if insecure, _ := tlsOptions["insecure"].(bool); !insecure {
		t.Fatalf("tls.insecure = %v, want true", tlsOptions["insecure"])
	}
}

func TestParseImportPayloadVMessPreservesAllowInsecure(t *testing.T) {
	raw := `{"v":"2","ps":"VM-1","add":"vm.example.com","port":"443","id":"uuid-1","aid":"0","scy":"auto","net":"ws","host":"cdn.example.com","path":"/ws","tls":"tls","sni":"vm.example.com","allowInsecure":"1"}`
	items, err := ParseImportPayload("vmess://" + base64.StdEncoding.EncodeToString([]byte(raw)))
	if err != nil {
		t.Fatalf("ParseImportPayload() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}

	var tlsOptions map[string]any
	if err := json.Unmarshal([]byte(items[0].TLSJSON), &tlsOptions); err != nil {
		t.Fatalf("json.Unmarshal(tls) error = %v", err)
	}
	if insecure, _ := tlsOptions["insecure"].(bool); !insecure {
		t.Fatalf("tls.insecure = %v, want true", tlsOptions["insecure"])
	}
}
