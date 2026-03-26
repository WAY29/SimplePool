package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WAY29/SimplePool/internal/httpapi"
)

func TestNewRouterServesEmbeddedIndex(t *testing.T) {
	router := httpapi.NewRouter(httpapi.Options{})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", resp.Code)
	}

	if contentType := resp.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", contentType)
	}

	if !strings.Contains(resp.Body.String(), "<!doctype html>") && !strings.Contains(strings.ToLower(resp.Body.String()), "<!doctype html>") {
		t.Fatalf("body does not look like index.html")
	}
}

func TestNewRouterReadyzNoLongerReturnsFrontendPort(t *testing.T) {
	router := httpapi.NewRouter(httpapi.Options{})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /readyz status = %d, want 200", resp.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got, want := payload["status"], "ready"; got != want {
		t.Fatalf("status = %#v, want %q", got, want)
	}

	if _, ok := payload["frontend_port"]; ok {
		t.Fatalf("frontend_port should not exist")
	}
}
