package subscription_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/WAY29/SimplePool/internal/crypto"
	"github.com/WAY29/SimplePool/internal/domain"
	"github.com/WAY29/SimplePool/internal/node"
	"github.com/WAY29/SimplePool/internal/store/sqlite"
	"github.com/WAY29/SimplePool/internal/subscription"
)

func TestSubscriptionServiceCRUD(t *testing.T) {
	ctx := context.Background()
	service, _ := newSubscriptionService(t, nil, nil)

	created, err := service.Create(ctx, subscription.CreateInput{
		Name: "sub-a",
		URL:  "https://example.com/sub.txt",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := service.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != "sub-a" {
		t.Fatalf("Name = %q, want sub-a", got.Name)
	}

	updated, err := service.Update(ctx, created.ID, subscription.UpdateInput{
		Name:    "sub-b",
		URL:     "https://example.com/sub2.txt",
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != "sub-b" || updated.Enabled {
		t.Fatalf("Update() = %+v, want renamed and disabled", updated)
	}

	list, err := service.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(List()) = %d, want 1", len(list))
	}

	if err := service.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestSubscriptionRefreshCoversOverwriteAndDeleteMissing(t *testing.T) {
	ctx := context.Background()
	fetcher := &fakeFetcher{
		responses: []string{
			"vmess://eyJ2IjoiMiIsInBzIjoiTk9ERS0xIiwiYWRkIjoidm0uZXhhbXBsZS5jb20iLCJwb3J0IjoiNDQzIiwiaWQiOiJ1dWlkLTEiLCJhaWQiOiIwIiwic2N5IjoiYXV0byIsIm5ldCI6InRjcCIsInRscyI6InRscyJ9\n" +
				"trojan://pass1@tr.example.com:443?security=tls#NODE-2",
			"vmess://eyJ2IjoiMiIsInBzIjoiTk9ERS0xLU5FVyIsImFkZCI6InZtMi5leGFtcGxlLmNvbSIsInBvcnQiOiI0NDMiLCJpZCI6InV1aWQtMSIsImFpZCI6IjAiLCJzY3kiOiJhdXRvIiwibmV0IjoidGNwIiwidGxzIjoidGxzIn0=",
		},
	}
	service, repos := newSubscriptionService(t, fetcher, &fakeProber{
		result: node.ProbeResult{
			Success:   true,
			LatencyMS: 42,
			TestURL:   "https://cloudflare.com/cdn-cgi/trace",
		},
	})

	source, err := service.Create(ctx, subscription.CreateInput{
		Name: "sub-a",
		URL:  "https://example.com/sub.txt",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	first, err := service.Refresh(ctx, source.ID, true)
	if err != nil {
		t.Fatalf("Refresh() first error = %v", err)
	}
	if len(first.UpsertedNodes) != 2 || first.DeletedCount != 0 {
		t.Fatalf("Refresh() first = %+v, want 2 upserts and 0 delete", first)
	}

	second, err := service.Refresh(ctx, source.ID, true)
	if err != nil {
		t.Fatalf("Refresh() second error = %v", err)
	}
	if len(second.UpsertedNodes) != 1 || second.DeletedCount != 2 {
		t.Fatalf("Refresh() second = %+v, want 1 upsert and 2 delete", second)
	}

	nodes, err := repos.Nodes.List(ctx)
	if err != nil {
		t.Fatalf("Nodes.List() error = %v", err)
	}
	if len(nodes) != 1 || nodes[0].Name != "NODE-1-NEW" {
		t.Fatalf("Nodes.List() = %+v, want only NODE-1-NEW", nodes)
	}
}

func TestSubscriptionRefreshDoesNotProbeImportedNodes(t *testing.T) {
	ctx := context.Background()
	fetcher := &fakeFetcher{
		responses: []string{
			"trojan://pass1@tr.example.com:443?security=tls#NODE-1",
		},
	}
	prober := &fakeProber{
		result: node.ProbeResult{
			Success:   true,
			LatencyMS: 42,
			TestURL:   "https://cloudflare.com/cdn-cgi/trace",
		},
	}
	service, repos := newSubscriptionService(t, fetcher, prober)

	source, err := service.Create(ctx, subscription.CreateInput{
		Name: "sub-a",
		URL:  "https://example.com/sub.txt",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	result, err := service.Refresh(ctx, source.ID, true)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if len(result.UpsertedNodes) != 1 {
		t.Fatalf("len(UpsertedNodes) = %d, want 1", len(result.UpsertedNodes))
	}
	if prober.calls != 0 {
		t.Fatalf("prober.calls = %d, want 0", prober.calls)
	}

	nodes, err := repos.Nodes.List(ctx)
	if err != nil {
		t.Fatalf("Nodes.List() error = %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("len(nodes) = %d, want 1", len(nodes))
	}
	if nodes[0].LastStatus != domain.NodeStatusUnknown {
		t.Fatalf("LastStatus = %q, want %q", nodes[0].LastStatus, domain.NodeStatusUnknown)
	}
	if nodes[0].LastCheckedAt != nil {
		t.Fatalf("LastCheckedAt = %v, want nil", nodes[0].LastCheckedAt)
	}
	if nodes[0].LastLatencyMS != nil {
		t.Fatalf("LastLatencyMS = %v, want nil", nodes[0].LastLatencyMS)
	}
}

func TestSubscriptionRefreshPreservesExistingProbeState(t *testing.T) {
	ctx := context.Background()
	fetcher := &fakeFetcher{
		responses: []string{
			"trojan://pass1@tr.example.com:443?security=tls#NODE-1",
			"trojan://pass1@tr.example.com:443?security=tls#NODE-1-RENAMED",
		},
	}
	prober := &fakeProber{
		result: node.ProbeResult{
			Success:   true,
			LatencyMS: 42,
			TestURL:   "https://cloudflare.com/cdn-cgi/trace",
		},
	}
	service, repos := newSubscriptionService(t, fetcher, prober)

	source, err := service.Create(ctx, subscription.CreateInput{
		Name: "sub-a",
		URL:  "https://example.com/sub.txt",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := service.Refresh(ctx, source.ID, true); err != nil {
		t.Fatalf("Refresh() first error = %v", err)
	}

	nodes, err := repos.Nodes.List(ctx)
	if err != nil {
		t.Fatalf("Nodes.List() error = %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("len(nodes) = %d, want 1", len(nodes))
	}

	latency := int64(88)
	checkedAt := time.Date(2026, 3, 26, 13, 0, 0, 0, time.UTC)
	nodes[0].LastStatus = domain.NodeStatusHealthy
	nodes[0].LastLatencyMS = &latency
	nodes[0].LastCheckedAt = &checkedAt
	if err := repos.Nodes.Update(ctx, nodes[0]); err != nil {
		t.Fatalf("Nodes.Update() error = %v", err)
	}

	if _, err := service.Refresh(ctx, source.ID, true); err != nil {
		t.Fatalf("Refresh() second error = %v", err)
	}
	if prober.calls != 0 {
		t.Fatalf("prober.calls = %d, want 0", prober.calls)
	}

	nodes, err = repos.Nodes.List(ctx)
	if err != nil {
		t.Fatalf("Nodes.List() second error = %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("len(nodes) after second refresh = %d, want 1", len(nodes))
	}
	if nodes[0].Name != "NODE-1-RENAMED" {
		t.Fatalf("Name = %q, want NODE-1-RENAMED", nodes[0].Name)
	}
	if nodes[0].LastStatus != domain.NodeStatusHealthy {
		t.Fatalf("LastStatus = %q, want %q", nodes[0].LastStatus, domain.NodeStatusHealthy)
	}
	if nodes[0].LastCheckedAt == nil || !nodes[0].LastCheckedAt.Equal(checkedAt) {
		t.Fatalf("LastCheckedAt = %v, want %v", nodes[0].LastCheckedAt, checkedAt)
	}
	if nodes[0].LastLatencyMS == nil || *nodes[0].LastLatencyMS != latency {
		t.Fatalf("LastLatencyMS = %v, want %d", nodes[0].LastLatencyMS, latency)
	}
}

func TestSubscriptionRefreshRecordsError(t *testing.T) {
	ctx := context.Background()
	fetcher := &fakeFetcher{err: subscription.ErrFetchFailed}
	service, _ := newSubscriptionService(t, fetcher, &fakeProber{})

	source, err := service.Create(ctx, subscription.CreateInput{
		Name: "sub-a",
		URL:  "https://example.com/sub.txt",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := service.Refresh(ctx, source.ID, true); err == nil {
		t.Fatal("Refresh() error = nil, want error")
	}

	updated, err := service.Get(ctx, source.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if updated.LastError == "" {
		t.Fatal("LastError empty, want recorded refresh error")
	}
}

func TestSubscriptionCreateRejectsDuplicateFetchFingerprint(t *testing.T) {
	ctx := context.Background()
	service, _ := newSubscriptionService(t, nil, nil)

	_, err := service.Create(ctx, subscription.CreateInput{
		Name: "sub-a",
		URL:  "https://example.com/sub.txt",
	})
	if err != nil {
		t.Fatalf("Create() first error = %v", err)
	}

	if _, err := service.Create(ctx, subscription.CreateInput{
		Name: "sub-b",
		URL:  "https://example.com/sub.txt",
	}); err == nil {
		t.Fatal("Create() duplicate error = nil, want error")
	}
}

func TestHTTPFetcherSetsSubscriptionUserAgent(t *testing.T) {
	expectedUA := "sing-box-windows/1.0 (sing-box; compatible; Windows NT 10.0)"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("User-Agent"); got != expectedUA {
			t.Fatalf("User-Agent = %q, want %q", got, expectedUA)
		}
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("ok"))
	}))
	defer server.Close()

	fetcher := subscription.NewHTTPFetcher(time.Second)
	body, err := fetcher.Fetch(context.Background(), subscription.FetchRequest{URL: server.URL})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("Fetch() body = %q, want ok", string(body))
	}
}

func newSubscriptionService(t *testing.T, fetcher subscription.Fetcher, prober node.Prober) (*subscription.Service, *sqlite.Repositories) {
	t.Helper()

	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "subscription.db"))
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
	cipher, err := crypto.NewAESGCM(make([]byte, 32))
	if err != nil {
		t.Fatalf("NewAESGCM() error = %v", err)
	}
	if fetcher == nil {
		fetcher = &fakeFetcher{}
	}
	if prober == nil {
		prober = &fakeProber{
			result: node.ProbeResult{
				Success:   true,
				LatencyMS: 50,
				TestURL:   "https://cloudflare.com/cdn-cgi/trace",
			},
		}
	}

	svc := subscription.NewService(subscription.Options{
		SubscriptionSources: repos.SubscriptionSources,
		Nodes:               repos.Nodes,
		LatencySamples:      repos.LatencySamples,
		Cipher:              cipher,
		Fetcher:             fetcher,
		Prober:              prober,
		Now: func() time.Time {
			return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
		},
		ProbeCacheTTL: 30 * time.Second,
	})

	return svc, repos
}

type fakeFetcher struct {
	responses []string
	index     int
	err       error
}

func (f *fakeFetcher) Fetch(ctx context.Context, request subscription.FetchRequest) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	if len(f.responses) == 0 {
		return []byte(""), nil
	}
	if f.index >= len(f.responses) {
		return []byte(f.responses[len(f.responses)-1]), nil
	}
	value := f.responses[f.index]
	f.index++
	return []byte(value), nil
}

type fakeProber struct {
	result node.ProbeResult
	calls  int
}

func (f *fakeProber) Probe(ctx context.Context, target node.ProbeTarget) (node.ProbeResult, error) {
	f.calls++
	return f.result, nil
}
