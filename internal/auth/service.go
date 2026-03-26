package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"time"

	"github.com/WAY29/SimplePool/internal/domain"
	"github.com/WAY29/SimplePool/internal/security"
	"github.com/WAY29/SimplePool/internal/store"
	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	ErrUnauthorized       = errors.New("auth: unauthorized")
)

type Options struct {
	AdminUsers     store.AdminUserRepository
	Sessions       store.SessionRepository
	Now            func() time.Time
	SessionTTL     time.Duration
	TokenGenerator func() (string, error)
	IDGenerator    func() string
}

type Service struct {
	adminUsers     store.AdminUserRepository
	sessions       store.SessionRepository
	now            func() time.Time
	sessionTTL     time.Duration
	tokenGenerator func() (string, error)
	idGenerator    func() string
}

type LoginInput struct {
	Username string
	Password string
}

type LoginResult struct {
	Token   string
	User    *domain.AdminUser
	Session *domain.Session
}

type Authenticated struct {
	User    *domain.AdminUser
	Session *domain.Session
}

func NewService(options Options) *Service {
	now := options.Now
	if now == nil {
		now = time.Now
	}

	tokenGenerator := options.TokenGenerator
	if tokenGenerator == nil {
		tokenGenerator = func() (string, error) {
			return security.GenerateSessionToken(rand.Reader)
		}
	}

	idGenerator := options.IDGenerator
	if idGenerator == nil {
		idGenerator = uuid.NewString
	}

	sessionTTL := options.SessionTTL
	if sessionTTL <= 0 {
		sessionTTL = 7 * 24 * time.Hour
	}

	return &Service{
		adminUsers:     options.AdminUsers,
		sessions:       options.Sessions,
		now:            now,
		sessionTTL:     sessionTTL,
		tokenGenerator: tokenGenerator,
		idGenerator:    idGenerator,
	}
}

func (s *Service) EnsureAdmin(ctx context.Context, username, password string) error {
	admins, err := s.adminUsers.List(ctx)
	if err != nil {
		return err
	}

	if len(admins) > 0 {
		return nil
	}

	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return ErrInvalidCredentials
	}

	passwordHash, err := security.HashPassword(password)
	if err != nil {
		return err
	}

	now := s.now().UTC()
	return s.adminUsers.Create(ctx, &domain.AdminUser{
		ID:           s.idGenerator(),
		Username:     username,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
}

func (s *Service) Login(ctx context.Context, input LoginInput) (*LoginResult, error) {
	user, err := s.adminUsers.GetByUsername(ctx, input.Username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := security.VerifyPassword(user.PasswordHash, input.Password); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := s.tokenGenerator()
	if err != nil {
		return nil, err
	}

	now := s.now().UTC()
	session := &domain.Session{
		ID:         s.idGenerator(),
		UserID:     user.ID,
		TokenHash:  security.HashToken(token),
		ExpiresAt:  now.Add(s.sessionTTL),
		CreatedAt:  now,
		LastSeenAt: now,
	}

	if err := s.sessions.Create(ctx, session); err != nil {
		return nil, err
	}

	return &LoginResult{
		Token:   token,
		User:    user,
		Session: session,
	}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (*Authenticated, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrUnauthorized
	}

	session, err := s.sessions.GetByTokenHash(ctx, security.HashToken(token))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}

	now := s.now().UTC()
	if !session.ExpiresAt.After(now) {
		_ = s.sessions.DeleteByID(ctx, session.ID)
		return nil, ErrUnauthorized
	}

	user, err := s.adminUsers.GetByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}

	session.LastSeenAt = now
	session.ExpiresAt = now.Add(s.sessionTTL)
	if err := s.sessions.Update(ctx, session); err != nil {
		return nil, err
	}

	return &Authenticated{
		User:    user,
		Session: session,
	}, nil
}

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	err := s.sessions.DeleteByID(ctx, sessionID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}

	return err
}
