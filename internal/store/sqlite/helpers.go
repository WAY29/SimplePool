package sqlite

import (
	"database/sql"
	"errors"
	"time"

	"github.com/WAY29/SimplePool/internal/store"
)

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(raw string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, raw)
}

func parseNullableTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid || value.String == "" {
		return nil, nil
	}

	parsed, err := parseTime(value.String)
	if err != nil {
		return nil, err
	}

	return &parsed, nil
}

func nullableTimeValue(value *time.Time) any {
	if value == nil {
		return nil
	}

	return formatTime(*value)
}

func nullableStringValue(value *string) any {
	if value == nil {
		return nil
	}

	return *value
}

func nullableInt64Value(value *int64) any {
	if value == nil {
		return nil
	}

	return *value
}

func boolToInt(value bool) int {
	if value {
		return 1
	}

	return 0
}

func translateNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return store.ErrNotFound
	}

	return err
}
