package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/WAY29/SimplePool/internal/app"
	"github.com/WAY29/SimplePool/internal/config"
	"github.com/WAY29/SimplePool/internal/node"
	"github.com/WAY29/SimplePool/internal/runtime/singbox"
	"github.com/WAY29/SimplePool/internal/subscription"
)

func TestAppLoginToNodeManagementFlow(t *testing.T) {
	instance := newIntegrationApp(t)
	defer shutdownIntegrationApp(t, instance)

	token := loginIntegrationApp(t, instance)
	resp := integrationRequest(t, instance, http.MethodPost, "/api/nodes", token, map[string]any{
		"name":             "HK-A",
		"protocol":         "trojan",
		"server":           "1.1.1.1",
		"server_port":      443,
		"transport_json":   `{}`,
		"tls_json":         `{"enabled":true,"server_name":"hk.example.com"}`,
		"raw_payload_json": `{}`,
		"credential":       `{"password":"secret"}`,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/nodes status = %d, want 201", resp.StatusCode)
	}

	resp = integrationRequest(t, instance, http.MethodGet, "/api/nodes", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/nodes status = %d, want 200", resp.StatusCode)
	}
}

func TestAppSubscriptionRefreshToGroupMemberFlow(t *testing.T) {
	instance := newIntegrationApp(t)
	defer shutdownIntegrationApp(t, instance)

	token := loginIntegrationApp(t, instance)
	resp := integrationRequest(t, instance, http.MethodPost, "/api/subscriptions", token, map[string]any{
		"name": "sub-a",
		"url":  "https://example.com/sub.txt",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/subscriptions status = %d, want 201", resp.StatusCode)
	}
	var created map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode subscription response error = %v", err)
	}
	subID := created["id"].(string)

	resp = integrationRequest(t, instance, http.MethodPost, "/api/subscriptions/"+subID+"/refresh", token, map[string]any{
		"force": true,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/subscriptions/:id/refresh status = %d, want 200", resp.StatusCode)
	}

	resp = integrationRequest(t, instance, http.MethodPost, "/api/groups", token, map[string]any{
		"name":         "trojan",
		"filter_regex": "^TR-",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/groups status = %d, want 201", resp.StatusCode)
	}
	var group map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&group); err != nil {
		t.Fatalf("decode group response error = %v", err)
	}

	resp = integrationRequest(t, instance, http.MethodGet, "/api/groups/"+group["id"].(string)+"/members", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/groups/:id/members status = %d, want 200", resp.StatusCode)
	}
}

func TestAppGroupToTunnelCreationFlow(t *testing.T) {
	instance := newIntegrationApp(t)
	defer shutdownIntegrationApp(t, instance)

	token := loginIntegrationApp(t, instance)
	groupID := createGroupWithNodesForTunnel(t, instance, token)

	resp := integrationRequest(t, instance, http.MethodPost, "/api/tunnels", token, map[string]any{
		"name":     "proxy-a",
		"group_id": groupID,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/tunnels status = %d, want 201", resp.StatusCode)
	}
}

func TestAppTunnelRefreshFlow(t *testing.T) {
	instance := newIntegrationApp(t)
	defer shutdownIntegrationApp(t, instance)

	token := loginIntegrationApp(t, instance)
	groupID := createGroupWithNodesForTunnel(t, instance, token)

	resp := integrationRequest(t, instance, http.MethodPost, "/api/tunnels", token, map[string]any{
		"name":     "proxy-a",
		"group_id": groupID,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/tunnels status = %d, want 201", resp.StatusCode)
	}

	var created map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode tunnel response error = %v", err)
	}
	tunnelID := created["id"].(string)

	resp = integrationRequest(t, instance, http.MethodPost, "/api/tunnels/"+tunnelID+"/refresh", token, map[string]any{})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/tunnels/:id/refresh status = %d, want 200", resp.StatusCode)
	}
}

func TestAppTunnelUsesConfiguredLogLevelForSingBox(t *testing.T) {
	runtime := newIntegrationRuntime()
	instance := newIntegrationAppWithRuntime(t, runtime, "warning")
	defer shutdownIntegrationApp(t, instance)

	token := loginIntegrationApp(t, instance)
	groupID := createGroupWithNodesForTunnel(t, instance, token)

	resp := integrationRequest(t, instance, http.MethodPost, "/api/tunnels", token, map[string]any{
		"name":     "proxy-a",
		"group_id": groupID,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/tunnels status = %d, want 201", resp.StatusCode)
	}

	var created map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode tunnel response error = %v", err)
	}
	tunnelID := created["id"].(string)

	state := runtime.states[tunnelID]
	if state == nil {
		t.Fatal("runtime state missing")
	}
	if state.logLevel != "warn" {
		t.Fatalf("runtime logLevel = %q, want warn", state.logLevel)
	}
}

func newIntegrationApp(t *testing.T) *app.App {
	t.Helper()

	return newIntegrationAppWithRuntime(t, newIntegrationRuntime(), "debug")
}

func newIntegrationAppWithRuntime(t *testing.T, runtime *integrationRuntime, logLevel string) *app.App {
	t.Helper()

	root := t.TempDir()
	cfg := config.Config{
		HTTPAddr: "127.0.0.1:0",
		LogLevel: logLevel,
		Paths: config.Paths{
			DataDir:    root + "/data",
			RuntimeDir: root + "/runtime",
			TempDir:    root + "/tmp",
			DBPath:     root + "/data/simplepool.db",
		},
		Admin: config.Admin{
			Username: "admin",
			Password: "super-secret",
		},
		Security: config.Security{
			MasterKey: bytes.Repeat([]byte{1}, 32),
		},
	}

	instance, err := app.NewWithDependencies(context.Background(), cfg, app.Dependencies{
		NodeProber:          &integrationProber{},
		SubscriptionFetcher: &integrationFetcher{payload: "trojan://pass@example.com:443?security=tls#TR-1"},
		TunnelRuntime:       runtime,
		Now: func() time.Time {
			return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("app.NewWithDependencies() error = %v", err)
	}
	if err := instance.Start(); err != nil {
		t.Fatalf("app.Start() error = %v", err)
	}
	return instance
}

func shutdownIntegrationApp(t *testing.T, instance *app.App) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := instance.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func loginIntegrationApp(t *testing.T, instance *app.App) string {
	t.Helper()
	resp := integrationRequest(t, instance, http.MethodPost, "/api/auth/login", "", map[string]any{
		"username": "admin",
		"password": "super-secret",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/auth/login status = %d, want 200", resp.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode login response error = %v", err)
	}
	return payload["token"].(string)
}

func createGroupWithNodesForTunnel(t *testing.T, instance *app.App, token string) string {
	t.Helper()

	for _, item := range []map[string]any{
		{
			"name":             "HK-fast",
			"protocol":         "trojan",
			"server":           "1.1.1.1",
			"server_port":      443,
			"transport_json":   `{}`,
			"tls_json":         `{"enabled":true,"server_name":"hk.example.com"}`,
			"raw_payload_json": `{}`,
			"credential":       `{"password":"secret-1"}`,
		},
		{
			"name":             "JP-slow",
			"protocol":         "trojan",
			"server":           "2.2.2.2",
			"server_port":      443,
			"transport_json":   `{}`,
			"tls_json":         `{"enabled":true,"server_name":"jp.example.com"}`,
			"raw_payload_json": `{}`,
			"credential":       `{"password":"secret-2"}`,
		},
	} {
		resp := integrationRequest(t, instance, http.MethodPost, "/api/nodes", token, item)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("POST /api/nodes status = %d, want 201", resp.StatusCode)
		}
	}

	resp := integrationRequest(t, instance, http.MethodPost, "/api/groups", token, map[string]any{
		"name":         "asia",
		"filter_regex": "^(HK|JP)-",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/groups status = %d, want 201", resp.StatusCode)
	}
	var group map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&group); err != nil {
		t.Fatalf("decode group response error = %v", err)
	}
	return group["id"].(string)
}

func integrationRequest(t *testing.T, instance *app.App, method, path, token string, body any) *http.Response {
	t.Helper()

	var reqBody *bytes.Reader
	if body == nil {
		reqBody = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, "http://"+instance.Address()+path, reqBody)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	return resp
}

type integrationFetcher struct {
	payload string
}

func (f *integrationFetcher) Fetch(ctx context.Context, request subscription.FetchRequest) ([]byte, error) {
	return []byte(f.payload), nil
}

type integrationProber struct{}

func (p *integrationProber) Probe(ctx context.Context, target node.ProbeTarget) (node.ProbeResult, error) {
	latency := int64(20)
	switch target.Name {
	case "HK-fast":
		latency = 10
	case "JP-slow":
		latency = 30
	case "TR-1":
		latency = 15
	}
	return node.ProbeResult{
		Success:   true,
		LatencyMS: latency,
		TestURL:   "https://cloudflare.com/cdn-cgi/trace",
	}, nil
}

type integrationRuntime struct {
	states map[string]*integrationRuntimeState
}

type integrationRuntimeState struct {
	all      []string
	now      string
	logLevel string
}

func newIntegrationRuntime() *integrationRuntime {
	return &integrationRuntime{states: make(map[string]*integrationRuntimeState)}
}

func (r *integrationRuntime) Start(ctx context.Context, tunnelID string, layout singbox.RuntimeLayout, config []byte) error {
	if err := os.MkdirAll(layout.RootDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(layout.ConfigPath, config, 0o644); err != nil {
		return err
	}
	var payload struct {
		Outbounds []map[string]any `json:"outbounds"`
		Log       map[string]any   `json:"log"`
	}
	if err := json.Unmarshal(config, &payload); err != nil {
		return err
	}
	state := &integrationRuntimeState{}
	state.logLevel, _ = payload.Log["level"].(string)
	for _, outbound := range payload.Outbounds {
		tag, _ := outbound["tag"].(string)
		if tag != "tunnel-selector" {
			continue
		}
		if raw, ok := outbound["outbounds"].([]any); ok {
			for _, item := range raw {
				if value, ok := item.(string); ok {
					state.all = append(state.all, value)
				}
			}
		}
		state.now, _ = outbound["default"].(string)
	}
	r.states[tunnelID] = state
	return nil
}

func (r *integrationRuntime) Stop(ctx context.Context, tunnelID string) error {
	return nil
}

func (r *integrationRuntime) Delete(ctx context.Context, tunnelID string) error {
	delete(r.states, tunnelID)
	return nil
}

func (r *integrationRuntime) GetSelector(ctx context.Context, tunnelID string, controllerPort int, secret string) (*singbox.ProxyInfo, error) {
	state := r.states[tunnelID]
	if state == nil {
		return nil, errors.New("runtime missing")
	}
	return &singbox.ProxyInfo{
		Type: "Selector",
		Name: "tunnel-selector",
		Now:  state.now,
		All:  append([]string(nil), state.all...),
	}, nil
}

func (r *integrationRuntime) SwitchSelector(ctx context.Context, tunnelID string, controllerPort int, secret, outbound string) error {
	state := r.states[tunnelID]
	if state == nil {
		return errors.New("runtime missing")
	}
	state.now = outbound
	return nil
}

func (r *integrationRuntime) Close() error {
	return nil
}
