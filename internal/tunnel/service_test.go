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
	if created.CurrentNodeID == nil || *created.CurrentNodeID == "" {
		t.Fatalf("CurrentNodeID = %v, want selected node", created.CurrentNodeID)
	}
	if !created.HasAuth {
		t.Fatal("HasAuth = false, want true")
	}
	if got, want := created.RuntimeDir, filepath.Join(deps.runtimeRoot, "亚洲-proxy-a"); got != want {
		t.Fatalf("RuntimeDir = %q, want %q", got, want)
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
	if runtimeState.now != "node-"+*created.CurrentNodeID {
		t.Fatalf("runtime selector now = %q, want node-%s", runtimeState.now, *created.CurrentNodeID)
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

func TestTunnelServiceStartReusesStoredRuntimeConfigWithoutReprobe(t *testing.T) {
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
	if created.CurrentNodeID == nil || *created.CurrentNodeID == "" {
		t.Fatalf("Create() CurrentNodeID = %v, want selected node", created.CurrentNodeID)
	}
	if _, err := service.Stop(ctx, created.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	beforeProbeCalls := deps.prober.CallCount()
	deps.prober.err = errors.New("probe should not be called on start")

	started, err := service.Start(ctx, created.ID)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if started.Status != domain.TunnelStatusRunning {
		t.Fatalf("Start() status = %q, want running", started.Status)
	}
	if started.CurrentNodeID == nil || *started.CurrentNodeID != *created.CurrentNodeID {
		t.Fatalf("Start() CurrentNodeID = %v, want %s", started.CurrentNodeID, *created.CurrentNodeID)
	}
	if deps.prober.CallCount() != beforeProbeCalls {
		t.Fatalf("prober calls = %d, want %d without reprobe", deps.prober.CallCount(), beforeProbeCalls)
	}
	state := deps.runtime.state(created.ID)
	if state == nil || !state.running {
		t.Fatalf("runtime state = %+v, want running", state)
	}
	if state.now != "node-"+*created.CurrentNodeID {
		t.Fatalf("runtime selector now = %q, want node-%s", state.now, *created.CurrentNodeID)
	}
	if !slices.Equal(state.all, []string{"node-node-hk-fast", "node-node-jp-slow"}) {
		t.Fatalf("runtime all = %v, want stored selector pool", state.all)
	}
}

func TestTunnelServiceInitializeRestoresRunningTunnelWithoutReprobe(t *testing.T) {
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
	if created.CurrentNodeID == nil || *created.CurrentNodeID == "" {
		t.Fatalf("Create() CurrentNodeID = %v, want selected node", created.CurrentNodeID)
	}

	deps.runtime.states = make(map[string]*fakeRuntimeState)
	beforeProbeCalls := deps.prober.CallCount()
	deps.prober.err = errors.New("probe should not be called on initialize")

	if err := service.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	got, err := service.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != domain.TunnelStatusRunning {
		t.Fatalf("Status = %q, want running after initialize restore", got.Status)
	}
	if got.CurrentNodeID == nil || *got.CurrentNodeID != *created.CurrentNodeID {
		t.Fatalf("CurrentNodeID = %v, want %s", got.CurrentNodeID, *created.CurrentNodeID)
	}
	if deps.prober.CallCount() != beforeProbeCalls {
		t.Fatalf("prober calls = %d, want %d without reprobe", deps.prober.CallCount(), beforeProbeCalls)
	}
	state := deps.runtime.state(created.ID)
	if state == nil || !state.running {
		t.Fatalf("runtime state = %+v, want restored running state", state)
	}
	if state.now != "node-"+*created.CurrentNodeID {
		t.Fatalf("runtime selector now = %q, want node-%s", state.now, *created.CurrentNodeID)
	}
}

func TestTunnelServiceUpdateStoppedReplacesRuntimeDirWithTunnelName(t *testing.T) {
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
	oldRuntimeDir := created.RuntimeDir

	if _, err := service.Stop(ctx, created.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	updated, err := service.Update(ctx, created.ID, tunnel.UpdateInput{
		Name:    "proxy-b",
		GroupID: groupID,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.RuntimeDir == oldRuntimeDir {
		t.Fatalf("RuntimeDir = %q, want new dir after rename", updated.RuntimeDir)
	}
	if got, want := updated.RuntimeDir, filepath.Join(deps.runtimeRoot, "亚洲-proxy-b"); got != want {
		t.Fatalf("RuntimeDir = %q, want %q", got, want)
	}
	if _, err := os.Stat(oldRuntimeDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old runtime dir still exists, err = %v", err)
	}

	started, err := service.Start(ctx, created.ID)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if started.RuntimeDir != updated.RuntimeDir {
		t.Fatalf("Start() RuntimeDir = %q, want %q", started.RuntimeDir, updated.RuntimeDir)
	}
	if _, err := os.Stat(started.RuntimeDir); err != nil {
		t.Fatalf("Stat(runtimeDir) error = %v", err)
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
	if created.CurrentNodeID == nil || *created.CurrentNodeID == "" {
		t.Fatalf("Create() CurrentNodeID = %v, want selected node", created.CurrentNodeID)
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
	if got.CurrentNodeID == nil || *got.CurrentNodeID != *created.CurrentNodeID {
		t.Fatalf("CurrentNodeID = %v, want keep %s", got.CurrentNodeID, *created.CurrentNodeID)
	}
	if got.Status != domain.TunnelStatusDegraded {
		t.Fatalf("Status = %q, want degraded", got.Status)
	}
	if got.LastRefreshError == "" {
		t.Fatal("LastRefreshError empty, want recorded error")
	}
	if state := deps.runtime.state(created.ID); state == nil || !state.running || state.now != "node-"+*created.CurrentNodeID {
		t.Fatalf("runtime state = %+v, want kept runtime on node-%s", state, *created.CurrentNodeID)
	}

	events, err := service.ListEvents(ctx, created.ID, 10)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events) == 0 || events[0].EventType != "tunnel.refresh_failed" {
		t.Fatalf("events = %+v, want newest refresh_failed", events)
	}
}

func TestTunnelServiceRefreshExcludesCurrentNodeAndWaitsForAlternative(t *testing.T) {
	ctx := context.Background()
	service, deps := newTunnelService(t)
	seedTunnelNodes(t, ctx, deps.repos, deps.now, deps.cipher)
	groupID := seedTunnelGroup(t, ctx, deps.groupService, "亚洲", "^(HK|JP)-")
	latency := int64(10)
	if err := deps.repos.LatencySamples.Create(ctx, &domain.LatencySample{
		ID:        "sample-hk-fast-refresh-current",
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
		t.Fatalf("Create() CurrentNodeID = %v, want node-hk-fast", created.CurrentNodeID)
	}

	advanceTunnelClock(deps.now, 6*time.Minute)
	releaseAlternative := make(chan struct{})
	deps.prober.ResetHistory()
	deps.prober.blocks = map[string]chan struct{}{
		"node-jp-slow": releaseAlternative,
	}

	type refreshResult struct {
		view *tunnel.View
		err  error
	}
	resultCh := make(chan refreshResult, 1)
	go func() {
		view, err := service.Refresh(ctx, created.ID)
		resultCh <- refreshResult{view: view, err: err}
	}()

	select {
	case result := <-resultCh:
		t.Fatalf("Refresh() returned before alternative probe released: %+v, err = %v", result.view, result.err)
	case <-time.After(40 * time.Millisecond):
	}

	close(releaseAlternative)
	result := <-resultCh
	if result.err != nil {
		t.Fatalf("Refresh() error = %v", result.err)
	}
	if result.view.CurrentNodeID == nil || *result.view.CurrentNodeID != "node-jp-slow" {
		t.Fatalf("Refresh() CurrentNodeID = %v, want node-jp-slow", result.view.CurrentNodeID)
	}

	callIDs := deps.prober.CallIDs()
	if !slices.Equal(callIDs, []string{"node-jp-slow"}) {
		t.Fatalf("prober call ids = %v, want only alternative node-jp-slow", callIDs)
	}
}

func TestTunnelServiceRefreshUsesCachedAlternativeWithoutWaiting(t *testing.T) {
	ctx := context.Background()
	service, deps := newTunnelService(t)
	seedTunnelNodes(t, ctx, deps.repos, deps.now, deps.cipher)
	groupID := seedTunnelGroup(t, ctx, deps.groupService, "亚洲", "^(HK|JP)-")
	currentLatency := int64(10)
	if err := deps.repos.LatencySamples.Create(ctx, &domain.LatencySample{
		ID:        "sample-hk-fast-refresh-cache-current",
		NodeID:    "node-hk-fast",
		TestURL:   "https://cloudflare.com/cdn-cgi/trace",
		LatencyMS: &currentLatency,
		Success:   true,
		CreatedAt: deps.now().UTC(),
	}); err != nil {
		t.Fatalf("LatencySamples.Create() current error = %v", err)
	}

	created, err := service.Create(ctx, tunnel.CreateInput{
		Name:    "proxy-a",
		GroupID: groupID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.CurrentNodeID == nil || *created.CurrentNodeID != "node-hk-fast" {
		t.Fatalf("Create() CurrentNodeID = %v, want node-hk-fast", created.CurrentNodeID)
	}

	cachedLatency := int64(18)
	if err := deps.repos.LatencySamples.Create(ctx, &domain.LatencySample{
		ID:        "sample-jp-slow-refresh-cache-alt",
		NodeID:    "node-jp-slow",
		TestURL:   "https://cloudflare.com/cdn-cgi/trace",
		LatencyMS: &cachedLatency,
		Success:   true,
		CreatedAt: deps.now().UTC(),
	}); err != nil {
		t.Fatalf("LatencySamples.Create() alternative error = %v", err)
	}

	releaseUnexpectedProbe := make(chan struct{})
	deps.prober.ResetHistory()
	deps.prober.blocks = map[string]chan struct{}{
		"node-jp-slow": releaseUnexpectedProbe,
	}

	type refreshResult struct {
		view *tunnel.View
		err  error
	}
	resultCh := make(chan refreshResult, 1)
	go func() {
		view, err := service.Refresh(ctx, created.ID)
		resultCh <- refreshResult{view: view, err: err}
	}()

	var result refreshResult
	select {
	case result = <-resultCh:
	case <-time.After(40 * time.Millisecond):
		t.Fatal("Refresh() did not return immediately on cached alternative")
	}

	if result.err != nil {
		t.Fatalf("Refresh() error = %v", result.err)
	}
	if result.view.CurrentNodeID == nil || *result.view.CurrentNodeID != "node-jp-slow" {
		t.Fatalf("Refresh() CurrentNodeID = %v, want cached node-jp-slow", result.view.CurrentNodeID)
	}
	if deps.prober.CallCount() != 0 {
		t.Fatalf("prober calls = %d, want 0 on cache hit", deps.prober.CallCount())
	}
}

func TestTunnelServiceRefreshUsesFirstSuccessfulProbeAndCachesBackgroundResults(t *testing.T) {
	ctx := context.Background()
	service, deps := newTunnelService(t)
	seedTunnelNodes(t, ctx, deps.repos, deps.now, deps.cipher)
	groupID := seedTunnelGroup(t, ctx, deps.groupService, "亚洲", "^(HK|JP)-")
	currentLatency := int64(10)
	if err := deps.repos.LatencySamples.Create(ctx, &domain.LatencySample{
		ID:        "sample-hk-fast-refresh-first-current",
		NodeID:    "node-hk-fast",
		TestURL:   "https://cloudflare.com/cdn-cgi/trace",
		LatencyMS: &currentLatency,
		Success:   true,
		CreatedAt: deps.now().UTC(),
	}); err != nil {
		t.Fatalf("LatencySamples.Create() current error = %v", err)
	}

	created, err := service.Create(ctx, tunnel.CreateInput{
		Name:    "proxy-a",
		GroupID: groupID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.CurrentNodeID == nil || *created.CurrentNodeID != "node-hk-fast" {
		t.Fatalf("Create() CurrentNodeID = %v, want node-hk-fast", created.CurrentNodeID)
	}

	hkSlow, err := deps.repos.Nodes.GetByID(ctx, "node-hk-slow")
	if err != nil {
		t.Fatalf("Nodes.GetByID(node-hk-slow) error = %v", err)
	}
	hkSlow.Enabled = true
	hkSlow.UpdatedAt = deps.now().UTC()
	if err := deps.repos.Nodes.Update(ctx, hkSlow); err != nil {
		t.Fatalf("Nodes.Update(node-hk-slow) error = %v", err)
	}

	advanceTunnelClock(deps.now, 6*time.Minute)
	firstProbeRelease := make(chan struct{})
	backgroundRelease := make(chan struct{})
	deps.prober.ResetHistory()
	deps.prober.blocks = map[string]chan struct{}{
		"node-jp-slow": firstProbeRelease,
		"node-hk-slow": backgroundRelease,
	}
	deps.prober.started = make(chan string, 2)

	type refreshResult struct {
		view *tunnel.View
		err  error
	}
	resultCh := make(chan refreshResult, 1)
	go func() {
		view, err := service.Refresh(ctx, created.ID)
		resultCh <- refreshResult{view: view, err: err}
	}()

	waitForProbeStarts(t, deps.prober.started, "node-jp-slow", "node-hk-slow")
	close(firstProbeRelease)

	var refreshed *tunnel.View
	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("Refresh() error = %v", result.err)
		}
		refreshed = result.view
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Refresh() did not return after first successful probe")
	}

	if refreshed.CurrentNodeID == nil || *refreshed.CurrentNodeID != "node-jp-slow" {
		t.Fatalf("Refresh() CurrentNodeID = %v, want node-jp-slow", refreshed.CurrentNodeID)
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

func TestTunnelServiceCreatePublishesGroupMemberUpdateForBackgroundProbe(t *testing.T) {
	ctx := context.Background()
	service, deps := newTunnelService(t)
	seedTunnelNodes(t, ctx, deps.repos, deps.now, deps.cipher)
	groupID := seedTunnelGroup(t, ctx, deps.groupService, "亚洲", "^(HK|JP)-")

	updates, unsubscribe, err := deps.groupService.SubscribeMemberUpdates(ctx, groupID)
	if err != nil {
		t.Fatalf("SubscribeMemberUpdates() error = %v", err)
	}
	defer unsubscribe()

	firstProbeRelease := make(chan struct{})
	backgroundRelease := make(chan struct{})
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
	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("Create() error = %v", result.err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Create() did not return after first successful probe")
	}

	close(backgroundRelease)
	update := waitForGroupMemberUpdate(t, updates, "node-jp-slow")
	if update.LastLatencyMS == nil || *update.LastLatencyMS != 30 {
		t.Fatalf("background update latency = %v, want 30", update.LastLatencyMS)
	}
	if update.LastStatus != domain.NodeStatusHealthy {
		t.Fatalf("background update status = %q, want healthy", update.LastStatus)
	}
}

func TestTunnelServiceRefreshPublishesGroupMemberUpdateForBackgroundProbe(t *testing.T) {
	ctx := context.Background()
	service, deps := newTunnelService(t)
	seedTunnelNodes(t, ctx, deps.repos, deps.now, deps.cipher)
	groupID := seedTunnelGroup(t, ctx, deps.groupService, "亚洲", "^(HK|JP)-")
	currentLatency := int64(10)
	if err := deps.repos.LatencySamples.Create(ctx, &domain.LatencySample{
		ID:        "sample-hk-fast-refresh-stream-current",
		NodeID:    "node-hk-fast",
		TestURL:   "https://cloudflare.com/cdn-cgi/trace",
		LatencyMS: &currentLatency,
		Success:   true,
		CreatedAt: deps.now().UTC(),
	}); err != nil {
		t.Fatalf("LatencySamples.Create() current error = %v", err)
	}

	created, err := service.Create(ctx, tunnel.CreateInput{
		Name:    "proxy-a",
		GroupID: groupID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.CurrentNodeID == nil || *created.CurrentNodeID != "node-hk-fast" {
		t.Fatalf("Create() CurrentNodeID = %v, want node-hk-fast", created.CurrentNodeID)
	}

	hkSlow, err := deps.repos.Nodes.GetByID(ctx, "node-hk-slow")
	if err != nil {
		t.Fatalf("Nodes.GetByID(node-hk-slow) error = %v", err)
	}
	hkSlow.Enabled = true
	hkSlow.UpdatedAt = deps.now().UTC()
	if err := deps.repos.Nodes.Update(ctx, hkSlow); err != nil {
		t.Fatalf("Nodes.Update(node-hk-slow) error = %v", err)
	}

	advanceTunnelClock(deps.now, 6*time.Minute)
	updates, unsubscribe, err := deps.groupService.SubscribeMemberUpdates(ctx, groupID)
	if err != nil {
		t.Fatalf("SubscribeMemberUpdates() error = %v", err)
	}
	defer unsubscribe()
	deps.prober.results["node-hk-slow"] = node.ProbeResult{
		Success:   true,
		LatencyMS: 20,
		TestURL:   "https://cloudflare.com/cdn-cgi/trace",
	}

	firstProbeRelease := make(chan struct{})
	backgroundRelease := make(chan struct{})
	deps.prober.blocks = map[string]chan struct{}{
		"node-jp-slow": firstProbeRelease,
		"node-hk-slow": backgroundRelease,
	}
	deps.prober.started = make(chan string, 2)

	type refreshResult struct {
		view *tunnel.View
		err  error
	}
	resultCh := make(chan refreshResult, 1)
	go func() {
		view, err := service.Refresh(ctx, created.ID)
		resultCh <- refreshResult{view: view, err: err}
	}()

	waitForProbeStarts(t, deps.prober.started, "node-jp-slow", "node-hk-slow")
	close(firstProbeRelease)
	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("Refresh() error = %v", result.err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Refresh() did not return after first successful probe")
	}

	close(backgroundRelease)
	update := waitForGroupMemberUpdate(t, updates, "node-hk-slow")
	if update.LastLatencyMS == nil || *update.LastLatencyMS != 20 {
		t.Fatalf("background update latency = %v, want 20", update.LastLatencyMS)
	}
	if update.LastStatus != domain.NodeStatusHealthy {
		t.Fatalf("background update status = %q, want healthy", update.LastStatus)
	}
}

func TestTunnelServiceRefreshFailsWhenOnlyCurrentNodeAvailable(t *testing.T) {
	ctx := context.Background()
	service, deps := newTunnelService(t)
	seedTunnelNodes(t, ctx, deps.repos, deps.now, deps.cipher)
	groupID := seedTunnelGroup(t, ctx, deps.groupService, "美国", "^US-")

	created, err := service.Create(ctx, tunnel.CreateInput{
		Name:    "proxy-a",
		GroupID: groupID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.CurrentNodeID == nil || *created.CurrentNodeID != "node-us-mid" {
		t.Fatalf("Create() CurrentNodeID = %v, want node-us-mid", created.CurrentNodeID)
	}

	advanceTunnelClock(deps.now, 6*time.Minute)
	deps.prober.ResetHistory()

	_, err = service.Refresh(ctx, created.ID)
	if !errors.Is(err, tunnel.ErrNoAvailableNodes) {
		t.Fatalf("Refresh() error = %v, want ErrNoAvailableNodes", err)
	}

	got, err := service.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.CurrentNodeID == nil || *got.CurrentNodeID != "node-us-mid" {
		t.Fatalf("CurrentNodeID = %v, want keep node-us-mid", got.CurrentNodeID)
	}
	if got.Status != domain.TunnelStatusDegraded {
		t.Fatalf("Status = %q, want degraded", got.Status)
	}
	if deps.prober.CallCount() != 0 {
		t.Fatalf("prober calls = %d, want 0 when no alternative nodes exist", deps.prober.CallCount())
	}
}

func TestTunnelServiceAllowsSameTunnelNameAcrossGroupsAndRejectsDuplicateInSameGroup(t *testing.T) {
	ctx := context.Background()
	service, deps := newTunnelService(t)
	seedTunnelNodes(t, ctx, deps.repos, deps.now, deps.cipher)
	asiaGroupID := seedTunnelGroup(t, ctx, deps.groupService, "亚洲", "^(HK|JP)-")
	usGroupID := seedTunnelGroup(t, ctx, deps.groupService, "美国", "^US-")

	asiaTunnel, err := service.Create(ctx, tunnel.CreateInput{
		Name:    "shared",
		GroupID: asiaGroupID,
	})
	if err != nil {
		t.Fatalf("Create() asia error = %v", err)
	}
	if got, want := asiaTunnel.RuntimeDir, filepath.Join(deps.runtimeRoot, "亚洲-shared"); got != want {
		t.Fatalf("asia RuntimeDir = %q, want %q", got, want)
	}

	usTunnel, err := service.Create(ctx, tunnel.CreateInput{
		Name:    "shared",
		GroupID: usGroupID,
	})
	if err != nil {
		t.Fatalf("Create() us error = %v", err)
	}
	if got, want := usTunnel.RuntimeDir, filepath.Join(deps.runtimeRoot, "美国-shared"); got != want {
		t.Fatalf("us RuntimeDir = %q, want %q", got, want)
	}

	_, err = service.Create(ctx, tunnel.CreateInput{
		Name:    "shared",
		GroupID: asiaGroupID,
	})
	if !errors.Is(err, tunnel.ErrTunnelConflict) {
		t.Fatalf("Create() duplicate error = %v, want ErrTunnelConflict", err)
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
	if created.CurrentNodeID == nil || *created.CurrentNodeID == "" {
		t.Fatalf("Create() CurrentNodeID = %v, want selected node", created.CurrentNodeID)
	}

	storedBefore, err := deps.repos.Tunnels.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("Tunnels.GetByID() error = %v", err)
	}
	if storedBefore.RuntimeConfigJSON == "" {
		t.Fatal("RuntimeConfigJSON empty before rebuild")
	}

	layout := singbox.NewRuntimeGroupLayout(deps.runtimeRoot, "亚洲", created.Name)
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
	if got.CurrentNodeID == nil || *got.CurrentNodeID != *created.CurrentNodeID {
		t.Fatalf("CurrentNodeID = %v, want keep %s", got.CurrentNodeID, *created.CurrentNodeID)
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
	oldRuntimeDir := created.RuntimeDir

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
	if updated.RuntimeDir == oldRuntimeDir {
		t.Fatalf("RuntimeDir = %q, want renamed runtime dir", updated.RuntimeDir)
	}
	if got, want := updated.RuntimeDir, filepath.Join(deps.runtimeRoot, "美国-proxy-b"); got != want {
		t.Fatalf("RuntimeDir = %q, want %q", got, want)
	}
	if _, err := os.Stat(oldRuntimeDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old runtime dir still exists, err = %v", err)
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

	secondOldRuntimeDir := updated.RuntimeDir
	secondUpdated, err := service.Update(ctx, created.ID, tunnel.UpdateInput{
		Name:    "proxy-c",
		GroupID: asiaGroupID,
	})
	if err != nil {
		t.Fatalf("Update() second error = %v", err)
	}
	if secondUpdated.RuntimeDir == secondOldRuntimeDir {
		t.Fatalf("RuntimeDir = %q, want renamed runtime dir", secondUpdated.RuntimeDir)
	}
	if got, want := secondUpdated.RuntimeDir, filepath.Join(deps.runtimeRoot, "亚洲-proxy-c"); got != want {
		t.Fatalf("RuntimeDir = %q, want %q", got, want)
	}
	if secondUpdated.CurrentNodeID == nil || *secondUpdated.CurrentNodeID == "" {
		t.Fatalf("Update() second CurrentNodeID = %v, want selected node", secondUpdated.CurrentNodeID)
	}
	if _, err := os.Stat(secondOldRuntimeDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("second old runtime dir still exists, err = %v", err)
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

	refreshed, err := service.Refresh(ctx, created.ID)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if refreshed.CurrentNodeID == nil || *refreshed.CurrentNodeID == *secondUpdated.CurrentNodeID {
		t.Fatalf("Refresh() CurrentNodeID = %v, want alternative to %s", refreshed.CurrentNodeID, *secondUpdated.CurrentNodeID)
	}
	if deps.runtime.switchCalls == 0 {
		t.Fatal("switchCalls = 0, want selector switch")
	}
	if state := deps.runtime.state(created.ID); state == nil || state.now == "node-"+*secondUpdated.CurrentNodeID {
		t.Fatalf("runtime selector now = %+v, want switched away from node-%s", state, *secondUpdated.CurrentNodeID)
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
	callIDs []string
	results map[string]node.ProbeResult
	err     error
	started chan string
	blocks  map[string]chan struct{}
}

func (f *fakeTunnelProber) Probe(ctx context.Context, target node.ProbeTarget) (node.ProbeResult, error) {
	f.mu.Lock()
	f.calls++
	f.callIDs = append(f.callIDs, target.ID)
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

func (f *fakeTunnelProber) CallIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.callIDs...)
}

func (f *fakeTunnelProber) ResetHistory() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = 0
	f.callIDs = nil
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

func waitForGroupMemberUpdate(t *testing.T, updates <-chan *group.MemberView, nodeID string) *group.MemberView {
	t.Helper()
	timeout := time.After(500 * time.Millisecond)
	for {
		select {
		case update, ok := <-updates:
			if !ok {
				t.Fatal("group member updates channel closed before expected update")
			}
			if update != nil && update.ID == nodeID {
				return update
			}
		case <-timeout:
			t.Fatalf("did not receive group member update for %s", nodeID)
		}
	}
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
