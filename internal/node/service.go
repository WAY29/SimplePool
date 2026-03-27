package node

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/WAY29/SimplePool/internal/domain"
	"github.com/WAY29/SimplePool/internal/store"
	"github.com/google/uuid"
)

var (
	ErrUnsupportedProtocol = errors.New("node: unsupported protocol")
	ErrInvalidPayload      = errors.New("node: invalid payload")
	ErrProbeUnavailable    = errors.New("node: probe unavailable")
)

type Cipher interface {
	Encrypt(plaintext, aad []byte) ([]byte, []byte, error)
	Decrypt(nonce, ciphertext, aad []byte) ([]byte, error)
}

type Prober interface {
	Probe(ctx context.Context, target ProbeTarget) (ProbeResult, error)
}

type Options struct {
	Nodes          store.NodeRepository
	LatencySamples store.LatencySampleRepository
	Cipher         Cipher
	Prober         Prober
	Now            func() time.Time
	ProbeCacheTTL  time.Duration
}

type Service struct {
	nodes          store.NodeRepository
	latencySamples store.LatencySampleRepository
	cipher         Cipher
	prober         Prober
	now            func() time.Time
	probeCacheTTL  time.Duration
}

type CreateManualInput struct {
	Name           string
	Protocol       string
	Server         string
	ServerPort     int
	TransportJSON  string
	TLSJSON        string
	RawPayloadJSON string
	Credential     []byte
}

type UpdateInput struct {
	Name           string
	Protocol       string
	Server         string
	ServerPort     int
	Enabled        bool
	TransportJSON  string
	TLSJSON        string
	RawPayloadJSON string
	Credential     []byte
}

type ImportInput struct {
	Payload string
}

type UpsertImportedInput struct {
	ExistingID           string
	SourceKind           string
	SubscriptionSourceID *string
	SourceNodeKey        string
	Imported             ImportedNode
}

type ProbeTarget struct {
	ID             string
	Name           string
	Protocol       string
	Server         string
	ServerPort     int
	Credential     []byte
	TransportJSON  string
	TLSJSON        string
	RawPayloadJSON string
}

type ProbeResult struct {
	Success      bool       `json:"success"`
	LatencyMS    int64      `json:"latency_ms,omitempty"`
	TestURL      string     `json:"test_url"`
	ErrorMessage string     `json:"error_message,omitempty"`
	Cached       bool       `json:"cached"`
	CheckedAt    *time.Time `json:"checked_at,omitempty"`
}

type ProbeBatchResult struct {
	NodeID string `json:"node_id"`
	ProbeResult
}

type View struct {
	ID                   string     `json:"id"`
	Name                 string     `json:"name"`
	SourceKind           string     `json:"source_kind"`
	SubscriptionSourceID *string    `json:"subscription_source_id,omitempty"`
	Protocol             string     `json:"protocol"`
	Server               string     `json:"server"`
	ServerPort           int        `json:"server_port"`
	TransportJSON        string     `json:"transport_json"`
	TLSJSON              string     `json:"tls_json"`
	RawPayloadJSON       string     `json:"raw_payload_json"`
	Enabled              bool       `json:"enabled"`
	LastLatencyMS        *int64     `json:"last_latency_ms,omitempty"`
	LastStatus           string     `json:"last_status"`
	LastCheckedAt        *time.Time `json:"last_checked_at,omitempty"`
	HasCredential        bool       `json:"has_credential"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type ImportedNode struct {
	Name              string
	Protocol          string
	Server            string
	ServerPort        int
	Credential        []byte
	TransportJSON     string
	TLSJSON           string
	RawPayloadJSON    string
	DedupeFingerprint string
}

func NewService(options Options) *Service {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	if options.ProbeCacheTTL <= 0 {
		options.ProbeCacheTTL = 5 * time.Minute
	}

	return &Service{
		nodes:          options.Nodes,
		latencySamples: options.LatencySamples,
		cipher:         options.Cipher,
		prober:         options.Prober,
		now:            now,
		probeCacheTTL:  options.ProbeCacheTTL,
	}
}

func (s *Service) CreateManual(ctx context.Context, input CreateManualInput) (*View, error) {
	entity, err := s.buildNodeEntity(uuid.NewString(), domain.NodeSourceManual, nil, "", input.Name, input.Protocol, input.Server, input.ServerPort, true, input.TransportJSON, input.TLSJSON, input.RawPayloadJSON, input.Credential)
	if err != nil {
		return nil, err
	}
	if err := s.nodes.Create(ctx, entity); err != nil {
		return nil, err
	}
	return toView(entity), nil
}

func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (*View, error) {
	current, err := s.nodes.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	updated, err := s.buildNodeEntity(current.ID, current.SourceKind, current.SubscriptionSourceID, current.SourceNodeKey, input.Name, input.Protocol, input.Server, input.ServerPort, input.Enabled, input.TransportJSON, input.TLSJSON, input.RawPayloadJSON, input.Credential)
	if err != nil {
		return nil, err
	}
	updated.CreatedAt = current.CreatedAt
	updated.LastLatencyMS = current.LastLatencyMS
	updated.LastStatus = current.LastStatus
	updated.LastCheckedAt = current.LastCheckedAt
	if err := s.nodes.Update(ctx, updated); err != nil {
		return nil, err
	}
	return toView(updated), nil
}

func (s *Service) SetEnabled(ctx context.Context, id string, enabled bool) (*View, error) {
	current, err := s.nodes.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current.Enabled == enabled {
		return toView(current), nil
	}
	current.Enabled = enabled
	current.UpdatedAt = s.now().UTC()
	if err := s.nodes.Update(ctx, current); err != nil {
		return nil, err
	}
	return toView(current), nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.nodes.DeleteByID(ctx, id)
}

func (s *Service) Get(ctx context.Context, id string) (*View, error) {
	entity, err := s.nodes.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toView(entity), nil
}

func (s *Service) List(ctx context.Context) ([]*View, error) {
	items, err := s.nodes.List(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]*View, 0, len(items))
	for _, item := range items {
		views = append(views, toView(item))
	}
	return views, nil
}

func (s *Service) Import(ctx context.Context, input ImportInput) ([]*View, error) {
	imported, err := ParseImportPayload(input.Payload)
	if err != nil {
		return nil, err
	}
	result := make([]*View, 0, len(imported))
	for _, item := range imported {
		view, err := s.UpsertImported(ctx, UpsertImportedInput{
			SourceKind: domain.NodeSourceImport,
			Imported:   item,
		})
		if err != nil {
			return nil, err
		}
		result = append(result, view)
	}
	return result, nil
}

func (s *Service) UpsertImported(ctx context.Context, input UpsertImportedInput) (*View, error) {
	id := input.ExistingID
	if id == "" {
		id = uuid.NewString()
	}
	entity, err := s.buildNodeEntity(
		id,
		input.SourceKind,
		input.SubscriptionSourceID,
		input.SourceNodeKey,
		input.Imported.Name,
		input.Imported.Protocol,
		input.Imported.Server,
		input.Imported.ServerPort,
		true,
		input.Imported.TransportJSON,
		input.Imported.TLSJSON,
		input.Imported.RawPayloadJSON,
		input.Imported.Credential,
	)
	if err != nil {
		return nil, err
	}
	entity.DedupeFingerprint = input.Imported.DedupeFingerprint
	if input.ExistingID == "" {
		if err := s.nodes.Create(ctx, entity); err != nil {
			return nil, err
		}
		return toView(entity), nil
	}

	current, err := s.nodes.GetByID(ctx, input.ExistingID)
	if err != nil {
		return nil, err
	}
	entity.CreatedAt = current.CreatedAt
	entity.LastLatencyMS = current.LastLatencyMS
	entity.LastStatus = current.LastStatus
	entity.LastCheckedAt = current.LastCheckedAt
	if err := s.nodes.Update(ctx, entity); err != nil {
		return nil, err
	}
	return toView(entity), nil
}

func (s *Service) ProbeByID(ctx context.Context, id string, force bool) (ProbeResult, error) {
	entity, err := s.nodes.GetByID(ctx, id)
	if err != nil {
		return ProbeResult{}, err
	}

	if !force {
		cached, ok, err := s.cachedProbeResult(ctx, entity.ID)
		if err != nil {
			return ProbeResult{}, err
		}
		if ok {
			return cached, nil
		}
	}

	target, err := s.toProbeTarget(entity)
	if err != nil {
		return ProbeResult{}, err
	}
	if s.prober == nil {
		return ProbeResult{}, ErrProbeUnavailable
	}

	result, err := s.prober.Probe(ctx, target)
	if err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
	}
	now := s.now().UTC()
	result.CheckedAt = &now
	if err := s.recordProbe(ctx, entity, result, now); err != nil {
		return ProbeResult{}, err
	}
	return result, nil
}

func (s *Service) ProbeBatch(ctx context.Context, ids []string, force bool) ([]ProbeBatchResult, error) {
	results := make([]ProbeBatchResult, len(ids))
	var wg sync.WaitGroup
	var firstErr error
	var firstErrMu sync.Mutex

	for index, id := range ids {
		wg.Add(1)
		go func(index int, id string) {
			defer wg.Done()

			result, err := s.ProbeByID(ctx, id, force)
			if err != nil {
				firstErrMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				firstErrMu.Unlock()
				return
			}
			results[index] = ProbeBatchResult{
				NodeID:      id,
				ProbeResult: result,
			}
		}(index, id)
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}

func (s *Service) buildNodeEntity(id, sourceKind string, subscriptionSourceID *string, sourceNodeKey, name, protocol, server string, serverPort int, enabled bool, transportJSON, tlsJSON, rawPayloadJSON string, credential []byte) (*domain.Node, error) {
	if !IsSupportedProtocol(protocol) {
		return nil, ErrUnsupportedProtocol
	}
	if strings.TrimSpace(name) == "" || strings.TrimSpace(server) == "" || serverPort <= 0 {
		return nil, ErrInvalidPayload
	}
	transportJSON, err := normalizeJSON(transportJSON)
	if err != nil {
		return nil, err
	}
	tlsJSON, err = normalizeJSON(tlsJSON)
	if err != nil {
		return nil, err
	}
	rawPayloadJSON, err = normalizeJSON(rawPayloadJSON)
	if err != nil {
		return nil, err
	}
	credentialJSON, err := normalizeCredential(credential)
	if err != nil {
		return nil, err
	}
	ciphertext, nonce, err := s.cipher.Encrypt(credentialJSON, []byte("node:credential:"+id))
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	entity := &domain.Node{
		ID:                   id,
		Name:                 name,
		SourceNodeKey:        sourceNodeKey,
		DedupeFingerprint:    ComputeDedupeFingerprint(protocol, server, serverPort, credentialJSON, transportJSON, tlsJSON),
		SourceKind:           sourceKind,
		SubscriptionSourceID: subscriptionSourceID,
		Protocol:             protocol,
		Server:               server,
		ServerPort:           serverPort,
		CredentialCiphertext: ciphertext,
		CredentialNonce:      nonce,
		TransportJSON:        transportJSON,
		TLSJSON:              tlsJSON,
		RawPayloadJSON:       rawPayloadJSON,
		Enabled:              enabled,
		LastStatus:           domain.NodeStatusUnknown,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	return entity, nil
}

func (s *Service) toProbeTarget(entity *domain.Node) (ProbeTarget, error) {
	credential, err := s.cipher.Decrypt(entity.CredentialNonce, entity.CredentialCiphertext, []byte("node:credential:"+entity.ID))
	if err != nil {
		return ProbeTarget{}, err
	}
	return ProbeTarget{
		ID:             entity.ID,
		Name:           entity.Name,
		Protocol:       entity.Protocol,
		Server:         entity.Server,
		ServerPort:     entity.ServerPort,
		Credential:     credential,
		TransportJSON:  entity.TransportJSON,
		TLSJSON:        entity.TLSJSON,
		RawPayloadJSON: entity.RawPayloadJSON,
	}, nil
}

func (s *Service) cachedProbeResult(ctx context.Context, nodeID string) (ProbeResult, bool, error) {
	samples, err := s.latencySamples.ListByNodeID(ctx, nodeID, 1)
	if err != nil {
		return ProbeResult{}, false, err
	}
	if len(samples) == 0 {
		return ProbeResult{}, false, nil
	}
	latest := samples[0]
	if s.now().UTC().Sub(latest.CreatedAt) > s.probeCacheTTL {
		return ProbeResult{}, false, nil
	}
	result := ProbeResult{
		Success:      latest.Success,
		TestURL:      latest.TestURL,
		ErrorMessage: latest.ErrorMessage,
		Cached:       true,
		CheckedAt:    &latest.CreatedAt,
	}
	if latest.LatencyMS != nil {
		result.LatencyMS = *latest.LatencyMS
	}
	return result, true, nil
}

func (s *Service) recordProbe(ctx context.Context, entity *domain.Node, result ProbeResult, checkedAt time.Time) error {
	var latency *int64
	if result.Success {
		latency = &result.LatencyMS
		entity.LastStatus = domain.NodeStatusHealthy
	} else {
		entity.LastStatus = domain.NodeStatusUnreachable
	}
	entity.LastLatencyMS = latency
	entity.LastCheckedAt = &checkedAt
	entity.UpdatedAt = checkedAt
	if err := s.nodes.Update(ctx, entity); err != nil {
		return err
	}
	return s.latencySamples.Create(ctx, &domain.LatencySample{
		ID:           uuid.NewString(),
		NodeID:       entity.ID,
		TestURL:      result.TestURL,
		LatencyMS:    latency,
		Success:      result.Success,
		ErrorMessage: result.ErrorMessage,
		CreatedAt:    checkedAt,
	})
}

func toView(entity *domain.Node) *View {
	return &View{
		ID:                   entity.ID,
		Name:                 entity.Name,
		SourceKind:           entity.SourceKind,
		SubscriptionSourceID: entity.SubscriptionSourceID,
		Protocol:             entity.Protocol,
		Server:               entity.Server,
		ServerPort:           entity.ServerPort,
		TransportJSON:        entity.TransportJSON,
		TLSJSON:              entity.TLSJSON,
		RawPayloadJSON:       entity.RawPayloadJSON,
		Enabled:              entity.Enabled,
		LastLatencyMS:        entity.LastLatencyMS,
		LastStatus:           entity.LastStatus,
		LastCheckedAt:        entity.LastCheckedAt,
		HasCredential:        len(entity.CredentialCiphertext) > 0,
		CreatedAt:            entity.CreatedAt,
		UpdatedAt:            entity.UpdatedAt,
	}
}

func normalizeCredential(raw []byte) ([]byte, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, ErrInvalidPayload
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("%w: credential json", ErrInvalidPayload)
	}
	return json.Marshal(payload)
}

func normalizeJSON(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return `{}`, nil
	}
	var payload any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", fmt.Errorf("%w: invalid json", ErrInvalidPayload)
	}
	normalized, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}

func ComputeDedupeFingerprint(protocol, server string, serverPort int, credential []byte, transportJSON, tlsJSON string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%s|%s|%s", strings.ToLower(protocol), strings.ToLower(server), serverPort, string(credential), transportJSON, tlsJSON)))
	return hex.EncodeToString(sum[:])
}

func ComputeSourceNodeKey(sourceID, dedupeFingerprint string) string {
	sum := sha256.Sum256([]byte(sourceID + ":" + dedupeFingerprint))
	return hex.EncodeToString(sum[:])
}

func IsSupportedProtocol(protocol string) bool {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "ss", "trojan", "vmess", "vless", "hysteria2", "hy2":
		return true
	default:
		return false
	}
}
