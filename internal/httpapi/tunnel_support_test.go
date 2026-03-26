package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/WAY29/SimplePool/internal/auth"
	appcrypto "github.com/WAY29/SimplePool/internal/crypto"
	"github.com/WAY29/SimplePool/internal/domain"
	"github.com/WAY29/SimplePool/internal/group"
	"github.com/WAY29/SimplePool/internal/node"
	"github.com/WAY29/SimplePool/internal/runtime/singbox"
	"github.com/WAY29/SimplePool/internal/store/sqlite"
	"github.com/WAY29/SimplePool/internal/tunnel"
)

type httpTunnelRuntime struct {
	states map[string]*httpTunnelState
}

type httpTunnelState struct {
	running bool
	all     []string
	now     string
}

func newHTTPTunnelRuntime() *httpTunnelRuntime {
	return &httpTunnelRuntime{states: make(map[string]*httpTunnelState)}
}

func (r *httpTunnelRuntime) Start(ctx context.Context, tunnelID string, layout singbox.RuntimeLayout, config []byte) error {
	if err := os.MkdirAll(layout.RootDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(layout.ConfigPath, config, 0o644); err != nil {
		return err
	}
	var payload struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal(config, &payload); err != nil {
		return err
	}
	state := &httpTunnelState{running: true}
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

func (r *httpTunnelRuntime) Stop(ctx context.Context, tunnelID string) error {
	if state := r.states[tunnelID]; state != nil {
		state.running = false
	}
	return nil
}

func (r *httpTunnelRuntime) Delete(ctx context.Context, tunnelID string) error {
	delete(r.states, tunnelID)
	return nil
}

func (r *httpTunnelRuntime) GetSelector(ctx context.Context, tunnelID string, controllerPort int, secret string) (*singbox.ProxyInfo, error) {
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

func (r *httpTunnelRuntime) SwitchSelector(ctx context.Context, tunnelID string, controllerPort int, secret, outbound string) error {
	state := r.states[tunnelID]
	if state == nil {
		return errors.New("runtime missing")
	}
	state.now = outbound
	return nil
}

func (r *httpTunnelRuntime) Close() error {
	return nil
}

type httpTunnelProber struct{}

func (p *httpTunnelProber) Probe(ctx context.Context, target node.ProbeTarget) (node.ProbeResult, error) {
	latency := int64(20)
	switch target.Name {
	case "HK-fast":
		latency = 10
	case "JP-slow":
		latency = 30
	case "US-mid":
		latency = 15
	}
	return node.ProbeResult{
		Success:   true,
		LatencyMS: latency,
		TestURL:   "https://cloudflare.com/cdn-cgi/trace",
	}, nil
}

func seedTunnelFixturesForHTTPTests(repos *sqlite.Repositories, cipher *appcrypto.AESGCM) error {
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	for _, groupItem := range []*domain.Group{
		{ID: "group-asia", Name: "asia", FilterRegex: "^(HK|JP)-", CreatedAt: now, UpdatedAt: now},
		{ID: "group-us", Name: "us", FilterRegex: "^US-", CreatedAt: now.Add(time.Minute), UpdatedAt: now.Add(time.Minute)},
	} {
		if err := repos.Groups.Create(context.Background(), groupItem); err != nil {
			return err
		}
	}

	for index, item := range []struct {
		ID   string
		Name string
	}{
		{ID: "node-hk-fast", Name: "HK-fast"},
		{ID: "node-jp-slow", Name: "JP-slow"},
		{ID: "node-us-mid", Name: "US-mid"},
	} {
		credential, nonce, err := cipher.Encrypt([]byte(`{"password":"secret"}`), []byte("node:credential:"+item.ID))
		if err != nil {
			return err
		}
		if err := repos.Nodes.Create(context.Background(), &domain.Node{
			ID:                   item.ID,
			Name:                 item.Name,
			DedupeFingerprint:    item.ID,
			SourceKind:           domain.NodeSourceManual,
			Protocol:             "trojan",
			Server:               "1.1.1.1",
			ServerPort:           443,
			CredentialCiphertext: credential,
			CredentialNonce:      nonce,
			TransportJSON:        "{}",
			TLSJSON:              `{"enabled":true,"server_name":"example.com"}`,
			RawPayloadJSON:       "{}",
			Enabled:              true,
			LastStatus:           domain.NodeStatusUnknown,
			CreatedAt:            now.Add(time.Duration(index+2) * time.Minute),
			UpdatedAt:            now.Add(time.Duration(index+2) * time.Minute),
		}); err != nil {
			return err
		}
	}

	return nil
}

func buildTunnelServiceForHTTPTests(repos *sqlite.Repositories, cipher *appcrypto.AESGCM, now func() time.Time, runtimeRoot string) *tunnel.Service {
	groupService := group.NewService(group.Options{
		Groups: repos.Groups,
		Nodes:  repos.Nodes,
		Now:    now,
	})
	return tunnel.NewService(tunnel.Options{
		Tunnels:        repos.Tunnels,
		TunnelEvents:   repos.TunnelEvents,
		LatencySamples: repos.LatencySamples,
		Groups:         groupService,
		Nodes:          repos.Nodes,
		Cipher:         cipher,
		Prober:         &httpTunnelProber{},
		Runtime:        newHTTPTunnelRuntime(),
		Renderer:       singbox.NewConfigRenderer(),
		PortAllocator:  singbox.NewPortAllocator(),
		RuntimeRoot:    filepath.Join(runtimeRoot, "runtime"),
		Now:            now,
	})
}

func buildAuthServiceForHTTPTests(repos *sqlite.Repositories, now func() time.Time) (*auth.Service, string, error) {
	authService := auth.NewService(auth.Options{
		AdminUsers: repos.AdminUsers,
		Sessions:   repos.Sessions,
		Now:        now,
		SessionTTL: 7 * 24 * time.Hour,
	})
	if err := authService.EnsureAdmin(context.Background(), "admin", "secret-1"); err != nil {
		return nil, "", err
	}
	login, err := authService.Login(context.Background(), auth.LoginInput{
		Username: "admin",
		Password: "secret-1",
	})
	if err != nil {
		return nil, "", err
	}
	return authService, login.Token, nil
}
