package group_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/WAY29/SimplePool/internal/domain"
	"github.com/WAY29/SimplePool/internal/group"
	"github.com/WAY29/SimplePool/internal/store/sqlite"
)

func TestGroupServiceCRUDAndMembers(t *testing.T) {
	ctx := context.Background()
	service, repos := newGroupService(t)
	seedGroupNodes(t, ctx, repos)

	created, err := service.Create(ctx, group.CreateInput{
		Name:        "亚洲组",
		FilterRegex: "^(HK|JP)-",
		Description: "测试组",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := service.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != "亚洲组" {
		t.Fatalf("Name = %q, want 亚洲组", got.Name)
	}

	members, err := service.ListMembers(ctx, created.ID)
	if err != nil {
		t.Fatalf("ListMembers() error = %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("len(ListMembers()) = %d, want 2", len(members))
	}
	if members[1].Enabled {
		t.Fatalf("成员过滤了禁用节点之外的语义错误，want disabled member included")
	}

	updated, err := service.Update(ctx, created.ID, group.UpdateInput{
		Name:        "美区组",
		FilterRegex: "^US-",
		Description: "updated",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != "美区组" || updated.FilterRegex != "^US-" {
		t.Fatalf("Update() = %+v, want renamed US group", updated)
	}

	list, err := service.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(List()) = %d, want 1", len(list))
	}

	members, err = service.ListMembers(ctx, created.ID)
	if err != nil {
		t.Fatalf("ListMembers() after update error = %v", err)
	}
	if len(members) != 1 || members[0].Name != "US-A" {
		t.Fatalf("ListMembers() after update = %+v, want only US-A", members)
	}

	if err := service.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := service.Get(ctx, created.ID); err == nil {
		t.Fatal("Get() error = nil, want not found")
	}
}

func TestGroupServicePreviewRejectsInvalidRegexAndAllowsEmptyResult(t *testing.T) {
	ctx := context.Background()
	service, repos := newGroupService(t)
	seedGroupNodes(t, ctx, repos)

	if _, err := service.PreviewMembers(ctx, group.PreviewInput{FilterRegex: "["}); err == nil {
		t.Fatal("PreviewMembers() error = nil, want invalid regex")
	}

	result, err := service.PreviewMembers(ctx, group.PreviewInput{FilterRegex: "^SG-"})
	if err != nil {
		t.Fatalf("PreviewMembers() empty error = %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("len(PreviewMembers()) = %d, want 0", len(result))
	}
}

func newGroupService(t *testing.T) (*group.Service, *sqlite.Repositories) {
	t.Helper()

	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "group.db"))
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
	service := group.NewService(group.Options{
		Groups: repos.Groups,
		Nodes:  repos.Nodes,
		Now: func() time.Time {
			return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
		},
	})
	return service, repos
}

func seedGroupNodes(t *testing.T, ctx context.Context, repos *sqlite.Repositories) {
	t.Helper()

	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	nodes := []*domain.Node{
		{
			ID:                "node-hk",
			Name:              "HK-A",
			DedupeFingerprint: "hk-a",
			SourceKind:        domain.NodeSourceManual,
			Protocol:          "vmess",
			Server:            "1.1.1.1",
			ServerPort:        443,
			Enabled:           true,
			LastStatus:        domain.NodeStatusHealthy,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                "node-jp",
			Name:              "JP-A",
			DedupeFingerprint: "jp-a",
			SourceKind:        domain.NodeSourceManual,
			Protocol:          "trojan",
			Server:            "2.2.2.2",
			ServerPort:        443,
			Enabled:           false,
			LastStatus:        domain.NodeStatusUnknown,
			CreatedAt:         now.Add(time.Minute),
			UpdatedAt:         now.Add(time.Minute),
		},
		{
			ID:                "node-us",
			Name:              "US-A",
			DedupeFingerprint: "us-a",
			SourceKind:        domain.NodeSourceManual,
			Protocol:          "vless",
			Server:            "3.3.3.3",
			ServerPort:        8443,
			Enabled:           true,
			LastStatus:        domain.NodeStatusHealthy,
			CreatedAt:         now.Add(2 * time.Minute),
			UpdatedAt:         now.Add(2 * time.Minute),
		},
	}

	for _, item := range nodes {
		item.CredentialCiphertext = []byte("cipher")
		item.CredentialNonce = []byte("nonce")
		if err := repos.Nodes.Create(ctx, item); err != nil {
			t.Fatalf("Nodes.Create(%s) error = %v", item.ID, err)
		}
	}
}
