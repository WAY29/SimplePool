package store

import (
	"context"
	"errors"
	"time"

	"github.com/WAY29/SimplePool/internal/domain"
)

var ErrNotFound = errors.New("store: not found")

type AdminUserRepository interface {
	Create(ctx context.Context, user *domain.AdminUser) error
	Update(ctx context.Context, user *domain.AdminUser) error
	GetByID(ctx context.Context, id string) (*domain.AdminUser, error)
	GetByUsername(ctx context.Context, username string) (*domain.AdminUser, error)
	List(ctx context.Context) ([]*domain.AdminUser, error)
}

type SessionRepository interface {
	Create(ctx context.Context, session *domain.Session) error
	Update(ctx context.Context, session *domain.Session) error
	GetByID(ctx context.Context, id string) (*domain.Session, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*domain.Session, error)
	ListByUserID(ctx context.Context, userID string) ([]*domain.Session, error)
	DeleteByID(ctx context.Context, id string) error
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}

type SubscriptionSourceRepository interface {
	Create(ctx context.Context, source *domain.SubscriptionSource) error
	Update(ctx context.Context, source *domain.SubscriptionSource) error
	GetByID(ctx context.Context, id string) (*domain.SubscriptionSource, error)
	GetByFetchFingerprint(ctx context.Context, fingerprint string) (*domain.SubscriptionSource, error)
	List(ctx context.Context) ([]*domain.SubscriptionSource, error)
	DeleteByID(ctx context.Context, id string) error
}

type NodeRepository interface {
	Create(ctx context.Context, node *domain.Node) error
	Update(ctx context.Context, node *domain.Node) error
	GetByID(ctx context.Context, id string) (*domain.Node, error)
	GetBySourceNodeKey(ctx context.Context, sourceID, sourceNodeKey string) (*domain.Node, error)
	List(ctx context.Context) ([]*domain.Node, error)
	DeleteByID(ctx context.Context, id string) error
}

type GroupRepository interface {
	Create(ctx context.Context, group *domain.Group) error
	Update(ctx context.Context, group *domain.Group) error
	GetByID(ctx context.Context, id string) (*domain.Group, error)
	List(ctx context.Context) ([]*domain.Group, error)
	DeleteByID(ctx context.Context, id string) error
}

type TunnelRepository interface {
	Create(ctx context.Context, tunnel *domain.Tunnel) error
	Update(ctx context.Context, tunnel *domain.Tunnel) error
	GetByID(ctx context.Context, id string) (*domain.Tunnel, error)
	List(ctx context.Context) ([]*domain.Tunnel, error)
	DeleteByID(ctx context.Context, id string) error
}

type TunnelEventRepository interface {
	Create(ctx context.Context, event *domain.TunnelEvent) error
	ListByTunnelID(ctx context.Context, tunnelID string, limit int) ([]*domain.TunnelEvent, error)
}

type LatencySampleRepository interface {
	Create(ctx context.Context, sample *domain.LatencySample) error
	ListByNodeID(ctx context.Context, nodeID string, limit int) ([]*domain.LatencySample, error)
	ListByTunnelID(ctx context.Context, tunnelID string, limit int) ([]*domain.LatencySample, error)
}
