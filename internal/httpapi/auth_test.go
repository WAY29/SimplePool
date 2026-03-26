package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/WAY29/SimplePool/internal/auth"
	"github.com/WAY29/SimplePool/internal/domain"
	"github.com/WAY29/SimplePool/internal/httpapi"
	"github.com/WAY29/SimplePool/internal/security"
	"github.com/WAY29/SimplePool/internal/store/sqlite"
	"github.com/google/uuid"
)

func TestAuthRoutesHappyPath(t *testing.T) {
	router, _ := newAuthRouter(t)

	loginBody := mustJSON(t, map[string]string{
		"username": "admin",
		"password": "secret-1",
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp := httptest.NewRecorder()
	router.ServeHTTP(loginResp, loginReq)

	if loginResp.Code != http.StatusOK {
		t.Fatalf("POST /api/auth/login status = %d, want 200", loginResp.Code)
	}

	var loginPayload struct {
		Token string `json:"token"`
		User  struct {
			Username string `json:"username"`
		} `json:"user"`
	}
	if err := json.Unmarshal(loginResp.Body.Bytes(), &loginPayload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if loginPayload.Token == "" || loginPayload.User.Username != "admin" {
		t.Fatalf("login payload = %+v, want token and admin user", loginPayload)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginPayload.Token)
	meResp := httptest.NewRecorder()
	router.ServeHTTP(meResp, meReq)

	if meResp.Code != http.StatusOK {
		t.Fatalf("GET /api/auth/me status = %d, want 200", meResp.Code)
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+loginPayload.Token)
	logoutResp := httptest.NewRecorder()
	router.ServeHTTP(logoutResp, logoutReq)

	if logoutResp.Code != http.StatusNoContent {
		t.Fatalf("POST /api/auth/logout status = %d, want 204", logoutResp.Code)
	}

	meAfterLogoutReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meAfterLogoutReq.Header.Set("Authorization", "Bearer "+loginPayload.Token)
	meAfterLogoutResp := httptest.NewRecorder()
	router.ServeHTTP(meAfterLogoutResp, meAfterLogoutReq)

	if meAfterLogoutResp.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/auth/me after logout status = %d, want 401", meAfterLogoutResp.Code)
	}
}

func TestAuthRoutesRejectWrongPassword(t *testing.T) {
	router, _ := newAuthRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(mustJSON(t, map[string]string{
		"username": "admin",
		"password": "wrong-secret",
	})))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.Code)
	}
}

func TestAuthRoutesRejectMissingBearerToken(t *testing.T) {
	router, _ := newAuthRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.Code)
	}
}

func TestAuthRoutesRejectExpiredSession(t *testing.T) {
	ctx := context.Background()
	router, repos := newAuthRouter(t)

	user, err := repos.AdminUsers.GetByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("GetByUsername() error = %v", err)
	}

	token := "expired-token"
	expiresAt := time.Date(2026, 3, 25, 9, 0, 0, 0, time.UTC)
	session := &domain.Session{
		ID:         uuid.NewString(),
		UserID:     user.ID,
		TokenHash:  security.HashToken(token),
		ExpiresAt:  expiresAt,
		CreatedAt:  expiresAt.Add(-time.Hour),
		LastSeenAt: expiresAt.Add(-time.Hour),
	}
	if err := repos.Sessions.Create(ctx, session); err != nil {
		t.Fatalf("Sessions.Create() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.Code)
	}
}

func newAuthRouter(t *testing.T) (http.Handler, *sqlite.Repositories) {
	t.Helper()

	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "httpapi.db"))
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
	service := auth.NewService(auth.Options{
		AdminUsers: repos.AdminUsers,
		Sessions:   repos.Sessions,
		Now: func() time.Time {
			return time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)
		},
		SessionTTL: 7 * 24 * time.Hour,
	})

	if err := service.EnsureAdmin(context.Background(), "admin", "secret-1"); err != nil {
		t.Fatalf("EnsureAdmin() error = %v", err)
	}

	router := httpapi.NewRouter(httpapi.Options{
		AuthService: service,
	})
	return router, repos
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	return data
}
