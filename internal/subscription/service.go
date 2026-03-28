package subscription

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/WAY29/SimplePool/internal/domain"
	"github.com/WAY29/SimplePool/internal/node"
	"github.com/WAY29/SimplePool/internal/store"
	"github.com/google/uuid"
)

var (
	ErrFetchFailed     = errors.New("subscription: fetch failed")
	ErrDuplicateSource = errors.New("subscription: duplicate fetch fingerprint")
	ErrInvalidURL      = errors.New("subscription: invalid url")
)

const subscriptionUserAgent = "sing-box-windows/1.0 (sing-box; compatible; Windows NT 10.0)"

type Cipher interface {
	Encrypt(plaintext, aad []byte) ([]byte, []byte, error)
	Decrypt(nonce, ciphertext, aad []byte) ([]byte, error)
}

type Fetcher interface {
	Fetch(ctx context.Context, request FetchRequest) ([]byte, error)
}

type FetchRequest struct {
	URL string
}

type Options struct {
	SubscriptionSources store.SubscriptionSourceRepository
	Nodes               store.NodeRepository
	LatencySamples      store.LatencySampleRepository
	Cipher              Cipher
	Fetcher             Fetcher
	Prober              node.Prober
	Now                 func() time.Time
	ProbeCacheTTL       time.Duration
}

type Service struct {
	sources     store.SubscriptionSourceRepository
	nodes       store.NodeRepository
	cipher      Cipher
	fetcher     Fetcher
	now         func() time.Time
	nodeService *node.Service
}

type CreateInput struct {
	Name string
	URL  string
}

type UpdateInput struct {
	Name    string
	URL     string
	Enabled bool
}

type View struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	FetchFingerprint string     `json:"fetch_fingerprint"`
	Enabled          bool       `json:"enabled"`
	LastRefreshAt    *time.Time `json:"last_refresh_at,omitempty"`
	LastError        string     `json:"last_error"`
	HasURL           bool       `json:"has_url"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type RefreshResult struct {
	SourceID      string       `json:"source_id"`
	UpsertedNodes []*node.View `json:"upserted_nodes"`
	DeletedCount  int          `json:"deleted_count"`
}

func NewService(options Options) *Service {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		sources: options.SubscriptionSources,
		nodes:   options.Nodes,
		cipher:  options.Cipher,
		fetcher: options.Fetcher,
		now:     now,
		nodeService: node.NewService(node.Options{
			Nodes:          options.Nodes,
			LatencySamples: options.LatencySamples,
			Cipher:         options.Cipher,
			Prober:         options.Prober,
			Now:            now,
			ProbeCacheTTL:  options.ProbeCacheTTL,
		}),
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*View, error) {
	source, err := s.buildSource(uuid.NewString(), input.Name, input.URL, true)
	if err != nil {
		return nil, err
	}
	if err := s.ensureUniqueFetchFingerprint(ctx, source.FetchFingerprint, ""); err != nil {
		return nil, err
	}
	if err := s.sources.Create(ctx, source); err != nil {
		return nil, err
	}
	return toView(source), nil
}

func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (*View, error) {
	current, err := s.sources.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	source, err := s.buildSource(id, input.Name, input.URL, input.Enabled)
	if err != nil {
		return nil, err
	}
	source.CreatedAt = current.CreatedAt
	source.LastRefreshAt = current.LastRefreshAt
	source.LastError = current.LastError
	if err := s.ensureUniqueFetchFingerprint(ctx, source.FetchFingerprint, id); err != nil {
		return nil, err
	}
	if err := s.sources.Update(ctx, source); err != nil {
		return nil, err
	}
	return toView(source), nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	nodes, err := s.nodes.List(ctx)
	if err != nil {
		return err
	}
	for _, item := range nodes {
		if item.SubscriptionSourceID != nil && *item.SubscriptionSourceID == id {
			if err := s.nodes.DeleteByID(ctx, item.ID); err != nil {
				return err
			}
		}
	}
	return s.sources.DeleteByID(ctx, id)
}

func (s *Service) Get(ctx context.Context, id string) (*View, error) {
	source, err := s.sources.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toView(source), nil
}

func (s *Service) List(ctx context.Context) ([]*View, error) {
	sources, err := s.sources.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*View, 0, len(sources))
	for _, item := range sources {
		result = append(result, toView(item))
	}
	return result, nil
}

func (s *Service) Refresh(ctx context.Context, id string, force bool) (*RefreshResult, error) {
	source, err := s.sources.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	rawURL, err := s.cipher.Decrypt(source.URLNonce, source.URLCiphertext, []byte("subscription:url:"+source.ID))
	if err != nil {
		return nil, err
	}
	body, err := s.fetcher.Fetch(ctx, FetchRequest{URL: string(rawURL)})
	if err != nil {
		source.LastError = err.Error()
		source.UpdatedAt = s.now().UTC()
		_ = s.sources.Update(ctx, source)
		return nil, ErrFetchFailed
	}
	parsed, err := node.ParseImportPayload(string(body))
	if err != nil {
		source.LastError = err.Error()
		source.UpdatedAt = s.now().UTC()
		_ = s.sources.Update(ctx, source)
		return nil, err
	}

	result := &RefreshResult{
		SourceID:      source.ID,
		UpsertedNodes: make([]*node.View, 0, len(parsed)),
	}
	seen := make(map[string]struct{}, len(parsed))
	for _, item := range parsed {
		view, err := s.upsertNode(ctx, source.ID, item, force)
		if err != nil {
			source.LastError = err.Error()
			source.UpdatedAt = s.now().UTC()
			_ = s.sources.Update(ctx, source)
			return nil, err
		}
		seen[node.ComputeSourceNodeKey(source.ID, item.DedupeFingerprint)] = struct{}{}
		result.UpsertedNodes = append(result.UpsertedNodes, view)
	}

	existingNodes, err := s.nodes.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range existingNodes {
		if item.SubscriptionSourceID == nil || *item.SubscriptionSourceID != source.ID {
			continue
		}
		if _, ok := seen[item.SourceNodeKey]; ok {
			continue
		}
		if err := s.nodes.DeleteByID(ctx, item.ID); err != nil {
			return nil, err
		}
		result.DeletedCount++
	}

	now := s.now().UTC()
	source.LastRefreshAt = &now
	source.LastError = ""
	source.UpdatedAt = now
	if err := s.sources.Update(ctx, source); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) upsertNode(ctx context.Context, sourceID string, item node.ImportedNode, _ bool) (*node.View, error) {
	sourceNodeKey := node.ComputeSourceNodeKey(sourceID, item.DedupeFingerprint)
	existing, err := s.nodes.GetBySourceNodeKey(ctx, sourceID, sourceNodeKey)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	sourceRef := sourceID
	if existing != nil {
		view, err := s.nodeService.UpsertImported(ctx, node.UpsertImportedInput{
			ExistingID:           existing.ID,
			SourceKind:           domain.NodeSourceSubscription,
			SubscriptionSourceID: &sourceRef,
			SourceNodeKey:        sourceNodeKey,
			Imported:             item,
		})
		if err != nil {
			return nil, err
		}
		return view, nil
	}

	view, err := s.nodeService.UpsertImported(ctx, node.UpsertImportedInput{
		SourceKind:           domain.NodeSourceSubscription,
		SubscriptionSourceID: &sourceRef,
		SourceNodeKey:        sourceNodeKey,
		Imported:             item,
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

func (s *Service) buildSource(id, name, rawURL string, enabled bool) (*domain.SubscriptionSource, error) {
	if strings.TrimSpace(name) == "" {
		return nil, ErrInvalidURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, ErrInvalidURL
	}
	normalizedURL := parsed.String()
	fingerprint := computeFetchFingerprint(parsed)
	ciphertext, nonce, err := s.cipher.Encrypt([]byte(normalizedURL), []byte("subscription:url:"+id))
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	return &domain.SubscriptionSource{
		ID:               id,
		Name:             name,
		FetchFingerprint: fingerprint,
		URLCiphertext:    ciphertext,
		URLNonce:         nonce,
		Enabled:          enabled,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

func (s *Service) ensureUniqueFetchFingerprint(ctx context.Context, fingerprint, currentID string) error {
	existing, err := s.sources.GetByFetchFingerprint(ctx, fingerprint)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}
	if existing.ID != currentID {
		return ErrDuplicateSource
	}
	return nil
}

func computeFetchFingerprint(value *url.URL) string {
	sum := sha256.Sum256([]byte(strings.ToLower(fmt.Sprintf("%s|%s|%s|%s|%s", value.Scheme, value.Host, value.Path, value.RawQuery, value.User.String()))))
	return hex.EncodeToString(sum[:])
}

func toView(source *domain.SubscriptionSource) *View {
	return &View{
		ID:               source.ID,
		Name:             source.Name,
		FetchFingerprint: source.FetchFingerprint,
		Enabled:          source.Enabled,
		LastRefreshAt:    source.LastRefreshAt,
		LastError:        source.LastError,
		HasURL:           len(source.URLCiphertext) > 0,
		CreatedAt:        source.CreatedAt,
		UpdatedAt:        source.UpdatedAt,
	}
}

type HTTPFetcher struct {
	client *http.Client
}

func NewHTTPFetcher(timeout time.Duration) *HTTPFetcher {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &HTTPFetcher{
		client: &http.Client{Timeout: timeout},
	}
}

func (f *HTTPFetcher) Fetch(ctx context.Context, request FetchRequest) ([]byte, error) {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, request.URL, nil)
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("User-Agent", subscriptionUserAgent)
	response, err := f.client.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: http status %d", ErrFetchFailed, response.StatusCode)
	}
	return io.ReadAll(response.Body)
}
