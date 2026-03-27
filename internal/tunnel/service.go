package tunnel

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/WAY29/SimplePool/internal/domain"
	"github.com/WAY29/SimplePool/internal/group"
	"github.com/WAY29/SimplePool/internal/node"
	"github.com/WAY29/SimplePool/internal/runtime/singbox"
	"github.com/WAY29/SimplePool/internal/security"
	"github.com/WAY29/SimplePool/internal/store"
	"github.com/google/uuid"
)

var (
	ErrInvalidPayload   = errors.New("tunnel: invalid payload")
	ErrNoAvailableNodes = errors.New("tunnel: no available nodes")
	ErrTunnelNotRunning = errors.New("tunnel: tunnel not running")
	ErrRuntimeConfigNil = errors.New("tunnel: runtime config missing")
	ErrTunnelConflict   = errors.New("tunnel: duplicate name in group")
)

const selectorTag = "tunnel-selector"

type Cipher interface {
	Encrypt(plaintext, aad []byte) ([]byte, []byte, error)
	Decrypt(nonce, ciphertext, aad []byte) ([]byte, error)
}

type Prober interface {
	Probe(ctx context.Context, target node.ProbeTarget) (node.ProbeResult, error)
}

type Renderer interface {
	Render(input singbox.RenderInput) ([]byte, error)
}

type PortAllocator interface {
	AllocatePair() (singbox.PortPair, error)
	Release(port int) error
	Close() error
}

type RuntimeManager interface {
	Start(ctx context.Context, tunnelID string, layout singbox.RuntimeLayout, config []byte) error
	Stop(ctx context.Context, tunnelID string) error
	Delete(ctx context.Context, tunnelID string) error
	GetSelector(ctx context.Context, tunnelID string, controllerPort int, secret string) (*singbox.ProxyInfo, error)
	SwitchSelector(ctx context.Context, tunnelID string, controllerPort int, secret, outbound string) error
	Close() error
}

type Options struct {
	Tunnels        store.TunnelRepository
	TunnelEvents   store.TunnelEventRepository
	LatencySamples store.LatencySampleRepository
	Groups         *group.Service
	Nodes          store.NodeRepository
	LogLevel       string
	Cipher         Cipher
	Prober         Prober
	ProbeCacheTTL  time.Duration
	Runtime        RuntimeManager
	PortAllocator  PortAllocator
	Renderer       Renderer
	RuntimeRoot    string
	Now            func() time.Time
	Logger         *slog.Logger
}

type Service struct {
	tunnels        store.TunnelRepository
	tunnelEvents   store.TunnelEventRepository
	latencySamples store.LatencySampleRepository
	groups         *group.Service
	nodes          store.NodeRepository
	logLevel       string
	cipher         Cipher
	prober         Prober
	probeCacheTTL  time.Duration
	runtime        RuntimeManager
	portAllocator  PortAllocator
	renderer       Renderer
	runtimeRoot    string
	now            func() time.Time
	logger         *slog.Logger
}

type CreateInput struct {
	Name       string
	GroupID    string
	ListenHost string
	Username   string
	Password   string
}

type UpdateInput struct {
	Name       string
	GroupID    string
	ListenHost string
	Username   string
	Password   string
}

type View struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	GroupID          string     `json:"group_id"`
	ListenHost       string     `json:"listen_host"`
	ListenPort       int        `json:"listen_port"`
	Status           string     `json:"status"`
	CurrentNodeID    *string    `json:"current_node_id,omitempty"`
	ControllerPort   int        `json:"controller_port"`
	RuntimeDir       string     `json:"runtime_dir"`
	LastRefreshAt    *time.Time `json:"last_refresh_at,omitempty"`
	LastRefreshError string     `json:"last_refresh_error"`
	HasAuth          bool       `json:"has_auth"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type EventView struct {
	ID         string    `json:"id"`
	TunnelID   string    `json:"tunnel_id"`
	EventType  string    `json:"event_type"`
	DetailJSON string    `json:"detail_json"`
	CreatedAt  time.Time `json:"created_at"`
}

type authPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type preparedRuntime struct {
	config           []byte
	currentNode      *domain.Node
	selectorTags     []string
	cancelBackground context.CancelFunc
	backgroundDone   <-chan struct{}
}

type runtimeCandidate struct {
	item        *domain.Node
	target      node.ProbeTarget
	runtimeNode singbox.RuntimeNode
}

type probeOutcome struct {
	item   *domain.Node
	result node.ProbeResult
}

func NewService(options Options) *Service {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	if options.ProbeCacheTTL <= 0 {
		options.ProbeCacheTTL = 5 * time.Minute
	}
	if options.PortAllocator == nil {
		options.PortAllocator = singbox.NewPortAllocator()
	}
	if options.Renderer == nil {
		options.Renderer = singbox.NewConfigRenderer()
	}
	if options.Runtime == nil {
		options.Runtime = NewRuntimeManager(RuntimeManagerOptions{Now: now})
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		tunnels:        options.Tunnels,
		tunnelEvents:   options.TunnelEvents,
		latencySamples: options.LatencySamples,
		groups:         options.Groups,
		nodes:          options.Nodes,
		logLevel:       options.LogLevel,
		cipher:         options.Cipher,
		prober:         options.Prober,
		probeCacheTTL:  options.ProbeCacheTTL,
		runtime:        options.Runtime,
		portAllocator:  options.PortAllocator,
		renderer:       options.Renderer,
		runtimeRoot:    options.RuntimeRoot,
		now:            now,
		logger:         logger,
	}
}

func (s *Service) Close() error {
	var errs []error
	if s.runtime != nil {
		errs = append(errs, s.runtime.Close())
	}
	if s.portAllocator != nil {
		errs = append(errs, s.portAllocator.Close())
	}
	return errors.Join(errs...)
}

func (s *Service) Initialize(ctx context.Context) error {
	items, err := s.tunnels.List(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Status == domain.TunnelStatusStopped {
			continue
		}
		item.Status = domain.TunnelStatusStopped
		item.LastRefreshError = "runtime state reset on startup"
		item.UpdatedAt = s.now().UTC()
		if err := s.tunnels.Update(ctx, item); err != nil {
			return err
		}
		s.logInfo("tunnel runtime reconciled", "tunnel_id", item.ID)
	}
	return nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*View, error) {
	name, groupID, listenHost, proxyAuth, err := s.normalizeInput(input.Name, input.GroupID, input.ListenHost, input.Username, input.Password)
	if err != nil {
		return nil, err
	}
	if err := s.ensureUniqueTunnelName(ctx, groupID, name, ""); err != nil {
		return nil, err
	}

	now := s.now().UTC()
	id := uuid.NewString()
	layout, err := s.runtimeLayoutFor(ctx, groupID, name)
	if err != nil {
		return nil, err
	}

	pair, err := s.portAllocator.AllocatePair()
	if err != nil {
		return nil, err
	}
	releaseReserved := true
	defer func() {
		if !releaseReserved {
			return
		}
		_ = s.portAllocator.Release(pair.ProxyPort)
		_ = s.portAllocator.Release(pair.ControllerPort)
	}()

	controllerSecret, err := security.GenerateSessionToken(rand.Reader)
	if err != nil {
		return nil, err
	}
	authCiphertext, authNonce, err := s.encryptAuth(id, proxyAuth)
	if err != nil {
		return nil, err
	}
	controllerCiphertext, controllerNonce, err := s.cipher.Encrypt([]byte(controllerSecret), []byte("tunnel:controller:"+id))
	if err != nil {
		return nil, err
	}

	entity := &domain.Tunnel{
		ID:                         id,
		Name:                       name,
		GroupID:                    groupID,
		ListenHost:                 listenHost,
		ListenPort:                 pair.ProxyPort,
		Status:                     domain.TunnelStatusStarting,
		AuthUsernameCiphertext:     nil,
		AuthPasswordCiphertext:     authCiphertext,
		AuthNonce:                  authNonce,
		ControllerPort:             pair.ControllerPort,
		ControllerSecretCiphertext: controllerCiphertext,
		ControllerSecretNonce:      controllerNonce,
		RuntimeDir:                 layout.RootDir,
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}
	if err := s.tunnels.Create(ctx, entity); err != nil {
		return nil, err
	}

	created := false
	defer func() {
		if created {
			return
		}
		_ = s.runtime.Delete(ctx, entity.ID)
		_ = os.RemoveAll(layout.RootDir)
		_ = s.tunnels.DeleteByID(ctx, entity.ID)
	}()

	prepared, err := s.prepareRuntime(ctx, entity.ID, entity.GroupID, layout, entity.ListenHost, entity.ListenPort, entity.ControllerPort, controllerSecret, proxyAuth)
	if err != nil {
		return nil, err
	}
	if err := s.releaseReservedPorts(pair); err != nil {
		prepared.stopBackground()
		return nil, err
	}
	releaseReserved = false

	if err := s.runtime.Start(ctx, entity.ID, layout, prepared.config); err != nil {
		prepared.stopBackground()
		return nil, err
	}

	entity.Status = domain.TunnelStatusRunning
	entity.CurrentNodeID = stringPtr(prepared.currentNode.ID)
	entity.RuntimeConfigJSON = string(prepared.config)
	entity.LastRefreshError = ""
	entity.UpdatedAt = s.now().UTC()
	if err := s.tunnels.Update(ctx, entity); err != nil {
		prepared.stopBackground()
		_ = s.runtime.Delete(ctx, entity.ID)
		return nil, err
	}
	s.recordEvent(ctx, entity.ID, "tunnel.created", map[string]any{
		"current_node_id": entity.CurrentNodeID,
		"listen_port":     entity.ListenPort,
	})
	s.logInfo("tunnel created", "tunnel_id", entity.ID, "group_id", entity.GroupID, "listen_port", entity.ListenPort)
	created = true
	return toView(entity), nil
}

func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (*View, error) {
	current, err := s.tunnels.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	name, groupID, listenHost, proxyAuth, err := s.normalizeInput(input.Name, input.GroupID, input.ListenHost, input.Username, input.Password)
	if err != nil {
		return nil, err
	}
	if err := s.ensureUniqueTunnelName(ctx, groupID, name, current.ID); err != nil {
		return nil, err
	}
	if current.Status == domain.TunnelStatusStopped {
		layout, err := s.runtimeLayoutFor(ctx, groupID, name)
		if err != nil {
			return nil, err
		}
		oldRuntimeDir := current.RuntimeDir
		authCiphertext, authNonce, err := s.encryptAuth(current.ID, proxyAuth)
		if err != nil {
			return nil, err
		}
		current.Name = name
		current.GroupID = groupID
		current.ListenHost = listenHost
		current.AuthUsernameCiphertext = nil
		current.AuthPasswordCiphertext = authCiphertext
		current.AuthNonce = authNonce
		current.RuntimeDir = layout.RootDir
		current.UpdatedAt = s.now().UTC()
		if err := s.tunnels.Update(ctx, current); err != nil {
			return nil, err
		}
		if oldRuntimeDir != "" && oldRuntimeDir != current.RuntimeDir {
			if err := os.RemoveAll(oldRuntimeDir); err != nil {
				return nil, err
			}
		}
		s.recordEvent(ctx, current.ID, "tunnel.updated", map[string]any{
			"group_id": groupID,
			"status":   current.Status,
		})
		return toView(current), nil
	}

	return s.rebuildTunnel(ctx, current, rebuildInput{
		Name:             name,
		GroupID:          groupID,
		ListenHost:       listenHost,
		ProxyAuth:        proxyAuth,
		RecordEvent:      "tunnel.updated",
		FailureEvent:     "tunnel.update_failed",
		FailureStatus:    domain.TunnelStatusDegraded,
		UpdateRefreshAt:  true,
		UpdateTunnelName: true,
	})
}

func (s *Service) Start(ctx context.Context, id string) (*View, error) {
	current, err := s.tunnels.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current.Status != domain.TunnelStatusStopped {
		return toView(current), nil
	}

	controllerSecret, err := s.decryptControllerSecret(current)
	if err != nil {
		return nil, err
	}
	proxyAuth, err := s.decryptAuth(current)
	if err != nil {
		return nil, err
	}

	layout, err := s.runtimeLayoutFor(ctx, current.GroupID, current.Name)
	if err != nil {
		return nil, err
	}
	prepared, err := s.prepareRuntime(ctx, current.ID, current.GroupID, layout, current.ListenHost, current.ListenPort, current.ControllerPort, controllerSecret, proxyAuth)
	if err != nil {
		return nil, err
	}
	if err := s.runtime.Start(ctx, current.ID, layout, prepared.config); err != nil {
		prepared.stopBackground()
		return nil, err
	}

	current.Status = domain.TunnelStatusRunning
	current.CurrentNodeID = stringPtr(prepared.currentNode.ID)
	current.RuntimeDir = layout.RootDir
	current.RuntimeConfigJSON = string(prepared.config)
	current.LastRefreshError = ""
	current.UpdatedAt = s.now().UTC()
	if err := s.tunnels.Update(ctx, current); err != nil {
		prepared.stopBackground()
		_ = s.runtime.Stop(ctx, current.ID)
		return nil, err
	}
	s.recordEvent(ctx, current.ID, "tunnel.started", map[string]any{
		"current_node_id": current.CurrentNodeID,
	})
	s.logInfo("tunnel started", "tunnel_id", current.ID)
	return toView(current), nil
}

func (s *Service) Stop(ctx context.Context, id string) (*View, error) {
	current, err := s.tunnels.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current.Status == domain.TunnelStatusStopped {
		return toView(current), nil
	}
	if err := s.runtime.Stop(ctx, current.ID); err != nil {
		return nil, err
	}
	current.Status = domain.TunnelStatusStopped
	current.UpdatedAt = s.now().UTC()
	if err := s.tunnels.Update(ctx, current); err != nil {
		return nil, err
	}
	s.recordEvent(ctx, current.ID, "tunnel.stopped", nil)
	s.logInfo("tunnel stopped", "tunnel_id", current.ID)
	return toView(current), nil
}

func (s *Service) Refresh(ctx context.Context, id string) (*View, error) {
	current, err := s.tunnels.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current.Status == domain.TunnelStatusStopped {
		return nil, ErrTunnelNotRunning
	}

	controllerSecret, err := s.decryptControllerSecret(current)
	if err != nil {
		return nil, err
	}
	proxyAuth, err := s.decryptAuth(current)
	if err != nil {
		return nil, err
	}

	layout, err := s.runtimeLayoutFor(ctx, current.GroupID, current.Name)
	if err != nil {
		return nil, err
	}
	prepared, err := s.prepareRuntime(ctx, current.ID, current.GroupID, layout, current.ListenHost, current.ListenPort, current.ControllerPort, controllerSecret, proxyAuth)
	if err != nil {
		return nil, s.markRefreshFailure(ctx, current, err)
	}

	selector, err := s.runtime.GetSelector(ctx, current.ID, current.ControllerPort, controllerSecret)
	if err == nil && sameStringSet(selector.All, prepared.selectorTags) {
		targetTag := outboundTag(prepared.currentNode.ID)
		if selector.Now != targetTag {
			if err := s.runtime.SwitchSelector(ctx, current.ID, current.ControllerPort, controllerSecret, targetTag); err != nil {
				prepared.stopBackground()
				return nil, s.markRefreshFailure(ctx, current, err)
			}
		}
		now := s.now().UTC()
		current.Status = domain.TunnelStatusRunning
		current.CurrentNodeID = stringPtr(prepared.currentNode.ID)
		current.LastRefreshAt = &now
		current.LastRefreshError = ""
		current.UpdatedAt = now
		if err := s.tunnels.Update(ctx, current); err != nil {
			prepared.stopBackground()
			return nil, err
		}
		s.recordEvent(ctx, current.ID, "tunnel.refreshed", map[string]any{
			"current_node_id": current.CurrentNodeID,
			"mode":            "selector_switch",
		})
		s.logInfo("tunnel refreshed by selector switch", "tunnel_id", current.ID, "node_id", prepared.currentNode.ID)
		return toView(current), nil
	}

	return s.rebuildTunnel(ctx, current, rebuildInput{
		Name:             current.Name,
		GroupID:          current.GroupID,
		ListenHost:       current.ListenHost,
		ProxyAuth:        proxyAuth,
		Prepared:         prepared,
		RecordEvent:      "tunnel.refreshed",
		FailureEvent:     "tunnel.refresh_failed",
		FailureStatus:    domain.TunnelStatusDegraded,
		UpdateRefreshAt:  true,
		UpdateTunnelName: false,
	})
}

func (s *Service) Delete(ctx context.Context, id string) error {
	current, err := s.tunnels.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.runtime.Delete(ctx, current.ID); err != nil {
		return err
	}
	if err := os.RemoveAll(current.RuntimeDir); err != nil {
		return err
	}
	s.logInfo("tunnel deleted", "tunnel_id", current.ID)
	return s.tunnels.DeleteByID(ctx, current.ID)
}

func (s *Service) Get(ctx context.Context, id string) (*View, error) {
	item, err := s.tunnels.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toView(item), nil
}

func (s *Service) List(ctx context.Context) ([]*View, error) {
	items, err := s.tunnels.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*View, 0, len(items))
	for _, item := range items {
		result = append(result, toView(item))
	}
	return result, nil
}

func (s *Service) ListEvents(ctx context.Context, id string, limit int) ([]*EventView, error) {
	if _, err := s.tunnels.GetByID(ctx, id); err != nil {
		return nil, err
	}
	items, err := s.tunnelEvents.ListByTunnelID(ctx, id, limit)
	if err != nil {
		return nil, err
	}
	result := make([]*EventView, 0, len(items))
	for _, item := range items {
		result = append(result, &EventView{
			ID:         item.ID,
			TunnelID:   item.TunnelID,
			EventType:  item.EventType,
			DetailJSON: item.DetailJSON,
			CreatedAt:  item.CreatedAt,
		})
	}
	return result, nil
}

type rebuildInput struct {
	Name             string
	GroupID          string
	ListenHost       string
	ProxyAuth        *singbox.ProxyAuth
	Prepared         *preparedRuntime
	RecordEvent      string
	FailureEvent     string
	FailureStatus    string
	UpdateRefreshAt  bool
	UpdateTunnelName bool
}

func (s *Service) rebuildTunnel(ctx context.Context, current *domain.Tunnel, input rebuildInput) (*View, error) {
	oldRuntimeDir := current.RuntimeDir
	oldLayout := runtimeLayoutFromRoot(current.RuntimeDir)
	layout, err := s.runtimeLayoutFor(ctx, input.GroupID, input.Name)
	if err != nil {
		return nil, err
	}
	oldConfig, err := loadStoredRuntimeConfig(current)
	if err != nil {
		return nil, err
	}

	controllerSecret, err := s.decryptControllerSecret(current)
	if err != nil {
		return nil, err
	}
	prepared := input.Prepared
	if prepared == nil {
		prepared, err = s.prepareRuntime(ctx, current.ID, input.GroupID, layout, input.ListenHost, current.ListenPort, current.ControllerPort, controllerSecret, input.ProxyAuth)
	}
	if err != nil {
		s.recordEvent(ctx, current.ID, input.FailureEvent, map[string]any{"error": err.Error()})
		return nil, err
	}

	if err := s.runtime.Stop(ctx, current.ID); err != nil {
		prepared.stopBackground()
		s.recordEvent(ctx, current.ID, input.FailureEvent, map[string]any{"error": err.Error()})
		return nil, err
	}
	if err := s.runtime.Start(ctx, current.ID, layout, prepared.config); err != nil {
		prepared.stopBackground()
		restartErr := s.runtime.Start(ctx, current.ID, oldLayout, oldConfig)
		if restartErr != nil {
			err = errors.Join(err, restartErr)
		}
		current.Status = input.FailureStatus
		current.LastRefreshError = err.Error()
		current.UpdatedAt = s.now().UTC()
		_ = s.tunnels.Update(ctx, current)
		s.recordEvent(ctx, current.ID, input.FailureEvent, map[string]any{"error": err.Error()})
		s.logError("tunnel rebuild failed", "tunnel_id", current.ID, "error", err)
		return nil, err
	}

	authCiphertext, authNonce, err := s.encryptAuth(current.ID, input.ProxyAuth)
	if err != nil {
		return nil, err
	}

	previous := *current
	if input.UpdateTunnelName {
		current.Name = input.Name
		current.GroupID = input.GroupID
		current.ListenHost = input.ListenHost
		current.AuthUsernameCiphertext = nil
		current.AuthPasswordCiphertext = authCiphertext
		current.AuthNonce = authNonce
		current.RuntimeDir = layout.RootDir
	}
	if current.RuntimeDir == "" {
		current.RuntimeDir = layout.RootDir
	}
	now := s.now().UTC()
	current.Status = domain.TunnelStatusRunning
	current.CurrentNodeID = stringPtr(prepared.currentNode.ID)
	current.RuntimeConfigJSON = string(prepared.config)
	current.LastRefreshError = ""
	if input.UpdateRefreshAt {
		current.LastRefreshAt = &now
	}
	current.UpdatedAt = now
	if err := s.tunnels.Update(ctx, current); err != nil {
		prepared.stopBackground()
		_ = s.runtime.Stop(ctx, current.ID)
		_ = s.runtime.Start(ctx, current.ID, oldLayout, oldConfig)
		*current = previous
		return nil, err
	}
	if oldRuntimeDir != "" && oldRuntimeDir != current.RuntimeDir {
		if err := os.RemoveAll(oldRuntimeDir); err != nil {
			return nil, err
		}
	}

	s.recordEvent(ctx, current.ID, input.RecordEvent, map[string]any{
		"current_node_id": current.CurrentNodeID,
		"mode":            "rebuild",
	})
	s.logInfo("tunnel rebuilt", "tunnel_id", current.ID, "event", input.RecordEvent)
	return toView(current), nil
}

func (s *Service) prepareRuntime(ctx context.Context, tunnelID, groupID string, layout singbox.RuntimeLayout, listenHost string, listenPort, controllerPort int, controllerSecret string, proxyAuth *singbox.ProxyAuth) (*preparedRuntime, error) {
	nodes, err := s.loadGroupSnapshot(ctx, groupID)
	if err != nil {
		return nil, err
	}

	candidates := make([]runtimeCandidate, 0, len(nodes))
	runtimeNodes := make([]singbox.RuntimeNode, 0, len(nodes))
	selectorTags := make([]string, 0, len(nodes))
	var cachedNode *domain.Node
	var cachedLatency int64
	for _, item := range nodes {
		target, runtimeNode, err := s.buildRuntimeNode(item)
		if err != nil {
			return nil, err
		}
		if cached, ok, err := s.cachedSuccessfulProbeResult(ctx, item.ID); err != nil {
			return nil, err
		} else if ok && (cachedNode == nil || cached.LatencyMS < cachedLatency) {
			cachedNode = item
			cachedLatency = cached.LatencyMS
		}
		candidates = append(candidates, runtimeCandidate{
			item:        item,
			target:      target,
			runtimeNode: runtimeNode,
		})
		runtimeNodes = append(runtimeNodes, runtimeNode)
		selectorTags = append(selectorTags, outboundTag(item.ID))
	}

	if len(runtimeNodes) == 0 {
		return nil, ErrNoAvailableNodes
	}
	if cachedNode != nil {
		return s.renderPreparedRuntime(layout, listenHost, listenPort, controllerPort, controllerSecret, proxyAuth, runtimeNodes, cachedNode, nil, nil)
	}
	return s.prepareRuntimeWithConcurrentProbes(ctx, tunnelID, layout, listenHost, listenPort, controllerPort, controllerSecret, proxyAuth, candidates, runtimeNodes, selectorTags)
}

func (s *Service) prepareRuntimeWithConcurrentProbes(ctx context.Context, tunnelID string, layout singbox.RuntimeLayout, listenHost string, listenPort, controllerPort int, controllerSecret string, proxyAuth *singbox.ProxyAuth, candidates []runtimeCandidate, runtimeNodes []singbox.RuntimeNode, selectorTags []string) (*preparedRuntime, error) {
	probeCtx, cancel := context.WithCancel(context.Background())
	successCh := make(chan probeOutcome, 1)
	errCh := make(chan error, 1)
	doneCh := make(chan struct{})
	var wg sync.WaitGroup

	for _, candidate := range candidates {
		wg.Add(1)
		go func(candidate runtimeCandidate) {
			defer wg.Done()
			s.probeCandidate(probeCtx, tunnelID, candidate, successCh, errCh)
		}(candidate)
	}

	go func() {
		wg.Wait()
		close(doneCh)
	}()

	for {
		select {
		case outcome := <-successCh:
			return s.renderPreparedRuntime(layout, listenHost, listenPort, controllerPort, controllerSecret, proxyAuth, runtimeNodes, outcome.item, cancel, doneCh)
		case err := <-errCh:
			cancel()
			waitForProbeDrain(doneCh)
			return nil, err
		case <-doneCh:
			cancel()
			return nil, ErrNoAvailableNodes
		case <-ctx.Done():
			cancel()
			waitForProbeDrain(doneCh)
			return nil, ctx.Err()
		}
	}
}

func (s *Service) renderPreparedRuntime(layout singbox.RuntimeLayout, listenHost string, listenPort, controllerPort int, controllerSecret string, proxyAuth *singbox.ProxyAuth, runtimeNodes []singbox.RuntimeNode, currentNode *domain.Node, cancelBackground context.CancelFunc, backgroundDone <-chan struct{}) (*preparedRuntime, error) {
	selectedRuntimeNodes, selectedSelectorTags := runtimeNodesForCurrentProtocol(runtimeNodes, currentNode.Protocol)
	config, err := s.renderer.Render(singbox.RenderInput{
		ListenHost:       listenHost,
		ListenPort:       listenPort,
		LogLevel:         s.logLevel,
		Auth:             proxyAuth,
		ControllerPort:   controllerPort,
		ControllerSecret: controllerSecret,
		Nodes:            selectedRuntimeNodes,
		CurrentNodeID:    currentNode.ID,
	})
	if err != nil {
		if cancelBackground != nil {
			cancelBackground()
			waitForProbeDrain(backgroundDone)
		}
		return nil, err
	}

	return &preparedRuntime{
		config:           config,
		currentNode:      currentNode,
		selectorTags:     selectedSelectorTags,
		cancelBackground: cancelBackground,
		backgroundDone:   backgroundDone,
	}, nil
}

func runtimeNodesForCurrentProtocol(nodes []singbox.RuntimeNode, protocol string) ([]singbox.RuntimeNode, []string) {
	filteredNodes := make([]singbox.RuntimeNode, 0, len(nodes))
	filteredTags := make([]string, 0, len(nodes))
	for _, item := range nodes {
		if !strings.EqualFold(item.Protocol, protocol) {
			continue
		}
		filteredNodes = append(filteredNodes, item)
		filteredTags = append(filteredTags, outboundTag(item.ID))
	}
	if len(filteredNodes) == 0 {
		filteredNodes = append(filteredNodes, nodes...)
		for _, item := range nodes {
			filteredTags = append(filteredTags, outboundTag(item.ID))
		}
	}
	return filteredNodes, filteredTags
}

func (s *Service) probeCandidate(ctx context.Context, tunnelID string, candidate runtimeCandidate, successCh chan<- probeOutcome, errCh chan<- error) {
	result, probeErr := s.prober.Probe(ctx, candidate.target)
	if probeErr != nil {
		result = node.ProbeResult{
			Success:      false,
			TestURL:      "",
			ErrorMessage: probeErr.Error(),
		}
	}
	checkedAt := s.now().UTC()
	if result.TestURL == "" {
		result.TestURL = "https://cloudflare.com/cdn-cgi/trace"
	}
	if ctx.Err() != nil {
		return
	}
	if err := s.recordProbe(ctx, tunnelID, candidate.item, result, checkedAt); err != nil {
		select {
		case errCh <- err:
		default:
			s.logError("record tunnel probe failed", "tunnel_id", tunnelID, "node_id", candidate.item.ID, "error", err)
		}
		return
	}
	if !result.Success {
		return
	}
	select {
	case successCh <- probeOutcome{item: candidate.item, result: result}:
	default:
	}
}

func (s *Service) cachedSuccessfulProbeResult(ctx context.Context, nodeID string) (node.ProbeResult, bool, error) {
	samples, err := s.latencySamples.ListByNodeID(ctx, nodeID, 1)
	if err != nil {
		return node.ProbeResult{}, false, err
	}
	if len(samples) == 0 {
		return node.ProbeResult{}, false, nil
	}
	latest := samples[0]
	if !latest.Success || s.now().UTC().Sub(latest.CreatedAt) > s.probeCacheTTL {
		return node.ProbeResult{}, false, nil
	}
	result := node.ProbeResult{
		Success:      true,
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

func (s *Service) loadGroupSnapshot(ctx context.Context, groupID string) ([]*domain.Node, error) {
	members, err := s.groups.ListMembers(ctx, groupID)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Node, 0, len(members))
	for _, item := range members {
		if !item.Enabled {
			continue
		}
		entity, err := s.nodes.GetByID(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, entity)
	}
	if len(result) == 0 {
		return nil, ErrNoAvailableNodes
	}
	return result, nil
}

func loadStoredRuntimeConfig(item *domain.Tunnel) ([]byte, error) {
	if item == nil || item.RuntimeConfigJSON == "" {
		return nil, ErrRuntimeConfigNil
	}
	return []byte(item.RuntimeConfigJSON), nil
}

func (s *Service) buildRuntimeNode(item *domain.Node) (node.ProbeTarget, singbox.RuntimeNode, error) {
	credential, err := s.cipher.Decrypt(item.CredentialNonce, item.CredentialCiphertext, []byte("node:credential:"+item.ID))
	if err != nil {
		return node.ProbeTarget{}, singbox.RuntimeNode{}, err
	}
	target := node.ProbeTarget{
		ID:             item.ID,
		Name:           item.Name,
		Protocol:       item.Protocol,
		Server:         item.Server,
		ServerPort:     item.ServerPort,
		Credential:     credential,
		TransportJSON:  item.TransportJSON,
		TLSJSON:        item.TLSJSON,
		RawPayloadJSON: item.RawPayloadJSON,
	}
	runtimeNode := singbox.RuntimeNode{
		ID:             item.ID,
		Name:           item.Name,
		Protocol:       item.Protocol,
		Server:         item.Server,
		ServerPort:     item.ServerPort,
		Credential:     credential,
		TransportJSON:  item.TransportJSON,
		TLSJSON:        item.TLSJSON,
		RawPayloadJSON: item.RawPayloadJSON,
	}
	return target, runtimeNode, nil
}

func (s *Service) recordProbe(ctx context.Context, tunnelID string, item *domain.Node, result node.ProbeResult, checkedAt time.Time) error {
	var latency *int64
	if result.Success {
		latency = &result.LatencyMS
		item.LastStatus = domain.NodeStatusHealthy
	} else {
		item.LastStatus = domain.NodeStatusUnreachable
	}
	item.LastLatencyMS = latency
	item.LastCheckedAt = &checkedAt
	item.UpdatedAt = checkedAt
	if err := s.nodes.Update(ctx, item); err != nil {
		return err
	}

	var tunnelRef *string
	if tunnelID != "" {
		tunnelRef = &tunnelID
	}
	return s.latencySamples.Create(ctx, &domain.LatencySample{
		ID:           uuid.NewString(),
		NodeID:       item.ID,
		TunnelID:     tunnelRef,
		TestURL:      result.TestURL,
		LatencyMS:    latency,
		Success:      result.Success,
		ErrorMessage: result.ErrorMessage,
		CreatedAt:    checkedAt,
	})
}

func (s *Service) markRefreshFailure(ctx context.Context, item *domain.Tunnel, err error) error {
	now := s.now().UTC()
	item.Status = domain.TunnelStatusDegraded
	item.LastRefreshError = err.Error()
	item.UpdatedAt = now
	_ = s.tunnels.Update(ctx, item)
	s.recordEvent(ctx, item.ID, "tunnel.refresh_failed", map[string]any{"error": err.Error()})
	s.logError("tunnel refresh failed", "tunnel_id", item.ID, "error", err)
	return err
}

func (s *Service) normalizeInput(name, groupID, listenHost, username, password string) (string, string, string, *singbox.ProxyAuth, error) {
	name = strings.TrimSpace(name)
	groupID = strings.TrimSpace(groupID)
	listenHost = strings.TrimSpace(listenHost)
	if listenHost == "" {
		listenHost = "127.0.0.1"
	}
	if name == "" || groupID == "" {
		return "", "", "", nil, ErrInvalidPayload
	}
	proxyAuth, err := validateProxyAuth(username, password)
	if err != nil {
		return "", "", "", nil, err
	}
	return name, groupID, listenHost, proxyAuth, nil
}

func validateProxyAuth(username, password string) (*singbox.ProxyAuth, error) {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" && password == "" {
		return nil, nil
	}
	if username == "" || password == "" {
		return nil, ErrInvalidPayload
	}
	return &singbox.ProxyAuth{
		Username: username,
		Password: password,
	}, nil
}

func (s *Service) encryptAuth(tunnelID string, auth *singbox.ProxyAuth) ([]byte, []byte, error) {
	if auth == nil {
		return nil, nil, nil
	}
	payload, err := json.Marshal(authPayload{
		Username: auth.Username,
		Password: auth.Password,
	})
	if err != nil {
		return nil, nil, err
	}
	return s.cipher.Encrypt(payload, []byte("tunnel:auth:"+tunnelID))
}

func (s *Service) decryptAuth(item *domain.Tunnel) (*singbox.ProxyAuth, error) {
	if len(item.AuthPasswordCiphertext) == 0 {
		return nil, nil
	}
	plaintext, err := s.cipher.Decrypt(item.AuthNonce, item.AuthPasswordCiphertext, []byte("tunnel:auth:"+item.ID))
	if err != nil {
		return nil, err
	}
	var payload authPayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return nil, err
	}
	if payload.Username == "" && payload.Password == "" {
		return nil, nil
	}
	return &singbox.ProxyAuth{
		Username: payload.Username,
		Password: payload.Password,
	}, nil
}

func (s *Service) decryptControllerSecret(item *domain.Tunnel) (string, error) {
	plaintext, err := s.cipher.Decrypt(item.ControllerSecretNonce, item.ControllerSecretCiphertext, []byte("tunnel:controller:"+item.ID))
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func (s *Service) releaseReservedPorts(pair singbox.PortPair) error {
	var errs []error
	errs = append(errs, s.portAllocator.Release(pair.ProxyPort))
	errs = append(errs, s.portAllocator.Release(pair.ControllerPort))
	return errors.Join(errs...)
}

func (s *Service) recordEvent(ctx context.Context, tunnelID, eventType string, detail map[string]any) {
	if detail == nil {
		detail = map[string]any{}
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		s.logError("marshal tunnel event failed", "tunnel_id", tunnelID, "event_type", eventType, "error", err)
		return
	}
	if err := s.tunnelEvents.Create(ctx, &domain.TunnelEvent{
		ID:         uuid.NewString(),
		TunnelID:   tunnelID,
		EventType:  eventType,
		DetailJSON: string(raw),
		CreatedAt:  s.now().UTC(),
	}); err != nil {
		s.logError("record tunnel event failed", "tunnel_id", tunnelID, "event_type", eventType, "error", err)
	}
}

func toView(item *domain.Tunnel) *View {
	return &View{
		ID:               item.ID,
		Name:             item.Name,
		GroupID:          item.GroupID,
		ListenHost:       item.ListenHost,
		ListenPort:       item.ListenPort,
		Status:           item.Status,
		CurrentNodeID:    item.CurrentNodeID,
		ControllerPort:   item.ControllerPort,
		RuntimeDir:       item.RuntimeDir,
		LastRefreshAt:    item.LastRefreshAt,
		LastRefreshError: item.LastRefreshError,
		HasAuth:          len(item.AuthPasswordCiphertext) > 0,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func outboundTag(nodeID string) string {
	return "node-" + nodeID
}

func (p *preparedRuntime) stopBackground() {
	if p == nil || p.cancelBackground == nil {
		return
	}
	p.cancelBackground()
	waitForProbeDrain(p.backgroundDone)
}

func waitForProbeDrain(done <-chan struct{}) {
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
	}
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy := append([]string(nil), left...)
	rightCopy := append([]string(nil), right...)
	slices.Sort(leftCopy)
	slices.Sort(rightCopy)
	for i := range leftCopy {
		if leftCopy[i] != rightCopy[i] {
			return false
		}
	}
	return true
}

func stringPtr(value string) *string {
	return &value
}

func (s *Service) runtimeLayoutFor(ctx context.Context, groupID, tunnelName string) (singbox.RuntimeLayout, error) {
	groupView, err := s.groups.Get(ctx, groupID)
	if err != nil {
		return singbox.RuntimeLayout{}, err
	}
	return singbox.NewRuntimeGroupLayout(s.runtimeRoot, groupView.Name, tunnelName), nil
}

func (s *Service) ensureUniqueTunnelName(ctx context.Context, groupID, name, excludeID string) error {
	item, err := s.tunnels.GetByGroupIDAndName(ctx, groupID, name)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if excludeID != "" && item.ID == excludeID {
		return nil
	}
	return ErrTunnelConflict
}

func runtimeLayoutFromRoot(root string) singbox.RuntimeLayout {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "tunnel"
	}
	return singbox.NewRuntimeLayoutFromRoot(filepath.Clean(root))
}

func (s *Service) logInfo(message string, args ...any) {
	if s.logger != nil {
		s.logger.Info(message, args...)
	}
}

func (s *Service) logError(message string, args ...any) {
	if s.logger != nil {
		s.logger.Error(message, args...)
	}
}

func formatWrappedError(prefix string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", prefix, err)
}
