package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/WAY29/SimplePool/internal/domain"
)

type LatencySampleRepository struct {
	db *sql.DB
}

func (r *LatencySampleRepository) Create(ctx context.Context, sample *domain.LatencySample) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO latency_samples(id, node_id, tunnel_id, test_url, latency_ms, success, error_message, created_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		sample.ID,
		sample.NodeID,
		nullableStringValue(sample.TunnelID),
		sample.TestURL,
		nullableInt64Value(sample.LatencyMS),
		boolToInt(sample.Success),
		sample.ErrorMessage,
		formatTime(sample.CreatedAt),
	)
	return err
}

func (r *LatencySampleRepository) ListByNodeID(ctx context.Context, nodeID string, limit int) ([]*domain.LatencySample, error) {
	return r.list(ctx, `WHERE node_id = ?`, nodeID, limit)
}

func (r *LatencySampleRepository) ListByTunnelID(ctx context.Context, tunnelID string, limit int) ([]*domain.LatencySample, error) {
	return r.list(ctx, `WHERE tunnel_id = ?`, tunnelID, limit)
}

func (r *LatencySampleRepository) list(ctx context.Context, clause string, arg string, limit int) ([]*domain.LatencySample, error) {
	if limit <= 0 {
		limit = 50
	}

	query := fmt.Sprintf(`SELECT id, node_id, tunnel_id, test_url, latency_ms, success, error_message, created_at FROM latency_samples %s ORDER BY created_at DESC LIMIT %d`, clause, limit)
	rows, err := r.db.QueryContext(ctx, query, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domain.LatencySample
	for rows.Next() {
		item, err := scanLatencySample(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

type latencySampleScanner interface {
	Scan(dest ...any) error
}

func scanLatencySample(scanner latencySampleScanner) (*domain.LatencySample, error) {
	var item domain.LatencySample
	var tunnelID sql.NullString
	var latency sql.NullInt64
	var success int
	var createdAt string
	if err := scanner.Scan(&item.ID, &item.NodeID, &tunnelID, &item.TestURL, &latency, &success, &item.ErrorMessage, &createdAt); err != nil {
		return nil, err
	}

	if tunnelID.Valid {
		item.TunnelID = &tunnelID.String
	}
	if latency.Valid {
		item.LatencyMS = &latency.Int64
	}
	item.Success = success == 1

	var err error
	item.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, err
	}

	return &item, nil
}
