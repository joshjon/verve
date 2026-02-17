package setting

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSettingRepo struct {
	settings  map[string]string
	upsertErr error
	readErr   error
	deleteErr error
	listErr   error
}

func newMockSettingRepo() *mockSettingRepo {
	return &mockSettingRepo{settings: make(map[string]string)}
}

func (m *mockSettingRepo) UpsertSetting(_ context.Context, key, value string) error {
	if m.upsertErr != nil {
		return m.upsertErr
	}
	m.settings[key] = value
	return nil
}

func (m *mockSettingRepo) ReadSetting(_ context.Context, key string) (string, error) {
	if m.readErr != nil {
		return "", m.readErr
	}
	v, ok := m.settings[key]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (m *mockSettingRepo) DeleteSetting(_ context.Context, key string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.settings, key)
	return nil
}

func (m *mockSettingRepo) ListSettings(_ context.Context) (map[string]string, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make(map[string]string)
	for k, v := range m.settings {
		result[k] = v
	}
	return result, nil
}

func TestService_GetEmpty(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewService(repo)

	val := svc.Get("nonexistent")
	assert.Empty(t, val)
}

func TestService_SetAndGet(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewService(repo)

	ctx := context.Background()
	err := svc.Set(ctx, KeyDefaultModel, "opus")
	require.NoError(t, err)

	val := svc.Get(KeyDefaultModel)
	assert.Equal(t, "opus", val)

	// Verify it was persisted to the repo
	assert.Equal(t, "opus", repo.settings[KeyDefaultModel], "expected setting to be persisted in repo")
}

func TestService_SetError(t *testing.T) {
	repo := newMockSettingRepo()
	repo.upsertErr = errors.New("db error")
	svc := NewService(repo)

	err := svc.Set(context.Background(), "key", "value")
	assert.Error(t, err, "expected error from repo")

	// Cache should not be updated on error
	val := svc.Get("key")
	assert.Empty(t, val, "expected cache to not be updated on error")
}

func TestService_Delete(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewService(repo)

	ctx := context.Background()
	_ = svc.Set(ctx, "key", "value")

	err := svc.Delete(ctx, "key")
	require.NoError(t, err)

	val := svc.Get("key")
	assert.Empty(t, val, "expected empty string after delete")
}

func TestService_DeleteError(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewService(repo)

	ctx := context.Background()
	_ = svc.Set(ctx, "key", "value")

	repo.deleteErr = errors.New("db error")
	err := svc.Delete(ctx, "key")
	assert.Error(t, err, "expected error from repo")

	// Cache should not be updated on error
	val := svc.Get("key")
	assert.Equal(t, "value", val, "expected cache to retain value on delete error")
}

func TestService_Load(t *testing.T) {
	repo := newMockSettingRepo()
	repo.settings["key1"] = "value1"
	repo.settings["key2"] = "value2"

	svc := NewService(repo)

	err := svc.Load(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "value1", svc.Get("key1"))
	assert.Equal(t, "value2", svc.Get("key2"))
}

func TestService_LoadError(t *testing.T) {
	repo := newMockSettingRepo()
	repo.listErr = errors.New("db error")
	svc := NewService(repo)

	err := svc.Load(context.Background())
	assert.Error(t, err, "expected error from repo")
}

func TestKeyDefaultModel(t *testing.T) {
	assert.Equal(t, "default_model", KeyDefaultModel)
}
