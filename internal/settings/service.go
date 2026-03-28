package settings

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/WAY29/SimplePool/internal/domain"
	"github.com/WAY29/SimplePool/internal/store"
)

const (
	ProbeTestURLKey      = "probe_test_url"
	DefaultProbeTestURL  = "http://cp.cloudflare.com/generate_204"
	GstaticProbeTestURL  = "https://www.gstatic.com/generate_204"
)

var (
	ErrInvalidProbeTestURL = errors.New("settings: invalid probe test url")
	PresetProbeTestURLs    = []string{
		DefaultProbeTestURL,
		GstaticProbeTestURL,
	}
)

type Options struct {
	AppSettings store.AppSettingRepository
	Now         func() time.Time
}

type Service struct {
	appSettings store.AppSettingRepository
	now         func() time.Time
}

type ProbeConfigView struct {
	TestURL        string     `json:"test_url"`
	DefaultTestURL string     `json:"default_test_url"`
	PresetURLs     []string   `json:"preset_urls"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
}

func NewService(options Options) *Service {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		appSettings: options.AppSettings,
		now:         now,
	}
}

func (s *Service) GetProbeConfig(ctx context.Context) (*ProbeConfigView, error) {
	if s == nil || s.appSettings == nil {
		return defaultProbeConfigView(nil), nil
	}

	item, err := s.appSettings.GetByKey(ctx, ProbeTestURLKey)
	switch {
	case err == nil:
		updatedAt := item.UpdatedAt
		return defaultProbeConfigView(&updatedAt, NormalizeProbeTestURL(item.Value)), nil
	case errors.Is(err, store.ErrNotFound):
		return defaultProbeConfigView(nil), nil
	default:
		return nil, err
	}
}

func (s *Service) SetProbeTestURL(ctx context.Context, raw string) (*ProbeConfigView, error) {
	if s == nil || s.appSettings == nil {
		return nil, store.ErrNotFound
	}
	value := strings.TrimSpace(raw)
	if err := ValidateProbeTestURL(value); err != nil {
		return nil, err
	}

	now := s.now().UTC()
	if err := s.appSettings.Upsert(ctx, &domain.AppSetting{
		Key:       ProbeTestURLKey,
		Value:     value,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		return nil, err
	}

	return defaultProbeConfigView(&now, value), nil
}

func NormalizeProbeTestURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DefaultProbeTestURL
	}
	return trimmed
}

func ValidateProbeTestURL(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%w: test url is required", ErrInvalidProbeTestURL)
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return fmt.Errorf("%w: test url must be a valid absolute URL", ErrInvalidProbeTestURL)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: test url must use http or https", ErrInvalidProbeTestURL)
	}

	return nil
}

func defaultProbeConfigView(updatedAt *time.Time, overrides ...string) *ProbeConfigView {
	testURL := DefaultProbeTestURL
	if len(overrides) > 0 && overrides[0] != "" {
		testURL = overrides[0]
	}
	return &ProbeConfigView{
		TestURL:        testURL,
		DefaultTestURL: DefaultProbeTestURL,
		PresetURLs:     append([]string(nil), PresetProbeTestURLs...),
		UpdatedAt:      updatedAt,
	}
}
