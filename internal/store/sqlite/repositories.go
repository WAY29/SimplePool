package sqlite

import "database/sql"

type Repositories struct {
	AdminUsers          *AdminUserRepository
	Sessions            *SessionRepository
	AppSettings         *AppSettingRepository
	SubscriptionSources *SubscriptionSourceRepository
	Nodes               *NodeRepository
	Groups              *GroupRepository
	Tunnels             *TunnelRepository
	TunnelEvents        *TunnelEventRepository
	LatencySamples      *LatencySampleRepository
}

func NewRepositories(db *sql.DB) *Repositories {
	return &Repositories{
		AdminUsers:          &AdminUserRepository{db: db},
		Sessions:            &SessionRepository{db: db},
		AppSettings:         &AppSettingRepository{db: db},
		SubscriptionSources: &SubscriptionSourceRepository{db: db},
		Nodes:               &NodeRepository{db: db},
		Groups:              &GroupRepository{db: db},
		Tunnels:             &TunnelRepository{db: db},
		TunnelEvents:        &TunnelEventRepository{db: db},
		LatencySamples:      &LatencySampleRepository{db: db},
	}
}
