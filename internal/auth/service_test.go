package auth_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/WAY29/SimplePool/internal/auth"
	"github.com/WAY29/SimplePool/internal/store/sqlite"
)

func TestEnsureAdminCreatesOnlyOnce(t *testing.T) {
	ctx := context.Background()
	service, repos := newTestService(t)

	if err := service.EnsureAdmin(ctx, "admin", "secret-1"); err != nil {
		t.Fatalf("EnsureAdmin() first error = %v", err)
	}

	first, err := repos.AdminUsers.GetByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("GetByUsername() error = %v", err)
	}

	if err := service.EnsureAdmin(ctx, "admin", "secret-2"); err != nil {
		t.Fatalf("EnsureAdmin() second error = %v", err)
	}

	second, err := repos.AdminUsers.GetByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("GetByUsername() second error = %v", err)
	}

	if first.PasswordHash != second.PasswordHash {
		t.Fatal("EnsureAdmin() unexpectedly overwrote existing admin password")
	}
}

func TestLoginAndAuthenticateWithSlidingRenewal(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	if err := service.EnsureAdmin(ctx, "admin", "secret-1"); err != nil {
		t.Fatalf("EnsureAdmin() error = %v", err)
	}

	login, err := service.Login(ctx, auth.LoginInput{
		Username: "admin",
		Password: "secret-1",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	authenticated, err := service.Authenticate(ctx, login.Token)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	if authenticated.User.Username != "admin" {
		t.Fatalf("Username = %q, want admin", authenticated.User.Username)
	}

	if !authenticated.Session.ExpiresAt.After(login.Session.ExpiresAt) {
		t.Fatalf("ExpiresAt = %v, want later than %v", authenticated.Session.ExpiresAt, login.Session.ExpiresAt)
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	if err := service.EnsureAdmin(ctx, "admin", "secret-1"); err != nil {
		t.Fatalf("EnsureAdmin() error = %v", err)
	}

	_, err := service.Login(ctx, auth.LoginInput{
		Username: "admin",
		Password: "wrong-secret",
	})
	if err == nil {
		t.Fatal("Login() error = nil, want error")
	}
}

func TestAuthenticateRejectsExpiredSession(t *testing.T) {
	ctx := context.Background()
	service, repos := newTestService(t)
	if err := service.EnsureAdmin(ctx, "admin", "secret-1"); err != nil {
		t.Fatalf("EnsureAdmin() error = %v", err)
	}

	login, err := service.Login(ctx, auth.LoginInput{
		Username: "admin",
		Password: "secret-1",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	session, err := repos.Sessions.GetByID(ctx, login.Session.ID)
	if err != nil {
		t.Fatalf("Sessions.GetByID() error = %v", err)
	}
	session.ExpiresAt = login.Session.CreatedAt.Add(-time.Minute)
	if err := repos.Sessions.Update(ctx, session); err != nil {
		t.Fatalf("Sessions.Update() error = %v", err)
	}

	if _, err := service.Authenticate(ctx, login.Token); err == nil {
		t.Fatal("Authenticate() error = nil, want error")
	}
}

func TestLogoutDeletesSession(t *testing.T) {
	ctx := context.Background()
	service, repos := newTestService(t)
	if err := service.EnsureAdmin(ctx, "admin", "secret-1"); err != nil {
		t.Fatalf("EnsureAdmin() error = %v", err)
	}

	login, err := service.Login(ctx, auth.LoginInput{
		Username: "admin",
		Password: "secret-1",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	authenticated, err := service.Authenticate(ctx, login.Token)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	if err := service.Logout(ctx, authenticated.Session.ID); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}

	if _, err := repos.Sessions.GetByID(ctx, authenticated.Session.ID); err == nil {
		t.Fatal("GetByID() error = nil, want not found")
	}
}

func newTestService(t *testing.T) (*auth.Service, *sqlite.Repositories) {
	t.Helper()

	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "auth.db"))
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
	base := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)
	calls := 0
	service := auth.NewService(auth.Options{
		AdminUsers: repos.AdminUsers,
		Sessions:   repos.Sessions,
		Now: func() time.Time {
			value := base.Add(time.Duration(calls) * time.Minute)
			calls++
			return value
		},
		SessionTTL: 7 * 24 * time.Hour,
	})

	return service, repos
}
