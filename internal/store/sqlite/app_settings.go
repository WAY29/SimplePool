package sqlite

import (
	"context"
	"database/sql"

	"github.com/WAY29/SimplePool/internal/domain"
)

type AppSettingRepository struct {
	db *sql.DB
}

func (r *AppSettingRepository) Upsert(ctx context.Context, setting *domain.AppSetting) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO app_settings(key, value, created_at, updated_at)
		 VALUES(?, ?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		setting.Key,
		setting.Value,
		formatTime(setting.CreatedAt),
		formatTime(setting.UpdatedAt),
	)
	return err
}

func (r *AppSettingRepository) GetByKey(ctx context.Context, key string) (*domain.AppSetting, error) {
	row := r.db.QueryRowContext(ctx, `SELECT key, value, created_at, updated_at FROM app_settings WHERE key = ?`, key)
	item, err := scanAppSetting(row)
	if err != nil {
		return nil, translateNotFound(err)
	}
	return item, nil
}

type appSettingScanner interface {
	Scan(dest ...any) error
}

func scanAppSetting(scanner appSettingScanner) (*domain.AppSetting, error) {
	var item domain.AppSetting
	var createdAt string
	var updatedAt string
	if err := scanner.Scan(&item.Key, &item.Value, &createdAt, &updatedAt); err != nil {
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
