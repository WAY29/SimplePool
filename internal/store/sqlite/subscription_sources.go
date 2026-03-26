package sqlite

import (
	"context"
	"database/sql"

	"github.com/WAY29/SimplePool/internal/domain"
)

type SubscriptionSourceRepository struct {
	db *sql.DB
}

func (r *SubscriptionSourceRepository) Create(ctx context.Context, source *domain.SubscriptionSource) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO subscription_sources(id, name, fetch_fingerprint, url_ciphertext, url_nonce, enabled, last_refresh_at, last_error, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		source.ID,
		source.Name,
		source.FetchFingerprint,
		source.URLCiphertext,
		source.URLNonce,
		boolToInt(source.Enabled),
		nullableTimeValue(source.LastRefreshAt),
		source.LastError,
		formatTime(source.CreatedAt),
		formatTime(source.UpdatedAt),
	)
	return err
}

func (r *SubscriptionSourceRepository) Update(ctx context.Context, source *domain.SubscriptionSource) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE subscription_sources
		 SET name = ?, fetch_fingerprint = ?, url_ciphertext = ?, url_nonce = ?, enabled = ?, last_refresh_at = ?, last_error = ?, updated_at = ?
		 WHERE id = ?`,
		source.Name,
		source.FetchFingerprint,
		source.URLCiphertext,
		source.URLNonce,
		boolToInt(source.Enabled),
		nullableTimeValue(source.LastRefreshAt),
		source.LastError,
		formatTime(source.UpdatedAt),
		source.ID,
	)
	if err != nil {
		return err
	}

	return ensureRowsAffected(result)
}

func (r *SubscriptionSourceRepository) GetByID(ctx context.Context, id string) (*domain.SubscriptionSource, error) {
	return r.getOne(ctx, `SELECT id, name, fetch_fingerprint, url_ciphertext, url_nonce, enabled, last_refresh_at, last_error, created_at, updated_at FROM subscription_sources WHERE id = ?`, id)
}

func (r *SubscriptionSourceRepository) GetByFetchFingerprint(ctx context.Context, fingerprint string) (*domain.SubscriptionSource, error) {
	return r.getOne(ctx, `SELECT id, name, fetch_fingerprint, url_ciphertext, url_nonce, enabled, last_refresh_at, last_error, created_at, updated_at FROM subscription_sources WHERE fetch_fingerprint = ?`, fingerprint)
}

func (r *SubscriptionSourceRepository) List(ctx context.Context) ([]*domain.SubscriptionSource, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, fetch_fingerprint, url_ciphertext, url_nonce, enabled, last_refresh_at, last_error, created_at, updated_at FROM subscription_sources ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domain.SubscriptionSource
	for rows.Next() {
		item, err := scanSubscriptionSource(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *SubscriptionSourceRepository) DeleteByID(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM subscription_sources WHERE id = ?`, id)
	if err != nil {
		return err
	}

	return ensureRowsAffected(result)
}

func (r *SubscriptionSourceRepository) getOne(ctx context.Context, query string, args ...any) (*domain.SubscriptionSource, error) {
	row := r.db.QueryRowContext(ctx, query, args...)
	item, err := scanSubscriptionSource(row)
	if err != nil {
		return nil, translateNotFound(err)
	}

	return item, nil
}

type subscriptionSourceScanner interface {
	Scan(dest ...any) error
}

func scanSubscriptionSource(scanner subscriptionSourceScanner) (*domain.SubscriptionSource, error) {
	var item domain.SubscriptionSource
	var enabled int
	var lastRefresh sql.NullString
	var createdAt string
	var updatedAt string
	if err := scanner.Scan(
		&item.ID,
		&item.Name,
		&item.FetchFingerprint,
		&item.URLCiphertext,
		&item.URLNonce,
		&enabled,
		&lastRefresh,
		&item.LastError,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}

	item.Enabled = enabled == 1
	var err error
	item.LastRefreshAt, err = parseNullableTime(lastRefresh)
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
