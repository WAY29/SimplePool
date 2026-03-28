package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	appcrypto "github.com/WAY29/SimplePool/internal/crypto"
	"github.com/WAY29/SimplePool/internal/group"
	"github.com/WAY29/SimplePool/internal/httpapi"
	"github.com/WAY29/SimplePool/internal/node"
	"github.com/WAY29/SimplePool/internal/settings"
	"github.com/WAY29/SimplePool/internal/store/sqlite"
	"github.com/WAY29/SimplePool/internal/subscription"
)

func TestNodeAndSubscriptionRoutes(t *testing.T) {
	router, token := newProtectedRouter(t)

	createNodeResp := performJSON(t, router, http.MethodPost, "/api/nodes", token, map[string]any{
		"name":             "HK-A",
		"protocol":         "vmess",
		"server":           "1.1.1.1",
		"server_port":      443,
		"transport_json":   `{"network":"tcp"}`,
		"tls_json":         `{"enabled":true}`,
		"raw_payload_json": `{"uuid":"u-1"}`,
		"credential":       `{"uuid":"u-1"}`,
	})
	if createNodeResp.Code != http.StatusCreated {
		t.Fatalf("POST /api/nodes status = %d, want 201", createNodeResp.Code)
	}

	var createdNode map[string]any
	_ = json.Unmarshal(createNodeResp.Body.Bytes(), &createdNode)
	nodeID := createdNode["id"].(string)

	listNodesResp := perform(t, router, http.MethodGet, "/api/nodes", token, nil)
	if listNodesResp.Code != http.StatusOK {
		t.Fatalf("GET /api/nodes status = %d, want 200", listNodesResp.Code)
	}

	getNodeResp := perform(t, router, http.MethodGet, "/api/nodes/"+nodeID, token, nil)
	if getNodeResp.Code != http.StatusOK {
		t.Fatalf("GET /api/nodes/:id status = %d, want 200", getNodeResp.Code)
	}

	updateNodeResp := performJSON(t, router, http.MethodPut, "/api/nodes/"+nodeID, token, map[string]any{
		"name":             "HK-B",
		"protocol":         "vmess",
		"server":           "2.2.2.2",
		"server_port":      8443,
		"enabled":          false,
		"transport_json":   `{"network":"ws"}`,
		"tls_json":         `{"enabled":true}`,
		"raw_payload_json": `{"uuid":"u-2"}`,
		"credential":       `{"uuid":"u-2"}`,
	})
	if updateNodeResp.Code != http.StatusOK {
		t.Fatalf("PUT /api/nodes/:id status = %d, want 200", updateNodeResp.Code)
	}

	setEnabledResp := performJSON(t, router, http.MethodPut, "/api/nodes/"+nodeID+"/enabled", token, map[string]any{
		"enabled": true,
	})
	if setEnabledResp.Code != http.StatusOK {
		t.Fatalf("PUT /api/nodes/:id/enabled status = %d, want 200", setEnabledResp.Code)
	}

	importResp := performJSON(t, router, http.MethodPost, "/api/nodes/import", token, map[string]any{
		"payload": "trojan://pass@example.com:443?security=tls#TR-1",
	})
	if importResp.Code != http.StatusCreated {
		t.Fatalf("POST /api/nodes/import status = %d, want 201", importResp.Code)
	}

	probeNodeResp := performJSON(t, router, http.MethodPost, "/api/nodes/"+nodeID+"/probe", token, map[string]any{
		"force": true,
	})
	if probeNodeResp.Code != http.StatusOK {
		t.Fatalf("POST /api/nodes/:id/probe status = %d, want 200", probeNodeResp.Code)
	}

	probeBatchResp := performJSON(t, router, http.MethodPost, "/api/nodes/probe", token, map[string]any{
		"ids":   []string{nodeID},
		"force": true,
	})
	if probeBatchResp.Code != http.StatusOK {
		t.Fatalf("POST /api/nodes/probe status = %d, want 200", probeBatchResp.Code)
	}

	createSubResp := performJSON(t, router, http.MethodPost, "/api/subscriptions", token, map[string]any{
		"name": "sub-a",
		"url":  "https://example.com/sub.txt",
	})
	if createSubResp.Code != http.StatusCreated {
		t.Fatalf("POST /api/subscriptions status = %d, want 201", createSubResp.Code)
	}

	var createdSub map[string]any
	_ = json.Unmarshal(createSubResp.Body.Bytes(), &createdSub)
	subID := createdSub["id"].(string)

	listSubResp := perform(t, router, http.MethodGet, "/api/subscriptions", token, nil)
	if listSubResp.Code != http.StatusOK {
		t.Fatalf("GET /api/subscriptions status = %d, want 200", listSubResp.Code)
	}

	updateSubResp := performJSON(t, router, http.MethodPut, "/api/subscriptions/"+subID, token, map[string]any{
		"name":    "sub-b",
		"url":     "https://example.com/sub2.txt",
		"enabled": false,
	})
	if updateSubResp.Code != http.StatusOK {
		t.Fatalf("PUT /api/subscriptions/:id status = %d, want 200", updateSubResp.Code)
	}

	refreshSubResp := performJSON(t, router, http.MethodPost, "/api/subscriptions/"+subID+"/refresh", token, map[string]any{
		"force": true,
	})
	if refreshSubResp.Code != http.StatusOK {
		t.Fatalf("POST /api/subscriptions/:id/refresh status = %d, want 200", refreshSubResp.Code)
	}

	deleteNodeResp := perform(t, router, http.MethodDelete, "/api/nodes/"+nodeID, token, nil)
	if deleteNodeResp.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/nodes/:id status = %d, want 204", deleteNodeResp.Code)
	}

	deleteSubResp := perform(t, router, http.MethodDelete, "/api/subscriptions/"+subID, token, nil)
	if deleteSubResp.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/subscriptions/:id status = %d, want 204", deleteSubResp.Code)
	}
}

func TestCreateSubscriptionAutoRefreshesNodes(t *testing.T) {
	router, token := newProtectedRouter(t)

	createSubResp := performJSON(t, router, http.MethodPost, "/api/subscriptions", token, map[string]any{
		"name": "sub-a",
		"url":  "https://example.com/sub.txt",
	})
	if createSubResp.Code != http.StatusCreated {
		t.Fatalf("POST /api/subscriptions status = %d, want 201", createSubResp.Code)
	}

	var created subscription.View
	if err := json.Unmarshal(createSubResp.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(create) error = %v", err)
	}
	if created.LastRefreshAt == nil {
		t.Fatal("LastRefreshAt = nil, want initial auto refresh timestamp")
	}
	if created.LastError != "" {
		t.Fatalf("LastError = %q, want empty", created.LastError)
	}

	listNodesResp := perform(t, router, http.MethodGet, "/api/nodes", token, nil)
	if listNodesResp.Code != http.StatusOK {
		t.Fatalf("GET /api/nodes status = %d, want 200", listNodesResp.Code)
	}

	var nodes []node.View
	if err := json.Unmarshal(listNodesResp.Body.Bytes(), &nodes); err != nil {
		t.Fatalf("json.Unmarshal(nodes) error = %v", err)
	}
	var subscriptionNodes []node.View
	for _, item := range nodes {
		if item.SubscriptionSourceID != nil && *item.SubscriptionSourceID == created.ID {
			subscriptionNodes = append(subscriptionNodes, item)
		}
	}
	if len(subscriptionNodes) != 1 {
		t.Fatalf("len(subscriptionNodes) = %d, want 1", len(subscriptionNodes))
	}
	if subscriptionNodes[0].Name != "TR-1" {
		t.Fatalf("Name = %q, want TR-1", subscriptionNodes[0].Name)
	}
}

func TestCreateSubscriptionRefreshFailureKeepsSubscription(t *testing.T) {
	router, token := newProtectedRouterWithFetcher(t, &httpFakeFetcher{err: errors.New("dial tcp timeout")})

	createSubResp := performJSON(t, router, http.MethodPost, "/api/subscriptions", token, map[string]any{
		"name": "sub-a",
		"url":  "https://example.com/sub.txt",
	})
	if createSubResp.Code != http.StatusCreated {
		t.Fatalf("POST /api/subscriptions status = %d, want 201", createSubResp.Code)
	}

	var created subscription.View
	if err := json.Unmarshal(createSubResp.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(create) error = %v", err)
	}
	if created.LastRefreshAt != nil {
		t.Fatalf("LastRefreshAt = %v, want nil", created.LastRefreshAt)
	}
	if created.LastError == "" {
		t.Fatal("LastError empty, want initial refresh error")
	}

	listSubResp := perform(t, router, http.MethodGet, "/api/subscriptions", token, nil)
	if listSubResp.Code != http.StatusOK {
		t.Fatalf("GET /api/subscriptions status = %d, want 200", listSubResp.Code)
	}

	var subscriptions []subscription.View
	if err := json.Unmarshal(listSubResp.Body.Bytes(), &subscriptions); err != nil {
		t.Fatalf("json.Unmarshal(subscriptions) error = %v", err)
	}
	if len(subscriptions) != 1 {
		t.Fatalf("len(subscriptions) = %d, want 1", len(subscriptions))
	}
	if subscriptions[0].ID != created.ID {
		t.Fatalf("subscriptions[0].ID = %q, want %q", subscriptions[0].ID, created.ID)
	}
	if subscriptions[0].LastError == "" {
		t.Fatal("subscriptions[0].LastError empty, want persisted initial refresh error")
	}

	listNodesResp := perform(t, router, http.MethodGet, "/api/nodes", token, nil)
	if listNodesResp.Code != http.StatusOK {
		t.Fatalf("GET /api/nodes status = %d, want 200", listNodesResp.Code)
	}

	var nodes []node.View
	if err := json.Unmarshal(listNodesResp.Body.Bytes(), &nodes); err != nil {
		t.Fatalf("json.Unmarshal(nodes) error = %v", err)
	}
	for _, item := range nodes {
		if item.SubscriptionSourceID != nil && *item.SubscriptionSourceID == created.ID {
			t.Fatalf("found subscription node %q for failed initial refresh, want none", item.ID)
		}
	}
}

func TestSettingsRoutes(t *testing.T) {
	router, token := newProtectedRouter(t)

	getResp := perform(t, router, http.MethodGet, "/api/settings/probe", token, nil)
	if getResp.Code != http.StatusOK {
		t.Fatalf("GET /api/settings/probe status = %d, want 200", getResp.Code)
	}

	var initial map[string]any
	if err := json.Unmarshal(getResp.Body.Bytes(), &initial); err != nil {
		t.Fatalf("json.Unmarshal(initial) error = %v", err)
	}
	if got := initial["test_url"]; got != settings.DefaultProbeTestURL {
		t.Fatalf("initial test_url = %v, want %q", got, settings.DefaultProbeTestURL)
	}

	updateResp := performJSON(t, router, http.MethodPut, "/api/settings/probe", token, map[string]any{
		"test_url": "https://www.gstatic.com/generate_204",
	})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("PUT /api/settings/probe status = %d, want 200", updateResp.Code)
	}

	var updated map[string]any
	if err := json.Unmarshal(updateResp.Body.Bytes(), &updated); err != nil {
		t.Fatalf("json.Unmarshal(updated) error = %v", err)
	}
	if got := updated["test_url"]; got != "https://www.gstatic.com/generate_204" {
		t.Fatalf("updated test_url = %v, want gstatic", got)
	}

	invalidResp := performJSON(t, router, http.MethodPut, "/api/settings/probe", token, map[string]any{
		"test_url": "ftp://example.com",
	})
	if invalidResp.Code != http.StatusBadRequest {
		t.Fatalf("PUT /api/settings/probe invalid status = %d, want 400", invalidResp.Code)
	}
}

func newProtectedRouter(t *testing.T) (http.Handler, string) {
	t.Helper()

	return newProtectedRouterWithFetcher(t, &httpFakeFetcher{
		payload: "trojan://pass@example.com:443?security=tls#TR-1",
	})
}

func newProtectedRouterWithFetcher(t *testing.T, fetcher subscription.Fetcher) (http.Handler, string) {
	t.Helper()

	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "node-sub-http.db"))
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
	now := func() time.Time {
		return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	}
	authService, token, err := buildAuthServiceForHTTPTests(repos, now)
	if err != nil {
		t.Fatalf("buildAuthServiceForHTTPTests() error = %v", err)
	}

	cipher, err := appcrypto.NewAESGCM(make([]byte, 32))
	if err != nil {
		t.Fatalf("NewAESGCM() error = %v", err)
	}

	prober := &httpFakeProber{}
	nodeService := node.NewService(node.Options{
		Nodes:          repos.Nodes,
		LatencySamples: repos.LatencySamples,
		Cipher:         cipher,
		Prober:         prober,
		Now:            now,
		ProbeCacheTTL:  30 * time.Second,
	})
	subscriptionService := subscription.NewService(subscription.Options{
		SubscriptionSources: repos.SubscriptionSources,
		Nodes:               repos.Nodes,
		LatencySamples:      repos.LatencySamples,
		Cipher:              cipher,
		Fetcher:             fetcher,
		Prober:        prober,
		Now:           now,
		ProbeCacheTTL: 30 * time.Second,
	})
	groupService := group.NewService(group.Options{
		Groups: repos.Groups,
		Nodes:  repos.Nodes,
		Now:    now,
	})
	settingsService := settings.NewService(settings.Options{
		AppSettings: repos.AppSettings,
		Now:         now,
	})
	tunnelService := buildTunnelServiceForHTTPTests(repos, cipher, now, t.TempDir())
	if err := seedTunnelFixturesForHTTPTests(repos, cipher); err != nil {
		t.Fatalf("seedTunnelFixturesForHTTPTests() error = %v", err)
	}

	router := httpapi.NewRouter(httpapi.Options{
		AuthService:         authService,
		GroupService:        groupService,
		NodeService:         nodeService,
		SettingsService:     settingsService,
		SubscriptionService: subscriptionService,
		TunnelService:       tunnelService,
	})

	return router, token
}

func performJSON(t *testing.T, handler http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return perform(t, handler, method, path, token, data)
}

func perform(t *testing.T, handler http.Handler, method, path, token string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

type httpFakeProber struct{}

func (p *httpFakeProber) Probe(ctx context.Context, target node.ProbeTarget) (node.ProbeResult, error) {
	return node.ProbeResult{
		Success:   true,
		LatencyMS: 66,
		TestURL:   "https://cloudflare.com/cdn-cgi/trace",
	}, nil
}

type httpFakeFetcher struct {
	payload string
	err     error
}

func (f *httpFakeFetcher) Fetch(ctx context.Context, request subscription.FetchRequest) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []byte(f.payload), nil
}
