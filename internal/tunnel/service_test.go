package tunnel_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
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
	storedAfterCreate, err := deps.repos.Tunnels.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("Tunnels.GetByID() after create error = %v", err)
	}
	if storedAfterCreate.RuntimeConfigJSON == "" {
		t.Fatal("RuntimeConfigJSON empty after create")
	}

	runtimeState := deps.runtime.state(created.ID)
	if runtimeState == nil || !runtimeState.running {
		t.Fatal("runtime not running after create")
	}
	if runtimeState.now != "node-node-hk-fast" {
		t.Fatalf("runtime selector now = %q, want node-node-hk-fast", runtimeState.now)
	}
	if !slices.Equal(runtimeState.all, []string{"node-node-hk-fast", "node-node-jp-slow"}) {
		t.Fatalf("runtime all = %v, want enabled group snapshot", runtimeState.all)
	}

	waitForTunnelSamples(t, ctx, deps.repos, created.ID, 2)
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
	advanceTunnelClock(deps.now, 6*time.Minute)

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

func TestTunnelServiceRebuildFailureFallsBackToStoredRuntimeConfig(t *testing.T) {
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

	storedBefore, err := deps.repos.Tunnels.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("Tunnels.GetByID() error = %v", err)
	}
	if storedBefore.RuntimeConfigJSON == "" {
		t.Fatal("RuntimeConfigJSON empty before rebuild")
	}

	layout := singbox.NewRuntimeLayout(deps.runtimeRoot, created.ID)
	if err := os.Remove(layout.ConfigPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Remove(config) error = %v", err)
	}

	deps.runtime.startErrors = []error{errors.New("boom"), nil}

	_, err = service.Update(ctx, created.ID, tunnel.UpdateInput{
		Name:    "proxy-b",
		GroupID: usGroupID,
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("Update() error = %v, want boom", err)
	}

	got, err := service.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != domain.TunnelStatusDegraded {
		t.Fatalf("Status = %q, want degraded", got.Status)
	}
	if got.CurrentNodeID == nil || *got.CurrentNodeID != "node-hk-fast" {
		t.Fatalf("CurrentNodeID = %v, want old node-hk-fast", got.CurrentNodeID)
	}

	state := deps.runtime.state(created.ID)
	if state == nil || !state.running {
		t.Fatalf("runtime state = %+v, want running fallback runtime", state)
	}
	if !slices.Equal(state.all, []string{"node-node-hk-fast", "node-node-jp-slow"}) {
		t.Fatalf("runtime all = %v, want old asia snapshot after fallback", state.all)
	}

	storedAfter, err := deps.repos.Tunnels.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("Tunnels.GetByID() after rebuild error = %v", err)
	}
	if storedAfter.RuntimeConfigJSON != storedBefore.RuntimeConfigJSON {
		t.Fatalf("RuntimeConfigJSON changed after failed rebuild")
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
	advanceTunnelClock(deps.now, 6*time.Minute)

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
	hkFastRelease := make(chan struct{})
	jpSlowRelease := make(chan struct{})
	deps.prober.blocks = map[string]chan struct{}{
		"node-hk-fast": hkFastRelease,
		"node-jp-slow": jpSlowRelease,
	}

	refreshed, err := service.Refresh(ctx, created.ID)
	close(hkFastRelease)
	close(jpSlowRelease)
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
	if state := deps.runtime.state(created.ID); state == nil || !slices.Equal(state.all, []string{"node-node-hk-fast", "node-node-jp-slow", "node-node-hk-slow"}) {
		t.Fatalf("runtime all = %+v, want all asia trojan nodes", state)
	}
}

func TestTunnelServiceCreateFiltersRuntimePoolToCurrentProtocol(t *testing.T) {
	ctx := context.Background()
	service, deps := newTunnelService(t)
	seedTunnelNodes(t, ctx, deps.repos, deps.now, deps.cipher)
	groupID := seedTunnelGroup(t, ctx, deps.groupService, "亚洲", "^(HK|JP)-")

	hy2Node := &domain.Node{
		ID:                "node-hk-hy2",
		Name:              "HK-hy2",
		DedupeFingerprint: "hk-hy2",
		SourceKind:        domain.NodeSourceManual,
		Protocol:          "hysteria2",
		Server:            "5.5.5.5",
		ServerPort:        443,
		Enabled:           true,
		LastStatus:        domain.NodeStatusUnknown,
		CreatedAt:         deps.now().Add(4 * time.Minute),
		UpdatedAt:         deps.now().Add(4 * time.Minute),
	}
	ciphertext, nonce, err := deps.cipher.Encrypt([]byte(`{"password":"secret"}`), []byte("node:credential:"+hy2Node.ID))
	if err != nil {
		t.Fatalf("Encrypt(%s) error = %v", hy2Node.ID, err)
	}
	hy2Node.CredentialCiphertext = ciphertext
	hy2Node.CredentialNonce = nonce
	hy2Node.TLSJSON = `{"enabled":true,"server_name":"www.apple.com","insecure":true}`
	if err := deps.repos.Nodes.Create(ctx, hy2Node); err != nil {
		t.Fatalf("Nodes.Create(%s) error = %v", hy2Node.ID, err)
	}

	deps.prober.results["node-hk-fast"] = node.ProbeResult{Success: true, LatencyMS: 10, TestURL: "https://cloudflare.com/cdn-cgi/trace"}
	deps.prober.results["node-jp-slow"] = node.ProbeResult{Success: true, LatencyMS: 30, TestURL: "https://cloudflare.com/cdn-cgi/trace"}
	deps.prober.results["node-hk-hy2"] = node.ProbeResult{Success: true, LatencyMS: 20, TestURL: "https://cloudflare.com/cdn-cgi/trace"}
	latency := int64(10)
	if err := deps.repos.LatencySamples.Create(ctx, &domain.LatencySample{
		ID:        "sample-hk-fast-cached",
		NodeID:    "node-hk-fast",
		TestURL:   "https://cloudflare.com/cdn-cgi/trace",
		LatencyMS: &latency,
		Success:   true,
		CreatedAt: deps.now().UTC(),
	}); err != nil {
		t.Fatalf("LatencySamples.Create() error = %v", err)
	}

	created, err := service.Create(ctx, tunnel.CreateInput{
		Name:    "proxy-a",
		GroupID: groupID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.CurrentNodeID == nil || *created.CurrentNodeID != "node-hk-fast" {
		t.Fatalf("CurrentNodeID = %v, want node-hk-fast", created.CurrentNodeID)
	}

	state := deps.runtime.state(created.ID)
	if state == nil || !state.running {
		t.Fatal("runtime not running after create")
	}
	if !slices.Equal(state.all, []string{"node-node-hk-fast", "node-node-jp-slow"}) {
		t.Fatalf("runtime all = %v, want only trojan nodes in selector pool", state.all)
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

func TestTunnelServiceCreateUsesFirstSuccessfulProbeAndCachesBackgroundResults(t *testing.T) {
	ctx := context.Background()
	service, deps := newTunnelService(t)
	seedTunnelNodes(t, ctx, deps.repos, deps.now, deps.cipher)
	groupID := seedTunnelGroup(t, ctx, deps.groupService, "亚洲", "^(HK|JP)-")

	firstProbeRelease := make(chan struct{})
	backgroundRelease := make(chan struct{})
	deps.prober.results = map[string]node.ProbeResult{
		"node-hk-fast": {Success: true, LatencyMS: 10, TestURL: "https://cloudflare.com/cdn-cgi/trace"},
		"node-jp-slow": {Success: true, LatencyMS: 30, TestURL: "https://cloudflare.com/cdn-cgi/trace"},
	}
	deps.prober.blocks = map[string]chan struct{}{
		"node-hk-fast": firstProbeRelease,
		"node-jp-slow": backgroundRelease,
	}
	deps.prober.started = make(chan string, 2)

	type createResult struct {
		view *tunnel.View
		err  error
	}
	resultCh := make(chan createResult, 1)
	go func() {
		view, err := service.Create(ctx, tunnel.CreateInput{
			Name:    "proxy-a",
			GroupID: groupID,
		})
		resultCh <- createResult{view: view, err: err}
	}()

	waitForProbeStarts(t, deps.prober.started, "node-hk-fast", "node-jp-slow")
	close(firstProbeRelease)

	var created *tunnel.View
	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("Create() error = %v", result.err)
		}
		created = result.view
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Create() did not return after first successful probe")
	}
	if created.CurrentNodeID == nil || *created.CurrentNodeID != "node-hk-fast" {
		t.Fatalf("CurrentNodeID = %v, want node-hk-fast", created.CurrentNodeID)
	}

	samples, err := deps.repos.LatencySamples.ListByTunnelID(ctx, created.ID, 10)
	if err != nil {
		t.Fatalf("LatencySamples.ListByTunnelID() error = %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("len(samples) before background release = %d, want 1", len(samples))
	}

	close(backgroundRelease)
	waitForTunnelSamples(t, ctx, deps.repos, created.ID, 2)
}

func TestTunnelServiceCreateReusesRecentSuccessfulCache(t *testing.T) {
	ctx := context.Background()
	service, deps := newTunnelService(t)
	seedTunnelNodes(t, ctx, deps.repos, deps.now, deps.cipher)
	groupID := seedTunnelGroup(t, ctx, deps.groupService, "亚洲", "^(HK|JP)-")

	checkedAt := deps.now().UTC()
	latency := int64(12)
	if err := deps.repos.LatencySamples.Create(ctx, &domain.LatencySample{
		ID:        "sample-hk-fast",
		NodeID:    "node-hk-fast",
		TestURL:   "https://cloudflare.com/cdn-cgi/trace",
		LatencyMS: &latency,
		Success:   true,
		CreatedAt: checkedAt,
	}); err != nil {
		t.Fatalf("LatencySamples.Create() error = %v", err)
	}

	created, err := service.Create(ctx, tunnel.CreateInput{
		Name:    "proxy-a",
		GroupID: groupID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.CurrentNodeID == nil || *created.CurrentNodeID != "node-hk-fast" {
		t.Fatalf("CurrentNodeID = %v, want node-hk-fast from cache", created.CurrentNodeID)
	}
	if deps.prober.CallCount() != 0 {
		t.Fatalf("prober.calls = %d, want 0 when cache is reused", deps.prober.CallCount())
	}
}

type tunnelServiceDeps struct {
	repos        *sqlite.Repositories
	groupService *group.Service
	runtime      *fakeRuntimeManager
	prober       *fakeTunnelProber
	cipher       *appcrypto.AESGCM
	now          func() time.Time
	runtimeRoot  string
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
	var mu sync.Mutex
	var tick int
	now := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
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
	runtimeRoot := filepath.Join(t.TempDir(), "runtime")

	service := tunnel.NewService(tunnel.Options{
		Tunnels:        repos.Tunnels,
		TunnelEvents:   repos.TunnelEvents,
		LatencySamples: repos.LatencySamples,
		Groups:         groupService,
		Nodes:          repos.Nodes,
		Cipher:         cipher,
		Prober:         prober,
		ProbeCacheTTL:  5 * time.Minute,
		Runtime:        runtimeManager,
		PortAllocator:  singbox.NewPortAllocator(),
		Renderer:       singbox.NewConfigRenderer(),
		RuntimeRoot:    runtimeRoot,
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
		runtimeRoot:  runtimeRoot,
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
	mu      sync.Mutex
	calls   int
	results map[string]node.ProbeResult
	err     error
	started chan string
	blocks  map[string]chan struct{}
}

func (f *fakeTunnelProber) Probe(ctx context.Context, target node.ProbeTarget) (node.ProbeResult, error) {
	f.mu.Lock()
	f.calls++
	started := f.started
	block := f.blocks[target.ID]
	results := f.results
	err := f.err
	f.mu.Unlock()

	if started != nil {
		select {
		case started <- target.ID:
		default:
		}
	}
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return node.ProbeResult{}, ctx.Err()
		}
	}
	if err != nil {
		return node.ProbeResult{}, err
	}
	if result, ok := results[target.ID]; ok {
		return result, nil
	}
	if result, ok := results[target.Name]; ok {
		return result, nil
	}
	return node.ProbeResult{
		Success:   true,
		LatencyMS: 40,
		TestURL:   "https://cloudflare.com/cdn-cgi/trace",
	}, nil
}

func (f *fakeTunnelProber) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func waitForProbeStarts(t *testing.T, started <-chan string, wants ...string) {
	t.Helper()
	pending := make(map[string]struct{}, len(wants))
	for _, want := range wants {
		pending[want] = struct{}{}
	}
	timeout := time.After(200 * time.Millisecond)
	for len(pending) > 0 {
		select {
		case got := <-started:
			delete(pending, got)
		case <-timeout:
			t.Fatalf("probes %v did not all start", wants)
		}
	}
}

func waitForTunnelSamples(t *testing.T, ctx context.Context, repos *sqlite.Repositories, tunnelID string, want int) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		items, err := repos.LatencySamples.ListByTunnelID(ctx, tunnelID, 10)
		if err != nil {
			t.Fatalf("LatencySamples.ListByTunnelID() error = %v", err)
		}
		if len(items) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	items, err := repos.LatencySamples.ListByTunnelID(ctx, tunnelID, 10)
	if err != nil {
		t.Fatalf("LatencySamples.ListByTunnelID() error = %v", err)
	}
	t.Fatalf("len(samples) = %d, want %d", len(items), want)
}

func advanceTunnelClock(now func() time.Time, duration time.Duration) {
	steps := int(duration / time.Second)
	for range steps {
		_ = now()
	}
}

type fakeRuntimeManager struct {
	startCalls  int
	stopCalls   int
	switchCalls int
	states      map[string]*fakeRuntimeState
	startErr    error
	startErrors []error
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
	if len(f.startErrors) > 0 {
		err := f.startErrors[0]
		f.startErrors = f.startErrors[1:]
		if err != nil {
			f.startCalls++
			return err
		}
	}
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
	var final string
	hasSelector := false
	for _, outbound := range payload.Outbounds {
		tag, _ := outbound["tag"].(string)
		switch outbound["type"] {
		case "selector":
			hasSelector = true
			if raw, ok := outbound["outbounds"].([]any); ok {
				for _, item := range raw {
					if value, ok := item.(string); ok {
						if !slices.Contains(state.all, value) {
							state.all = append(state.all, value)
						}
					}
				}
			}
			if now, _ := outbound["default"].(string); now != "" {
				state.now = now
			}
		case "direct":
			continue
		default:
			if tag != "" && !hasSelector && !slices.Contains(state.all, tag) {
				state.all = append(state.all, tag)
			}
		}
	}
	if route, err := parseRuntimeRouteFinal(config); err == nil {
		final = route
	}
	if state.now == "" {
		state.now = final
	}
	if len(state.all) == 0 && final != "" {
		state.all = append(state.all, final)
	}

	f.states[tunnelID] = state
	f.startCalls++
	return nil
}

func parseRuntimeRouteFinal(config []byte) (string, error) {
	var payload struct {
		Route map[string]any `json:"route"`
	}
	if err := json.Unmarshal(config, &payload); err != nil {
		return "", err
	}
	value, _ := payload.Route["final"].(string)
	return value, nil
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
