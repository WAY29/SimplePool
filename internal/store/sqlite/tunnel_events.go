package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/WAY29/SimplePool/internal/domain"
)

type TunnelEventRepository struct {
	db *sql.DB
}

func (r *TunnelEventRepository) Create(ctx context.Context, event *domain.TunnelEvent) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO tunnel_events(id, tunnel_id, event_type, detail_json, created_at) VALUES(?, ?, ?, ?, ?)`,
		event.ID,
		event.TunnelID,
		event.EventType,
		event.DetailJSON,
		formatTime(event.CreatedAt),
	)
	return err
}

func (r *TunnelEventRepository) ListByTunnelID(ctx context.Context, tunnelID string, limit int) ([]*domain.TunnelEvent, error) {
	if limit <= 0 {
		limit = 50
	}

	query := fmt.Sprintf(`SELECT id, tunnel_id, event_type, detail_json, created_at FROM tunnel_events WHERE tunnel_id = ? ORDER BY created_at DESC LIMIT %d`, limit)
	rows, err := r.db.QueryContext(ctx, query, tunnelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domain.TunnelEvent
	for rows.Next() {
		item, err := scanTunnelEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

type tunnelEventScanner interface {
	Scan(dest ...any) error
}

func scanTunnelEvent(scanner tunnelEventScanner) (*domain.TunnelEvent, error) {
	var item domain.TunnelEvent
	var createdAt string
	if err := scanner.Scan(&item.ID, &item.TunnelID, &item.EventType, &item.DetailJSON, &createdAt); err != nil {
		return nil, err
	}

	var err error
	item.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, err
	}

	return &item, nil
}
