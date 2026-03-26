package tunnel_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	appcrypto "github.com/WAY29/SimplePool/internal/crypto"
	"github.com/WAY29/SimplePool/internal/domain"
	"github.com/WAY29/SimplePool/internal/group"
	"github.com/WAY29/SimplePool/internal/node"
	"github.com/WAY29/SimplePool/internal/runtime/singbox"
	"github.com/WAY29/SimplePool/internal/store/sqlite"
	"github.com/WAY29/SimplePool/internal/tunnel"
)

func TestTunnelServiceCreateStopStartAndDelete(t *testing.T) {
	ctx := context.Background()
	service, deps := newTunnelService(t)
	seedTunnelNodes(t, ctx, deps.repos, deps.now, deps.cipher)
	groupID := seedTunnelGroup(t, ctx, deps.groupService, "亚洲", "^(HK|JP)-")

	created, err := service.Create(ctx, tunnel.CreateInput{
		Name:     "proxy-a",
		GroupID:  groupID,
		Username: "user-a",
		Password: "pass-a",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Status != domain.TunnelStatusRunning {
		t.Fatalf("Status = %q, want running", created.Status)
	}
	if created.CurrentNodeID == nil || *created.CurrentNodeID != "node-hk-fast" {
		t.Fatalf("CurrentNodeID = %v, want node-hk-fast", created.CurrentNodeID)
	}
	if !created.HasAuth {
		t.Fatal("HasAuth = false, want true")
	}

	runtimeState := deps.runtime.state(created.ID)
	if runtimeState == nil || !runtimeState.running {
		t.Fatal("runtime not running after create")
	}
	if runtimeState.now != "node-node-hk-fast" {
		t.Fatalf("runtime selector now = %q, want node-node-hk-fast", runtimeState.now)
	}
	if !slices.Equal(runtimeState.all, []string{"node-node-hk-fast", "node-node-jp-slow"}) {
		t.Fatalf("runtime selector all = %v, want enabled group snapshot", runtimeState.all)
	}

	samples, err := deps.repos.LatencySamples.ListByTunnelID(ctx, created.ID, 10)
	if err != nil {
		t.Fatalf("LatencySamples.ListByTunnelID() error = %v", err)
	}
	if len(samples) != 2 {
		t.Fatalf("len(samples) = %d, want 2", len(samples))
	}

	stopped, err := service.Stop(ctx, created.ID)
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if stopped.Status != domain.TunnelStatusStopped {
		t.Fatalf("Stop() status = %q, want stopped", stopped.Status)
	}
	if deps.runtime.state(created.ID).running {
		t.Fatal("runtime still running after stop")
	}

	started, err := service.Start(ctx, created.ID)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if started.Status != domain.TunnelStatusRunning {
		t.Fatalf("Start() status = %q, want running", started.Status)
	}
	if deps.runtime.startCalls < 2 {
		t.Fatalf("startCalls = %d, want restart on Start()", deps.runtime.startCalls)
	}

	runtimeDir := created.RuntimeDir
	if _, err := os.Stat(runtimeDir); err != nil {
		t.Fatalf("Stat(runtimeDir) error = %v", err)
	}

	if err := service.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := service.Get(ctx, created.ID); err == nil {
		t.Fatal("Get() after delete error = nil, want not found")
	}
	if _, err := os.Stat(runtimeDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("runtimeDir still exists, err = %v", err)
	}

	events, err := deps.repos.TunnelEvents.ListByTunnelID(ctx, created.ID, 20)
	if err != nil {
		t.Fatalf("TunnelEvents.ListByTunnelID() error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("len(events) = %d, want 0 after hard delete cascade", len(events))
	}
}

func TestTunnelServiceRefreshFailureKeepsOldRuntimeAndMarksDegraded(t *testing.T) {
	ctx := context.Background()
	service, deps := newTunnelService(t)
	seedTunnelNodes(t, ctx, deps.repos, deps.now, deps.cipher)
	groupID := seedTunnelGroup(t, ctx, deps.groupService, "亚洲", "^(HK|JP)-")

	created, err := service.Create(ctx, tunnel.CreateInput{
		Name:    "proxy-a",
		GroupID: groupID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	deps.prober.results["node-hk-fast"] = node.ProbeResult{
		Success:      false,
		TestURL:      "https://cloudflare.com/cdn-cgi/trace",
		ErrorMessage: "timeout",
	}
	deps.prober.results["node-node-jp-slow"] = node.ProbeResult{
		Success:      false,
		TestURL:      "https://cloudflare.com/cdn-cgi/trace",
		ErrorMessage: "timeout",
	}
	deps.prober.results["node-jp-slow"] = node.ProbeResult{
		Success:      false,
		TestURL:      "https://cloudflare.com/cdn-cgi/trace",
		ErrorMessage: "timeout",
	}

	_, err = service.Refresh(ctx, created.ID)
	if !errors.Is(err, tunnel.ErrNoAvailableNodes) {
		t.Fatalf("Refresh() error = %v, want ErrNoAvailableNodes", err)
	}

	got, err := service.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.CurrentNodeID == nil || *got.CurrentNodeID != "node-hk-fast" {
		t.Fatalf("CurrentNodeID = %v, want old node-hk-fast", got.CurrentNodeID)
	}
	if got.Status != domain.TunnelStatusDegraded {
		t.Fatalf("Status = %q, want degraded", got.Status)
	}
	if got.LastRefreshError == "" {
		t.Fatal("LastRefreshError empty, want recorded error")
	}
	if state := deps.runtime.state(created.ID); state == nil || !state.running || state.now != "node-node-hk-fast" {
		t.Fatalf("runtime state = %+v, want old runtime kept", state)
	}

	events, err := service.ListEvents(ctx, created.ID, 10)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events) == 0 || events[0].EventType != "tunnel.refresh_failed" {
		t.Fatalf("events = %+v, want newest refresh_failed", events)
	}
}

func TestTunnelServiceUpdateRunningRebuildsRuntimeAndRefreshSwitchesSelector(t *testing.T) {
	ctx := context.Background()
	service, deps := newTunnelService(t)
	seedTunnelNodes(t, ctx, deps.repos, deps.now, deps.cipher)
	asiaGroupID := seedTunnelGroup(t, ctx, deps.groupService, "亚洲", "^(HK|JP)-")
	usGroupID := seedTunnelGroup(t, ctx, deps.groupService, "美国", "^US-")

	created, err := service.Create(ctx, tunnel.CreateInput{
		Name:    "proxy-a",
		GroupID: asiaGroupID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updated, err := service.Update(ctx, created.ID, tunnel.UpdateInput{
		Name:     "proxy-b",
		GroupID:  usGroupID,
		Username: "new-user",
		Password: "new-pass",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != "proxy-b" || updated.GroupID != usGroupID {
		t.Fatalf("Update() = %+v, want renamed and moved to US group", updated)
	}
	if updated.CurrentNodeID == nil || *updated.CurrentNodeID != "node-us-mid" {
		t.Fatalf("Update() CurrentNodeID = %v, want node-us-mid", updated.CurrentNodeID)
	}
	state := deps.runtime.state(created.ID)
	if state == nil || !state.running {
		t.Fatal("runtime not running after update rebuild")
	}
	if !slices.Equal(state.all, []string{"node-node-us-mid"}) {
		t.Fatalf("runtime all = %v, want rebuilt US snapshot", state.all)
	}

	hkSlow, err := deps.repos.Nodes.GetByID(ctx, "node-hk-slow")
	if err != nil {
		t.Fatalf("Nodes.GetByID(node-hk-slow) error = %v", err)
	}
	hkSlow.Enabled = true
	hkSlow.UpdatedAt = deps.now().Add(time.Minute)
	if err := deps.repos.Nodes.Update(ctx, hkSlow); err != nil {
		t.Fatalf("Nodes.Update(node-hk-slow) error = %v", err)
	}

	if _, err := service.Update(ctx, created.ID, tunnel.UpdateInput{
		Name:    "proxy-c",
		GroupID: asiaGroupID,
	}); err != nil {
		t.Fatalf("Update() second error = %v", err)
	}

	deps.prober.results["node-hk-fast"] = node.ProbeResult{
		Success:   true,
		TestURL:   "https://cloudflare.com/cdn-cgi/trace",
		LatencyMS: 90,
	}
	deps.prober.results["node-hk-slow"] = node.ProbeResult{
		Success:   true,
		TestURL:   "https://cloudflare.com/cdn-cgi/trace",
		LatencyMS: 20,
	}

	refreshed, err := service.Refresh(ctx, created.ID)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if refreshed.CurrentNodeID == nil || *refreshed.CurrentNodeID != "node-hk-slow" {
		t.Fatalf("Refresh() CurrentNodeID = %v, want node-hk-slow", refreshed.CurrentNodeID)
	}
	if deps.runtime.switchCalls == 0 {
		t.Fatal("switchCalls = 0, want selector switch")
	}
	if state := deps.runtime.state(created.ID); state == nil || state.now != "node-node-hk-slow" {
		t.Fatalf("runtime selector now = %+v, want switched to node-node-hk-slow", state)
	}
}

func TestTunnelServiceCreateFailsWithoutAvailableNodesAndRollsBack(t *testing.T) {
	ctx := context.Background()
	service, deps := newTunnelService(t)
	seedTunnelNodes(t, ctx, deps.repos, deps.now, deps.cipher)
	groupID := seedTunnelGroup(t, ctx, deps.groupService, "亚洲", "^(HK|JP)-")

	deps.prober.results["node-hk-fast"] = node.ProbeResult{
		Success:      false,
		TestURL:      "https://cloudflare.com/cdn-cgi/trace",
		ErrorMessage: "timeout",
	}
	deps.prober.results["node-jp-slow"] = node.ProbeResult{
		Success:      false,
		TestURL:      "https://cloudflare.com/cdn-cgi/trace",
		ErrorMessage: "timeout",
	}

	_, err := service.Create(ctx, tunnel.CreateInput{
		Name:    "proxy-a",
		GroupID: groupID,
	})
	if !errors.Is(err, tunnel.ErrNoAvailableNodes) {
		t.Fatalf("Create() error = %v, want ErrNoAvailableNodes", err)
	}

	items, err := deps.repos.Tunnels.List(ctx)
	if err != nil {
		t.Fatalf("Tunnels.List() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("len(Tunnels.List()) = %d, want 0 after rollback", len(items))
	}
}

type tunnelServiceDeps struct {
	repos        *sqlite.Repositories
	groupService *group.Service
	runtime      *fakeRuntimeManager
	prober       *fakeTunnelProber
	cipher       *appcrypto.AESGCM
	now          func() time.Time
}

func newTunnelService(t *testing.T) (*tunnel.Service, *tunnelServiceDeps) {
	t.Helper()

	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "tunnel.db"))
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
	base := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	var tick int
	now := func() time.Time {
		value := base.Add(time.Duration(tick) * time.Second)
		tick++
		return value
	}
	groupService := group.NewService(group.Options{
		Groups: repos.Groups,
		Nodes:  repos.Nodes,
		Now:    now,
	})
	cipher, err := appcrypto.NewAESGCM(make([]byte, 32))
	if err != nil {
		t.Fatalf("NewAESGCM() error = %v", err)
	}
	prober := &fakeTunnelProber{
		results: map[string]node.ProbeResult{
			"node-hk-fast": {Success: true, LatencyMS: 10, TestURL: "https://cloudflare.com/cdn-cgi/trace"},
			"node-jp-slow": {Success: true, LatencyMS: 30, TestURL: "https://cloudflare.com/cdn-cgi/trace"},
			"node-us-mid":  {Success: true, LatencyMS: 20, TestURL: "https://cloudflare.com/cdn-cgi/trace"},
			"node-hk-slow": {Success: true, LatencyMS: 50, TestURL: "https://cloudflare.com/cdn-cgi/trace"},
		},
	}
	runtimeManager := newFakeRuntimeManager()

	service := tunnel.NewService(tunnel.Options{
		Tunnels:        repos.Tunnels,
		TunnelEvents:   repos.TunnelEvents,
		LatencySamples: repos.LatencySamples,
		Groups:         groupService,
		Nodes:          repos.Nodes,
		Cipher:         cipher,
		Prober:         prober,
		Runtime:        runtimeManager,
		PortAllocator:  singbox.NewPortAllocator(),
		Renderer:       singbox.NewConfigRenderer(),
		RuntimeRoot:    filepath.Join(t.TempDir(), "runtime"),
		Now:            now,
	})
	t.Cleanup(func() {
		_ = runtimeManager.Close()
		_ = service.Close()
	})

	return service, &tunnelServiceDeps{
		repos:        repos,
		groupService: groupService,
		runtime:      runtimeManager,
		prober:       prober,
		cipher:       cipher,
		now:          now,
	}
}

func seedTunnelGroup(t *testing.T, ctx context.Context, service *group.Service, name, filter string) string {
	t.Helper()
	item, err := service.Create(ctx, group.CreateInput{
		Name:        name,
		FilterRegex: filter,
		Description: name,
	})
	if err != nil {
		t.Fatalf("Group.Create() error = %v", err)
	}
	return item.ID
}

func seedTunnelNodes(t *testing.T, ctx context.Context, repos *sqlite.Repositories, now func() time.Time, cipher *appcrypto.AESGCM) {
	t.Helper()

	nodes := []*domain.Node{
		{
			ID:                "node-hk-fast",
			Name:              "HK-fast",
			DedupeFingerprint: "hk-fast",
			SourceKind:        domain.NodeSourceManual,
			Protocol:          "trojan",
			Server:            "1.1.1.1",
			ServerPort:        443,
			Enabled:           true,
			LastStatus:        domain.NodeStatusUnknown,
			CreatedAt:         now(),
			UpdatedAt:         now(),
		},
		{
			ID:                "node-jp-slow",
			Name:              "JP-slow",
			DedupeFingerprint: "jp-slow",
			SourceKind:        domain.NodeSourceManual,
			Protocol:          "trojan",
			Server:            "2.2.2.2",
			ServerPort:        443,
			Enabled:           true,
			LastStatus:        domain.NodeStatusUnknown,
			CreatedAt:         now().Add(time.Minute),
			UpdatedAt:         now().Add(time.Minute),
		},
		{
			ID:                "node-us-mid",
			Name:              "US-mid",
			DedupeFingerprint: "us-mid",
			SourceKind:        domain.NodeSourceManual,
			Protocol:          "trojan",
			Server:            "3.3.3.3",
			ServerPort:        443,
			Enabled:           true,
			LastStatus:        domain.NodeStatusUnknown,
			CreatedAt:         now().Add(2 * time.Minute),
			UpdatedAt:         now().Add(2 * time.Minute),
		},
		{
			ID:                "node-hk-slow",
			Name:              "HK-slow",
			DedupeFingerprint: "hk-slow",
			SourceKind:        domain.NodeSourceManual,
			Protocol:          "trojan",
			Server:            "4.4.4.4",
			ServerPort:        443,
			Enabled:           false,
			LastStatus:        domain.NodeStatusUnknown,
			CreatedAt:         now().Add(3 * time.Minute),
			UpdatedAt:         now().Add(3 * time.Minute),
		},
	}

	for _, item := range nodes {
		ciphertext, nonce, err := cipher.Encrypt([]byte(`{"password":"secret"}`), []byte("node:credential:"+item.ID))
		if err != nil {
			t.Fatalf("Encrypt(%s) error = %v", item.ID, err)
		}
		item.CredentialCiphertext = ciphertext
		item.CredentialNonce = nonce
		if err := repos.Nodes.Create(ctx, item); err != nil {
			t.Fatalf("Nodes.Create(%s) error = %v", item.ID, err)
		}
	}
}

type fakeTunnelProber struct {
	results map[string]node.ProbeResult
	err     error
}

func (f *fakeTunnelProber) Probe(ctx context.Context, target node.ProbeTarget) (node.ProbeResult, error) {
	if f.err != nil {
		return node.ProbeResult{}, f.err
	}
	if result, ok := f.results[target.ID]; ok {
		return result, nil
	}
	if result, ok := f.results[target.Name]; ok {
		return result, nil
	}
	return node.ProbeResult{
		Success:   true,
		LatencyMS: 40,
		TestURL:   "https://cloudflare.com/cdn-cgi/trace",
	}, nil
}

type fakeRuntimeManager struct {
	startCalls  int
	stopCalls   int
	switchCalls int
	states      map[string]*fakeRuntimeState
	startErr    error
	stopErr     error
	switchErr   error
}

type fakeRuntimeState struct {
	running bool
	all     []string
	now     string
}

func newFakeRuntimeManager() *fakeRuntimeManager {
	return &fakeRuntimeManager{
		states: make(map[string]*fakeRuntimeState),
	}
}

func (f *fakeRuntimeManager) Start(ctx context.Context, tunnelID string, layout singbox.RuntimeLayout, config []byte) error {
	if f.startErr != nil {
		return f.startErr
	}
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

	state := &fakeRuntimeState{running: true}
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
		if now, _ := outbound["default"].(string); now != "" {
			state.now = now
		}
	}

	f.states[tunnelID] = state
	f.startCalls++
	return nil
}

func (f *fakeRuntimeManager) Stop(ctx context.Context, tunnelID string) error {
	if f.stopErr != nil {
		return f.stopErr
	}
	if state := f.states[tunnelID]; state != nil {
		state.running = false
	}
	f.stopCalls++
	return nil
}

func (f *fakeRuntimeManager) Delete(ctx context.Context, tunnelID string) error {
	delete(f.states, tunnelID)
	return nil
}

func (f *fakeRuntimeManager) GetSelector(ctx context.Context, tunnelID string, controllerPort int, secret string) (*singbox.ProxyInfo, error) {
	state := f.states[tunnelID]
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

func (f *fakeRuntimeManager) SwitchSelector(ctx context.Context, tunnelID string, controllerPort int, secret, outbound string) error {
	if f.switchErr != nil {
		return f.switchErr
	}
	state := f.states[tunnelID]
	if state == nil {
		return errors.New("runtime missing")
	}
	state.now = outbound
	f.switchCalls++
	return nil
}

func (f *fakeRuntimeManager) Close() error {
	return nil
}

func (f *fakeRuntimeManager) state(tunnelID string) *fakeRuntimeState {
	return f.states[tunnelID]
}
