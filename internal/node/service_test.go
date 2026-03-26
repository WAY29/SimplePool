package node_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/WAY29/SimplePool/internal/crypto"
	"github.com/WAY29/SimplePool/internal/domain"
	"github.com/WAY29/SimplePool/internal/node"
	"github.com/WAY29/SimplePool/internal/store/sqlite"
)

func TestNodeServiceCRUDAndToggle(t *testing.T) {
	ctx := context.Background()
	service := newNodeService(t)

	created, err := service.CreateManual(ctx, node.CreateManualInput{
		Name:           "HK-A",
		Protocol:       "vmess",
		Server:         "1.1.1.1",
		ServerPort:     443,
		TransportJSON:  `{"network":"tcp"}`,
		TLSJSON:        `{"enabled":true}`,
		RawPayloadJSON: `{"uuid":"u-1"}`,
		Credential:     []byte(`{"uuid":"u-1"}`),
	})
	if err != nil {
		t.Fatalf("CreateManual() error = %v", err)
	}

	if created.SourceKind != domain.NodeSourceManual {
		t.Fatalf("SourceKind = %q, want manual", created.SourceKind)
	}

	got, err := service.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != "HK-A" {
		t.Fatalf("Name = %q, want HK-A", got.Name)
	}

	updated, err := service.Update(ctx, created.ID, node.UpdateInput{
		Name:           "HK-A-NEW",
		Protocol:       "vmess",
		Server:         "2.2.2.2",
		ServerPort:     8443,
		Enabled:        false,
		TransportJSON:  `{"network":"ws"}`,
		TLSJSON:        `{"enabled":true}`,
		RawPayloadJSON: `{"uuid":"u-2"}`,
		Credential:     []byte(`{"uuid":"u-2"}`),
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if updated.Name != "HK-A-NEW" || updated.Enabled {
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

	if _, err := service.Get(ctx, created.ID); err == nil {
		t.Fatal("Get() error = nil, want not found")
	}
}

func TestNodeServiceImportAndProbeCaching(t *testing.T) {
	ctx := context.Background()
	prober := &fakeProber{
		result: node.ProbeResult{
			Success:   true,
			LatencyMS: 88,
			TestURL:   "https://cloudflare.com/cdn-cgi/trace",
		},
	}
	service := newNodeServiceWithProber(t, prober)

	imported, err := service.Import(ctx, node.ImportInput{
		Payload: "ss://YWVzLTI1Ni1nY206cGFzc0AxMjcuMC4wLjE6ODM4OA==#SS-1\n" +
			"trojan://pass@example.com:443?security=tls#TR-1\n" +
			"vmess://eyJ2IjoiMiIsInBzIjoiVk0tMSIsImFkZCI6InZtLmV4YW1wbGUuY29tIiwicG9ydCI6IjQ0MyIsImlkIjoidXVpZC0xIiwiYWlkIjoiMCIsInNjeSI6ImF1dG8iLCJuZXQiOiJ0Y3AiLCJ0bHMiOiJ0bHMifQ==\n" +
			"vless://uuid-2@vless.example.com:443?security=tls&type=tcp#VL-1\n" +
			"hysteria2://pass@hy2.example.com:443?sni=hy2.example.com#HY2-1",
	})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if len(imported) != 5 {
		t.Fatalf("len(imported) = %d, want 5", len(imported))
	}

	first, err := service.ProbeByID(ctx, imported[0].ID, false)
	if err != nil {
		t.Fatalf("ProbeByID() error = %v", err)
	}
	if !first.Success || first.LatencyMS != 88 {
		t.Fatalf("ProbeByID() = %+v, want success latency 88", first)
	}

	second, err := service.ProbeByID(ctx, imported[0].ID, false)
	if err != nil {
		t.Fatalf("ProbeByID() second error = %v", err)
	}
	if second.Cached != true {
		t.Fatalf("Cached = %v, want true", second.Cached)
	}
	if prober.calls != 1 {
		t.Fatalf("prober.calls = %d, want 1", prober.calls)
	}

	_, err = service.ProbeByID(ctx, imported[0].ID, true)
	if err != nil {
		t.Fatalf("ProbeByID(force) error = %v", err)
	}
	if prober.calls != 2 {
		t.Fatalf("prober.calls = %d, want 2 after force", prober.calls)
	}
}

func TestNodeServiceBatchProbeUpdatesStatus(t *testing.T) {
	ctx := context.Background()
	prober := &fakeProber{
		resultsByNode: map[string]node.ProbeResult{
			"good": {Success: true, LatencyMS: 30, TestURL: "https://cloudflare.com/cdn-cgi/trace"},
			"bad":  {Success: false, ErrorMessage: "timeout", TestURL: "https://cloudflare.com/cdn-cgi/trace"},
		},
	}
	service := newNodeServiceWithProber(t, prober)

	good, err := service.CreateManual(ctx, node.CreateManualInput{
		Name: "good", Protocol: "vmess", Server: "1.1.1.1", ServerPort: 443,
		TransportJSON: `{}`, TLSJSON: `{}`, RawPayloadJSON: `{}`, Credential: []byte(`{"uuid":"g"}`),
	})
	if err != nil {
		t.Fatalf("CreateManual() good error = %v", err)
	}
	bad, err := service.CreateManual(ctx, node.CreateManualInput{
		Name: "bad", Protocol: "trojan", Server: "2.2.2.2", ServerPort: 443,
		TransportJSON: `{}`, TLSJSON: `{}`, RawPayloadJSON: `{}`, Credential: []byte(`{"password":"b"}`),
	})
	if err != nil {
		t.Fatalf("CreateManual() bad error = %v", err)
	}

	results, err := service.ProbeBatch(ctx, []string{good.ID, bad.ID}, true)
	if err != nil {
		t.Fatalf("ProbeBatch() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	refreshedGood, err := service.Get(ctx, good.ID)
	if err != nil {
		t.Fatalf("Get(good) error = %v", err)
	}
	refreshedBad, err := service.Get(ctx, bad.ID)
	if err != nil {
		t.Fatalf("Get(bad) error = %v", err)
	}

	if refreshedGood.LastStatus != domain.NodeStatusHealthy {
		t.Fatalf("good LastStatus = %q, want healthy", refreshedGood.LastStatus)
	}
	if refreshedBad.LastStatus != domain.NodeStatusUnreachable {
		t.Fatalf("bad LastStatus = %q, want unreachable", refreshedBad.LastStatus)
	}
}

func newNodeService(t *testing.T) *node.Service {
	t.Helper()
	return newNodeServiceWithProber(t, &fakeProber{
		result: node.ProbeResult{
			Success:   true,
			LatencyMS: 55,
			TestURL:   "https://cloudflare.com/cdn-cgi/trace",
		},
	})
}

func newNodeServiceWithProber(t *testing.T, prober node.Prober) *node.Service {
	t.Helper()

	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "node.db"))
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

	return node.NewService(node.Options{
		Nodes:          repos.Nodes,
		LatencySamples: repos.LatencySamples,
		Cipher:         cipher,
		Prober:         prober,
		Now: func() time.Time {
			return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
		},
		ProbeCacheTTL: 30 * time.Second,
	})
}

type fakeProber struct {
	calls         int
	result        node.ProbeResult
	resultsByNode map[string]node.ProbeResult
}

func (f *fakeProber) Probe(ctx context.Context, target node.ProbeTarget) (node.ProbeResult, error) {
	f.calls++
	if f.resultsByNode != nil {
		if result, ok := f.resultsByNode[target.Name]; ok {
			return result, nil
		}
	}
	return f.result, nil
}
