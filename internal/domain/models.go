package domain

import "time"

const (
	NodeSourceManual       = "manual"
	NodeSourceImport       = "import"
	NodeSourceSubscription = "subscription"
)

const (
	NodeStatusUnknown     = "unknown"
	NodeStatusHealthy     = "healthy"
	NodeStatusUnreachable = "unreachable"
)

const (
	TunnelStatusStopped  = "stopped"
	TunnelStatusStarting = "starting"
	TunnelStatusRunning  = "running"
	TunnelStatusDegraded = "degraded"
	TunnelStatusError    = "error"
)

type AdminUser struct {
	ID           string
	Username     string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	ID         string
	UserID     string
	TokenHash  string
	ExpiresAt  time.Time
	CreatedAt  time.Time
	LastSeenAt time.Time
}

type AppSetting struct {
	Key       string
	Value     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SubscriptionSource struct {
	ID               string
	Name             string
	FetchFingerprint string
	URLCiphertext    []byte
	URLNonce         []byte
	Enabled          bool
	LastRefreshAt    *time.Time
	LastError        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Node struct {
	ID                   string
	Name                 string
	SourceNodeKey        string
	DedupeFingerprint    string
	SourceKind           string
	SubscriptionSourceID *string
	Protocol             string
	Server               string
	ServerPort           int
	CredentialCiphertext []byte
	CredentialNonce      []byte
	TransportJSON        string
	TLSJSON              string
	RawPayloadJSON       string
	Enabled              bool
	LastLatencyMS        *int64
	LastStatus           string
	LastCheckedAt        *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type Group struct {
	ID          string
	Name        string
	FilterRegex string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Tunnel struct {
	ID                         string
	Name                       string
	GroupID                    string
	ListenHost                 string
	ListenPort                 int
	Status                     string
	CurrentNodeID              *string
	AuthUsernameCiphertext     []byte
	AuthPasswordCiphertext     []byte
	AuthNonce                  []byte
	ControllerPort             int
	ControllerSecretCiphertext []byte
	ControllerSecretNonce      []byte
	RuntimeDir                 string
	RuntimeConfigJSON          string
	LastRefreshAt              *time.Time
	LastRefreshError           string
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

type TunnelEvent struct {
	ID         string
	TunnelID   string
	EventType  string
	DetailJSON string
	CreatedAt  time.Time
}

type LatencySample struct {
	ID           string
	NodeID       string
	TunnelID     *string
	TestURL      string
	LatencyMS    *int64
	Success      bool
	ErrorMessage string
	CreatedAt    time.Time
}
