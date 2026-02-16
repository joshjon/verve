package sqlite

import (
	"context"

	"verve/internal/setting"
	"verve/internal/sqlite/sqlc"
)

var _ setting.Repository = (*SettingRepository)(nil)

// SettingRepository implements setting.Repository using SQLite.
type SettingRepository struct {
	db *sqlc.Queries
}

// NewSettingRepository creates a new SettingRepository backed by the given SQLite DB.
func NewSettingRepository(db DB) *SettingRepository {
	return &SettingRepository{db: sqlc.New(db)}
}

func (r *SettingRepository) UpsertSetting(ctx context.Context, key, value string) error {
	return r.db.UpsertSetting(ctx, sqlc.UpsertSettingParams{Key: key, Value: value})
}

func (r *SettingRepository) ReadSetting(ctx context.Context, key string) (string, error) {
	return r.db.ReadSetting(ctx, key)
}

func (r *SettingRepository) DeleteSetting(ctx context.Context, key string) error {
	return r.db.DeleteSetting(ctx, key)
}

func (r *SettingRepository) ListSettings(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.ListSettings(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(rows))
	for _, row := range rows {
		m[row.Key] = row.Value
	}
	return m, nil
}
