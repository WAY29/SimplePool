package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/WAY29/SimplePool/internal/domain"
	"github.com/WAY29/SimplePool/internal/store"
	"github.com/WAY29/SimplePool/internal/store/sqlite"
)

func TestAdminUserRepositoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	repos := newTestRepos(t)

	now := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)
	user := &domain.AdminUser{
		ID:           "admin-1",
		Username:     "admin",
		PasswordHash: "hash-a",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := repos.AdminUsers.Create(ctx, user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repos.AdminUsers.GetByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("GetByUsername() error = %v", err)
	}

	if got.PasswordHash != "hash-a" {
		t.Fatalf("PasswordHash = %q, want hash-a", got.PasswordHash)
	}

	user.PasswordHash = "hash-b"
	user.UpdatedAt = now.Add(time.Hour)
	if err := repos.AdminUsers.Update(ctx, user); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	updated, err := repos.AdminUsers.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if updated.PasswordHash != "hash-b" {
		t.Fatalf("updated PasswordHash = %q, want hash-b", updated.PasswordHash)
	}
}

func TestSessionRepositoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	repos := newTestRepos(t)
	user := seedAdminUser(t, ctx, repos)

	now := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)
	session := &domain.Session{
		ID:         "session-1",
		UserID:     user.ID,
		TokenHash:  "token-hash",
		ExpiresAt:  now.Add(24 * time.Hour),
		CreatedAt:  now,
		LastSeenAt: now,
	}

	if err := repos.Sessions.Create(ctx, session); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repos.Sessions.GetByTokenHash(ctx, "token-hash")
	if err != nil {
		t.Fatalf("GetByTokenHash() error = %v", err)
	}

	if got.UserID != user.ID {
		t.Fatalf("UserID = %q, want %q", got.UserID, user.ID)
	}

	session.LastSeenAt = now.Add(2 * time.Hour)
	session.ExpiresAt = now.Add(7 * 24 * time.Hour)
	if err := repos.Sessions.Update(ctx, session); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	list, err := repos.Sessions.ListByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}

	if len(list) != 1 || !list[0].ExpiresAt.Equal(session.ExpiresAt) {
		t.Fatalf("ListByUserID() = %+v, want updated session", list)
	}

	deleted, err := repos.Sessions.DeleteExpired(ctx, now.Add(10*24*time.Hour))
	if err != nil {
		t.Fatalf("DeleteExpired() error = %v", err)
	}

	if deleted != 1 {
		t.Fatalf("DeleteExpired() = %d, want 1", deleted)
	}

	if _, err := repos.Sessions.GetByID(ctx, session.ID); err == nil {
		t.Fatal("GetByID() error = nil, want not found")
	}
}

func TestSubscriptionSourceRepositoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	repos := newTestRepos(t)
	now := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)

	source := &domain.SubscriptionSource{
		ID:               "sub-1",
		Name:             "主订阅",
		FetchFingerprint: "fp-1",
		URLCiphertext:    []byte("cipher"),
		URLNonce:         []byte("nonce"),
		Enabled:          true,
		LastError:        "",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := repos.SubscriptionSources.Create(ctx, source); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repos.SubscriptionSources.GetByFetchFingerprint(ctx, "fp-1")
	if err != nil {
		t.Fatalf("GetByFetchFingerprint() error = %v", err)
	}

	if got.Name != source.Name {
		t.Fatalf("Name = %q, want %q", got.Name, source.Name)
	}

	lastRefresh := now.Add(time.Hour)
	source.Enabled = false
	source.LastError = "boom"
	source.LastRefreshAt = &lastRefresh
	source.UpdatedAt = now.Add(2 * time.Hour)
	if err := repos.SubscriptionSources.Update(ctx, source); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	list, err := repos.SubscriptionSources.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 1 || list[0].LastError != "boom" {
		t.Fatalf("List() = %+v, want updated source", list)
	}

	if err := repos.SubscriptionSources.DeleteByID(ctx, source.ID); err != nil {
		t.Fatalf("DeleteByID() error = %v", err)
	}
}

func TestNodeRepositoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	repos := newTestRepos(t)
	now := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)

	source := &domain.SubscriptionSource{
		ID:               "sub-1",
		Name:             "主订阅",
		FetchFingerprint: "fp-1",
		URLCiphertext:    []byte("cipher"),
		URLNonce:         []byte("nonce"),
		Enabled:          true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := repos.SubscriptionSources.Create(ctx, source); err != nil {
		t.Fatalf("Create source error = %v", err)
	}

	lastChecked := now.Add(30 * time.Minute)
	lastLatency := int64(123)
	node := &domain.Node{
		ID:                   "node-1",
		Name:                 "节点 A",
		SourceNodeKey:        "source-key-1",
		DedupeFingerprint:    "dup-1",
		SourceKind:           domain.NodeSourceSubscription,
		SubscriptionSourceID: &source.ID,
		Protocol:             "vmess",
		Server:               "1.2.3.4",
		ServerPort:           443,
		CredentialCiphertext: []byte("cipher"),
		CredentialNonce:      []byte("nonce"),
		TransportJSON:        `{"network":"tcp"}`,
		TLSJSON:              `{"enabled":true}`,
		RawPayloadJSON:       `{"id":"abc"}`,
		Enabled:              true,
		LastLatencyMS:        &lastLatency,
		LastStatus:           domain.NodeStatusHealthy,
		LastCheckedAt:        &lastChecked,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	if err := repos.Nodes.Create(ctx, node); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repos.Nodes.GetBySourceNodeKey(ctx, source.ID, node.SourceNodeKey)
	if err != nil {
		t.Fatalf("GetBySourceNodeKey() error = %v", err)
	}

	if got.Server != "1.2.3.4" {
		t.Fatalf("Server = %q, want 1.2.3.4", got.Server)
	}

	node.LastStatus = domain.NodeStatusUnreachable
	node.Enabled = false
	node.UpdatedAt = now.Add(time.Hour)
	if err := repos.Nodes.Update(ctx, node); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	list, err := repos.Nodes.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 1 || list[0].LastStatus != domain.NodeStatusUnreachable {
		t.Fatalf("List() = %+v, want updated node", list)
	}

	if err := repos.Nodes.DeleteByID(ctx, node.ID); err != nil {
		t.Fatalf("DeleteByID() error = %v", err)
	}
}

func TestGroupAndTunnelRepositoriesRoundTrip(t *testing.T) {
	ctx := context.Background()
	repos := newTestRepos(t)
	now := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)

	group := &domain.Group{
		ID:          "group-1",
		Name:        "香港",
		FilterRegex: "HK",
		Description: "香港节点",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := repos.Groups.Create(ctx, group); err != nil {
		t.Fatalf("Groups.Create() error = %v", err)
	}

	group.FilterRegex = "HK|JP"
	group.UpdatedAt = now.Add(time.Hour)
	if err := repos.Groups.Update(ctx, group); err != nil {
		t.Fatalf("Groups.Update() error = %v", err)
	}

	gotGroup, err := repos.Groups.GetByID(ctx, group.ID)
	if err != nil {
		t.Fatalf("Groups.GetByID() error = %v", err)
	}
	if gotGroup.FilterRegex != "HK|JP" {
		t.Fatalf("FilterRegex = %q, want HK|JP", gotGroup.FilterRegex)
	}

	tunnel := &domain.Tunnel{
		ID:                         "tunnel-1",
		Name:                       "出口 A",
		GroupID:                    group.ID,
		ListenHost:                 "127.0.0.1",
		ListenPort:                 18080,
		Status:                     domain.TunnelStatusRunning,
		ControllerPort:             19090,
		ControllerSecretCiphertext: []byte("cipher"),
		ControllerSecretNonce:      []byte("nonce"),
		RuntimeDir:                 filepath.Join(t.TempDir(), "runtime"),
		LastRefreshError:           "",
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}
	if err := repos.Tunnels.Create(ctx, tunnel); err != nil {
		t.Fatalf("Tunnels.Create() error = %v", err)
	}

	tunnel.Status = domain.TunnelStatusDegraded
	tunnel.LastRefreshError = "probe failed"
	tunnel.UpdatedAt = now.Add(2 * time.Hour)
	if err := repos.Tunnels.Update(ctx, tunnel); err != nil {
		t.Fatalf("Tunnels.Update() error = %v", err)
	}

	gotTunnel, err := repos.Tunnels.GetByID(ctx, tunnel.ID)
	if err != nil {
		t.Fatalf("Tunnels.GetByID() error = %v", err)
	}
	if gotTunnel.Status != domain.TunnelStatusDegraded {
		t.Fatalf("Status = %q, want degraded", gotTunnel.Status)
	}
}

func TestTunnelEventAndLatencySampleRepositoriesRoundTrip(t *testing.T) {
	ctx := context.Background()
	repos := newTestRepos(t)
	now := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)

	group := &domain.Group{
		ID:          "group-1",
		Name:        "香港",
		FilterRegex: "HK",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := repos.Groups.Create(ctx, group); err != nil {
		t.Fatalf("Groups.Create() error = %v", err)
	}

	tunnel := &domain.Tunnel{
		ID:                         "tunnel-1",
		Name:                       "出口 A",
		GroupID:                    group.ID,
		ListenHost:                 "127.0.0.1",
		ListenPort:                 18080,
		Status:                     domain.TunnelStatusRunning,
		ControllerPort:             19090,
		ControllerSecretCiphertext: []byte("cipher"),
		ControllerSecretNonce:      []byte("nonce"),
		RuntimeDir:                 "runtime/tunnel-1",
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}
	if err := repos.Tunnels.Create(ctx, tunnel); err != nil {
		t.Fatalf("Tunnels.Create() error = %v", err)
	}

	node := &domain.Node{
		ID:             "node-1",
		Name:           "节点 A",
		SourceKind:     domain.NodeSourceManual,
		Protocol:       "vmess",
		Server:         "1.2.3.4",
		ServerPort:     443,
		TransportJSON:  "{}",
		TLSJSON:        "{}",
		RawPayloadJSON: "{}",
		Enabled:        true,
		LastStatus:     domain.NodeStatusUnknown,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := repos.Nodes.Create(ctx, node); err != nil {
		t.Fatalf("Nodes.Create() error = %v", err)
	}

	eventA := &domain.TunnelEvent{
		ID:         "event-1",
		TunnelID:   tunnel.ID,
		EventType:  "created",
		DetailJSON: `{"status":"running"}`,
		CreatedAt:  now,
	}
	eventB := &domain.TunnelEvent{
		ID:         "event-2",
		TunnelID:   tunnel.ID,
		EventType:  "refreshed",
		DetailJSON: `{"status":"running"}`,
		CreatedAt:  now.Add(time.Minute),
	}

	if err := repos.TunnelEvents.Create(ctx, eventA); err != nil {
		t.Fatalf("TunnelEvents.Create() error = %v", err)
	}
	if err := repos.TunnelEvents.Create(ctx, eventB); err != nil {
		t.Fatalf("TunnelEvents.Create() error = %v", err)
	}

	events, err := repos.TunnelEvents.ListByTunnelID(ctx, tunnel.ID, 10)
	if err != nil {
		t.Fatalf("TunnelEvents.ListByTunnelID() error = %v", err)
	}

	if len(events) != 2 || events[0].ID != "event-2" {
		t.Fatalf("events = %+v, want newest first", events)
	}

	latency := int64(87)
	sample := &domain.LatencySample{
		ID:           "sample-1",
		NodeID:       node.ID,
		TunnelID:     &tunnel.ID,
		TestURL:      "https://cp.cloudflare.com/generate_204",
		LatencyMS:    &latency,
		Success:      true,
		ErrorMessage: "",
		CreatedAt:    now,
	}
	if err := repos.LatencySamples.Create(ctx, sample); err != nil {
		t.Fatalf("LatencySamples.Create() error = %v", err)
	}

	samples, err := repos.LatencySamples.ListByNodeID(ctx, node.ID, 10)
	if err != nil {
		t.Fatalf("LatencySamples.ListByNodeID() error = %v", err)
	}

	if len(samples) != 1 || *samples[0].LatencyMS != latency {
		t.Fatalf("samples = %+v, want inserted sample", samples)
	}
}

func newTestRepos(t *testing.T) *sqlite.Repositories {
	t.Helper()

	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := sqlite.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	return sqlite.NewRepositories(db)
}

func seedAdminUser(t *testing.T, ctx context.Context, repos *sqlite.Repositories) *domain.AdminUser {
	t.Helper()

	now := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)
	user := &domain.AdminUser{
		ID:           "admin-1",
		Username:     "admin",
		PasswordHash: "hash-a",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := repos.AdminUsers.Create(ctx, user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	return user
}

var _ store.AdminUserRepository = (*sqlite.AdminUserRepository)(nil)
