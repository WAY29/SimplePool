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

func TestNewRouterOpenAPIReturns404OutsideDebug(t *testing.T) {
	router := httpapi.NewRouter(httpapi.Options{})

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("GET /openapi.json status = %d, want 404", resp.Code)
	}
}

func TestNewRouterOpenAPIReturnsSpecInDebug(t *testing.T) {
	router := httpapi.NewRouter(httpapi.Options{
		Debug: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /openapi.json status = %d, want 200", resp.Code)
	}

	if contentType := resp.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if _, ok := payload["openapi"]; !ok {
		t.Fatalf("openapi field missing in openapi payload")
	}

	paths, ok := payload["paths"].(map[string]any)
	if !ok || len(paths) == 0 {
		t.Fatalf("paths = %#v, want non-empty object", payload["paths"])
	}

	if _, ok := paths["/api/auth/login"]; !ok {
		t.Fatalf("/api/auth/login missing from spec")
	}
}
