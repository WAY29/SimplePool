package sqlite

import (
	"context"
	"database/sql"

	"github.com/WAY29/SimplePool/internal/domain"
)

type TunnelRepository struct {
	db *sql.DB
}

func (r *TunnelRepository) Create(ctx context.Context, tunnel *domain.Tunnel) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO tunnels(id, name, group_id, listen_host, listen_port, status, current_node_id, auth_username_ciphertext, auth_password_ciphertext, auth_nonce,
		 controller_port, controller_secret_ciphertext, controller_secret_nonce, runtime_dir, last_refresh_at, last_refresh_error, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tunnel.ID,
		tunnel.Name,
		tunnel.GroupID,
		tunnel.ListenHost,
		tunnel.ListenPort,
		tunnel.Status,
		nullableStringValue(tunnel.CurrentNodeID),
		tunnel.AuthUsernameCiphertext,
		tunnel.AuthPasswordCiphertext,
		tunnel.AuthNonce,
		tunnel.ControllerPort,
		tunnel.ControllerSecretCiphertext,
		tunnel.ControllerSecretNonce,
		tunnel.RuntimeDir,
		nullableTimeValue(tunnel.LastRefreshAt),
		tunnel.LastRefreshError,
		formatTime(tunnel.CreatedAt),
		formatTime(tunnel.UpdatedAt),
	)
	return err
}

func (r *TunnelRepository) Update(ctx context.Context, tunnel *domain.Tunnel) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE tunnels
		 SET name = ?, group_id = ?, listen_host = ?, listen_port = ?, status = ?, current_node_id = ?, auth_username_ciphertext = ?, auth_password_ciphertext = ?, auth_nonce = ?,
		     controller_port = ?, controller_secret_ciphertext = ?, controller_secret_nonce = ?, runtime_dir = ?, last_refresh_at = ?, last_refresh_error = ?, updated_at = ?
		 WHERE id = ?`,
		tunnel.Name,
		tunnel.GroupID,
		tunnel.ListenHost,
		tunnel.ListenPort,
		tunnel.Status,
		nullableStringValue(tunnel.CurrentNodeID),
		tunnel.AuthUsernameCiphertext,
		tunnel.AuthPasswordCiphertext,
		tunnel.AuthNonce,
		tunnel.ControllerPort,
		tunnel.ControllerSecretCiphertext,
		tunnel.ControllerSecretNonce,
		tunnel.RuntimeDir,
		nullableTimeValue(tunnel.LastRefreshAt),
		tunnel.LastRefreshError,
		formatTime(tunnel.UpdatedAt),
		tunnel.ID,
	)
	if err != nil {
		return err
	}

	return ensureRowsAffected(result)
}

func (r *TunnelRepository) GetByID(ctx context.Context, id string) (*domain.Tunnel, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, name, group_id, listen_host, listen_port, status, current_node_id, auth_username_ciphertext, auth_password_ciphertext, auth_nonce, controller_port, controller_secret_ciphertext, controller_secret_nonce, runtime_dir, last_refresh_at, last_refresh_error, created_at, updated_at FROM tunnels WHERE id = ?`, id)
	item, err := scanTunnel(row)
	if err != nil {
		return nil, translateNotFound(err)
	}
	return item, nil
}

func (r *TunnelRepository) List(ctx context.Context) ([]*domain.Tunnel, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, group_id, listen_host, listen_port, status, current_node_id, auth_username_ciphertext, auth_password_ciphertext, auth_nonce, controller_port, controller_secret_ciphertext, controller_secret_nonce, runtime_dir, last_refresh_at, last_refresh_error, created_at, updated_at FROM tunnels ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domain.Tunnel
	for rows.Next() {
		item, err := scanTunnel(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *TunnelRepository) DeleteByID(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM tunnels WHERE id = ?`, id)
	if err != nil {
		return err
	}

	return ensureRowsAffected(result)
}

type tunnelScanner interface {
	Scan(dest ...any) error
}

func scanTunnel(scanner tunnelScanner) (*domain.Tunnel, error) {
	var item domain.Tunnel
	var currentNodeID sql.NullString
	var lastRefreshAt sql.NullString
	var createdAt string
	var updatedAt string
	if err := scanner.Scan(
		&item.ID,
		&item.Name,
		&item.GroupID,
		&item.ListenHost,
		&item.ListenPort,
		&item.Status,
		&currentNodeID,
		&item.AuthUsernameCiphertext,
		&item.AuthPasswordCiphertext,
		&item.AuthNonce,
		&item.ControllerPort,
		&item.ControllerSecretCiphertext,
		&item.ControllerSecretNonce,
		&item.RuntimeDir,
		&lastRefreshAt,
		&item.LastRefreshError,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}

	if currentNodeID.Valid {
		item.CurrentNodeID = &currentNodeID.String
	}

	var err error
	item.LastRefreshAt, err = parseNullableTime(lastRefreshAt)
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
