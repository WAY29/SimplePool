package group

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/WAY29/SimplePool/internal/domain"
	"github.com/WAY29/SimplePool/internal/store"
	"github.com/google/uuid"
)

var (
	ErrInvalidPayload = errors.New("group: invalid payload")
	ErrInvalidFilter  = errors.New("group: invalid filter regex")
)

type Options struct {
	Groups store.GroupRepository
	Nodes  store.NodeRepository
	Now    func() time.Time
}

type Service struct {
	groups store.GroupRepository
	nodes  store.NodeRepository
	now    func() time.Time
}

type CreateInput struct {
	Name        string
	FilterRegex string
	Description string
}

type UpdateInput struct {
	Name        string
	FilterRegex string
	Description string
}

type PreviewInput struct {
	FilterRegex string
}

type View struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	FilterRegex string    `json:"filter_regex"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type MemberView struct {
	ID                   string     `json:"id"`
	Name                 string     `json:"name"`
	SourceKind           string     `json:"source_kind"`
	SubscriptionSourceID *string    `json:"subscription_source_id,omitempty"`
	Protocol             string     `json:"protocol"`
	Server               string     `json:"server"`
	ServerPort           int        `json:"server_port"`
	Enabled              bool       `json:"enabled"`
	LastLatencyMS        *int64     `json:"last_latency_ms,omitempty"`
	LastStatus           string     `json:"last_status"`
	LastCheckedAt        *time.Time `json:"last_checked_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

func NewService(options Options) *Service {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		groups: options.Groups,
		nodes:  options.Nodes,
		now:    now,
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*View, error) {
	entity, err := s.buildGroup(uuid.NewString(), input.Name, input.FilterRegex, input.Description)
	if err != nil {
		return nil, err
	}
	if err := s.groups.Create(ctx, entity); err != nil {
		return nil, err
	}
	return toView(entity), nil
}

func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (*View, error) {
	current, err := s.groups.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	entity, err := s.buildGroup(id, input.Name, input.FilterRegex, input.Description)
	if err != nil {
		return nil, err
	}
	entity.CreatedAt = current.CreatedAt
	if err := s.groups.Update(ctx, entity); err != nil {
		return nil, err
	}
	return toView(entity), nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.groups.DeleteByID(ctx, id)
}

func (s *Service) Get(ctx context.Context, id string) (*View, error) {
	entity, err := s.groups.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toView(entity), nil
}

func (s *Service) List(ctx context.Context) ([]*View, error) {
	items, err := s.groups.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*View, 0, len(items))
	for _, item := range items {
		result = append(result, toView(item))
	}
	return result, nil
}

func (s *Service) PreviewMembers(ctx context.Context, input PreviewInput) ([]*MemberView, error) {
	filter, err := compileFilter(input.FilterRegex)
	if err != nil {
		return nil, err
	}
	return s.listMembersByFilter(ctx, filter)
}

func (s *Service) ListMembers(ctx context.Context, id string) ([]*MemberView, error) {
	entity, err := s.groups.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	filter, err := compileFilter(entity.FilterRegex)
	if err != nil {
		return nil, err
	}
	return s.listMembersByFilter(ctx, filter)
}

func (s *Service) listMembersByFilter(ctx context.Context, filter *regexp.Regexp) ([]*MemberView, error) {
	nodes, err := s.nodes.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*MemberView, 0, len(nodes))
	for _, item := range nodes {
		if filter.MatchString(item.Name) {
			result = append(result, toMemberView(item))
		}
	}
	return result, nil
}

func (s *Service) buildGroup(id, name, filterRegex, description string) (*domain.Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalidPayload
	}
	if _, err := compileFilter(filterRegex); err != nil {
		return nil, err
	}
	now := s.now().UTC()
	return &domain.Group{
		ID:          id,
		Name:        name,
		FilterRegex: filterRegex,
		Description: strings.TrimSpace(description),
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func compileFilter(filterRegex string) (*regexp.Regexp, error) {
	filter, err := regexp.Compile(filterRegex)
	if err != nil {
		return nil, ErrInvalidFilter
	}
	return filter, nil
}

func toView(group *domain.Group) *View {
	return &View{
		ID:          group.ID,
		Name:        group.Name,
		FilterRegex: group.FilterRegex,
		Description: group.Description,
		CreatedAt:   group.CreatedAt,
		UpdatedAt:   group.UpdatedAt,
	}
}

func toMemberView(item *domain.Node) *MemberView {
	return &MemberView{
		ID:                   item.ID,
		Name:                 item.Name,
		SourceKind:           item.SourceKind,
		SubscriptionSourceID: item.SubscriptionSourceID,
		Protocol:             item.Protocol,
		Server:               item.Server,
		ServerPort:           item.ServerPort,
		Enabled:              item.Enabled,
		LastLatencyMS:        item.LastLatencyMS,
		LastStatus:           item.LastStatus,
		LastCheckedAt:        item.LastCheckedAt,
		CreatedAt:            item.CreatedAt,
		UpdatedAt:            item.UpdatedAt,
	}
}
