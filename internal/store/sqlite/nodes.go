package sqlite

import (
	"context"
	"database/sql"

	"github.com/WAY29/SimplePool/internal/domain"
)

type NodeRepository struct {
	db *sql.DB
}

func (r *NodeRepository) Create(ctx context.Context, node *domain.Node) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO nodes(id, name, source_node_key, dedupe_fingerprint, source_kind, subscription_source_id, protocol, server, server_port,
		 credential_ciphertext, credential_nonce, transport_json, tls_json, raw_payload_json, enabled, last_latency_ms, last_status, last_checked_at, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		node.ID,
		node.Name,
		node.SourceNodeKey,
		node.DedupeFingerprint,
		node.SourceKind,
		nullableStringValue(node.SubscriptionSourceID),
		node.Protocol,
		node.Server,
		node.ServerPort,
		node.CredentialCiphertext,
		node.CredentialNonce,
		node.TransportJSON,
		node.TLSJSON,
		node.RawPayloadJSON,
		boolToInt(node.Enabled),
		nullableInt64Value(node.LastLatencyMS),
		node.LastStatus,
		nullableTimeValue(node.LastCheckedAt),
		formatTime(node.CreatedAt),
		formatTime(node.UpdatedAt),
	)
	return err
}

func (r *NodeRepository) Update(ctx context.Context, node *domain.Node) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE nodes
		 SET name = ?, source_node_key = ?, dedupe_fingerprint = ?, source_kind = ?, subscription_source_id = ?, protocol = ?, server = ?, server_port = ?,
		     credential_ciphertext = ?, credential_nonce = ?, transport_json = ?, tls_json = ?, raw_payload_json = ?, enabled = ?, last_latency_ms = ?, last_status = ?,
		     last_checked_at = ?, updated_at = ?
		 WHERE id = ?`,
		node.Name,
		node.SourceNodeKey,
		node.DedupeFingerprint,
		node.SourceKind,
		nullableStringValue(node.SubscriptionSourceID),
		node.Protocol,
		node.Server,
		node.ServerPort,
		node.CredentialCiphertext,
		node.CredentialNonce,
		node.TransportJSON,
		node.TLSJSON,
		node.RawPayloadJSON,
		boolToInt(node.Enabled),
		nullableInt64Value(node.LastLatencyMS),
		node.LastStatus,
		nullableTimeValue(node.LastCheckedAt),
		formatTime(node.UpdatedAt),
		node.ID,
	)
	if err != nil {
		return err
	}

	return ensureRowsAffected(result)
}

func (r *NodeRepository) GetByID(ctx context.Context, id string) (*domain.Node, error) {
	return r.getOne(ctx, `SELECT id, name, source_node_key, dedupe_fingerprint, source_kind, subscription_source_id, protocol, server, server_port, credential_ciphertext, credential_nonce, transport_json, tls_json, raw_payload_json, enabled, last_latency_ms, last_status, last_checked_at, created_at, updated_at FROM nodes WHERE id = ?`, id)
}

func (r *NodeRepository) GetBySourceNodeKey(ctx context.Context, sourceID, sourceNodeKey string) (*domain.Node, error) {
	return r.getOne(ctx, `SELECT id, name, source_node_key, dedupe_fingerprint, source_kind, subscription_source_id, protocol, server, server_port, credential_ciphertext, credential_nonce, transport_json, tls_json, raw_payload_json, enabled, last_latency_ms, last_status, last_checked_at, created_at, updated_at FROM nodes WHERE subscription_source_id = ? AND source_node_key = ?`, sourceID, sourceNodeKey)
}

func (r *NodeRepository) List(ctx context.Context) ([]*domain.Node, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, source_node_key, dedupe_fingerprint, source_kind, subscription_source_id, protocol, server, server_port, credential_ciphertext, credential_nonce, transport_json, tls_json, raw_payload_json, enabled, last_latency_ms, last_status, last_checked_at, created_at, updated_at FROM nodes ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domain.Node
	for rows.Next() {
		item, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *NodeRepository) DeleteByID(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM nodes WHERE id = ?`, id)
	if err != nil {
		return err
	}

	return ensureRowsAffected(result)
}

func (r *NodeRepository) getOne(ctx context.Context, query string, args ...any) (*domain.Node, error) {
	row := r.db.QueryRowContext(ctx, query, args...)
	item, err := scanNode(row)
	if err != nil {
		return nil, translateNotFound(err)
	}

	return item, nil
}

type nodeScanner interface {
	Scan(dest ...any) error
}

func scanNode(scanner nodeScanner) (*domain.Node, error) {
	var item domain.Node
	var subscriptionSourceID sql.NullString
	var enabled int
	var lastLatency sql.NullInt64
	var lastChecked sql.NullString
	var createdAt string
	var updatedAt string
	if err := scanner.Scan(
		&item.ID,
		&item.Name,
		&item.SourceNodeKey,
		&item.DedupeFingerprint,
		&item.SourceKind,
		&subscriptionSourceID,
		&item.Protocol,
		&item.Server,
		&item.ServerPort,
		&item.CredentialCiphertext,
		&item.CredentialNonce,
		&item.TransportJSON,
		&item.TLSJSON,
		&item.RawPayloadJSON,
		&enabled,
		&lastLatency,
		&item.LastStatus,
		&lastChecked,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}

	if subscriptionSourceID.Valid {
		item.SubscriptionSourceID = &subscriptionSourceID.String
	}
	item.Enabled = enabled == 1
	if lastLatency.Valid {
		item.LastLatencyMS = &lastLatency.Int64
	}

	var err error
	item.LastCheckedAt, err = parseNullableTime(lastChecked)
	if err != nil {
		return nil, err
	}
	item.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	item.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return nil, err
	}

	return &item, nil
}
