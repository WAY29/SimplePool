package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/WAY29/SimplePool/internal/domain"
)

type SessionRepository struct {
	db *sql.DB
}

func (r *SessionRepository) Create(ctx context.Context, session *domain.Session) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO sessions(id, user_id, token_hash, expires_at, created_at, last_seen_at) VALUES(?, ?, ?, ?, ?, ?)`,
		session.ID,
		session.UserID,
		session.TokenHash,
		formatTime(session.ExpiresAt),
		formatTime(session.CreatedAt),
		formatTime(session.LastSeenAt),
	)
	return err
}

func (r *SessionRepository) Update(ctx context.Context, session *domain.Session) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE sessions SET user_id = ?, token_hash = ?, expires_at = ?, last_seen_at = ? WHERE id = ?`,
		session.UserID,
		session.TokenHash,
		formatTime(session.ExpiresAt),
		formatTime(session.LastSeenAt),
		session.ID,
	)
	if err != nil {
		return err
	}

	return ensureRowsAffected(result)
}

func (r *SessionRepository) GetByID(ctx context.Context, id string) (*domain.Session, error) {
	return r.getOne(ctx, `SELECT id, user_id, token_hash, expires_at, created_at, last_seen_at FROM sessions WHERE id = ?`, id)
}

func (r *SessionRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*domain.Session, error) {
	return r.getOne(ctx, `SELECT id, user_id, token_hash, expires_at, created_at, last_seen_at FROM sessions WHERE token_hash = ?`, tokenHash)
}

func (r *SessionRepository) ListByUserID(ctx context.Context, userID string) ([]*domain.Session, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, user_id, token_hash, expires_at, created_at, last_seen_at FROM sessions WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*domain.Session
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

func (r *SessionRepository) DeleteByID(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return err
	}

	return ensureRowsAffected(result)
}

func (r *SessionRepository) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, formatTime(before))
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

func (r *SessionRepository) getOne(ctx context.Context, query string, args ...any) (*domain.Session, error) {
	row := r.db.QueryRowContext(ctx, query, args...)
	session, err := scanSession(row)
	if err != nil {
		return nil, translateNotFound(err)
	}

	return session, nil
}

type sessionScanner interface {
	Scan(dest ...any) error
}

func scanSession(scanner sessionScanner) (*domain.Session, error) {
	var session domain.Session
	var expiresAt string
	var createdAt string
	var lastSeenAt string
	if err := scanner.Scan(&session.ID, &session.UserID, &session.TokenHash, &expiresAt, &createdAt, &lastSeenAt); err != nil {
		return nil, err
	}

	var err error
	session.ExpiresAt, err = parseTime(expiresAt)
	if err != nil {
		return nil, err
	}
	session.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	session.LastSeenAt, err = parseTime(lastSeenAt)
	if err != nil {
		return nil, err
	}

	return &session, nil
}
