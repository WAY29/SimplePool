package sqlite

import (
	"context"
	"database/sql"

	"github.com/WAY29/SimplePool/internal/domain"
)

type GroupRepository struct {
	db *sql.DB
}

func (r *GroupRepository) Create(ctx context.Context, group *domain.Group) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO groups(id, name, filter_regex, description, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?)`,
		group.ID,
		group.Name,
		group.FilterRegex,
		group.Description,
		formatTime(group.CreatedAt),
		formatTime(group.UpdatedAt),
	)
	return err
}

func (r *GroupRepository) Update(ctx context.Context, group *domain.Group) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE groups SET name = ?, filter_regex = ?, description = ?, updated_at = ? WHERE id = ?`,
		group.Name,
		group.FilterRegex,
		group.Description,
		formatTime(group.UpdatedAt),
		group.ID,
	)
	if err != nil {
		return err
	}

	return ensureRowsAffected(result)
}

func (r *GroupRepository) GetByID(ctx context.Context, id string) (*domain.Group, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, name, filter_regex, description, created_at, updated_at FROM groups WHERE id = ?`, id)
	item, err := scanGroup(row)
	if err != nil {
		return nil, translateNotFound(err)
	}
	return item, nil
}

func (r *GroupRepository) List(ctx context.Context) ([]*domain.Group, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, filter_regex, description, created_at, updated_at FROM groups ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domain.Group
	for rows.Next() {
		item, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *GroupRepository) DeleteByID(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, id)
	if err != nil {
		return err
	}

	return ensureRowsAffected(result)
}

type groupScanner interface {
	Scan(dest ...any) error
}

func scanGroup(scanner groupScanner) (*domain.Group, error) {
	var item domain.Group
	var createdAt string
	var updatedAt string
	if err := scanner.Scan(&item.ID, &item.Name, &item.FilterRegex, &item.Description, &createdAt, &updatedAt); err != nil {
		return nil, err
	}

	var err error
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
