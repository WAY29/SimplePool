package sqlite

import (
	"context"
	"database/sql"

	"github.com/WAY29/SimplePool/internal/domain"
)

type AdminUserRepository struct {
	db *sql.DB
}

func (r *AdminUserRepository) Create(ctx context.Context, user *domain.AdminUser) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO admin_users(id, username, password_hash, created_at, updated_at) VALUES(?, ?, ?, ?, ?)`,
		user.ID,
		user.Username,
		user.PasswordHash,
		formatTime(user.CreatedAt),
		formatTime(user.UpdatedAt),
	)
	return err
}

func (r *AdminUserRepository) Update(ctx context.Context, user *domain.AdminUser) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE admin_users SET username = ?, password_hash = ?, updated_at = ? WHERE id = ?`,
		user.Username,
		user.PasswordHash,
		formatTime(user.UpdatedAt),
		user.ID,
	)
	if err != nil {
		return err
	}

	return ensureRowsAffected(result)
}

func (r *AdminUserRepository) GetByID(ctx context.Context, id string) (*domain.AdminUser, error) {
	return r.getOne(ctx, `SELECT id, username, password_hash, created_at, updated_at FROM admin_users WHERE id = ?`, id)
}

func (r *AdminUserRepository) GetByUsername(ctx context.Context, username string) (*domain.AdminUser, error) {
	return r.getOne(ctx, `SELECT id, username, password_hash, created_at, updated_at FROM admin_users WHERE username = ?`, username)
}

func (r *AdminUserRepository) List(ctx context.Context) ([]*domain.AdminUser, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, username, password_hash, created_at, updated_at FROM admin_users ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.AdminUser
	for rows.Next() {
		user, err := scanAdminUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, rows.Err()
}

func (r *AdminUserRepository) getOne(ctx context.Context, query string, args ...any) (*domain.AdminUser, error) {
	row := r.db.QueryRowContext(ctx, query, args...)
	user, err := scanAdminUser(row)
	if err != nil {
		return nil, translateNotFound(err)
	}

	return user, nil
}

type adminUserScanner interface {
	Scan(dest ...any) error
}

func scanAdminUser(scanner adminUserScanner) (*domain.AdminUser, error) {
	var user domain.AdminUser
	var createdAt string
	var updatedAt string
	if err := scanner.Scan(&user.ID, &user.Username, &user.PasswordHash, &createdAt, &updatedAt); err != nil {
		return nil, err
	}

	var err error
	user.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	user.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return nil, err
	}

	return &user, nil
}
