package githubtoken

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"verve/internal/crypto"
)

type mockTokenRepo struct {
	token     string
	stored    bool
	upsertErr error
	readErr   error
	deleteErr error
}

func (m *mockTokenRepo) UpsertGitHubToken(_ context.Context, encryptedToken string, _ time.Time) error {
	if m.upsertErr != nil {
		return m.upsertErr
	}
	m.token = encryptedToken
	m.stored = true
	return nil
}

func (m *mockTokenRepo) ReadGitHubToken(_ context.Context) (string, error) {
	if m.readErr != nil {
		return "", m.readErr
	}
	if !m.stored {
		return "", ErrTokenNotFound
	}
	return m.token, nil
}

func (m *mockTokenRepo) DeleteGitHubToken(_ context.Context) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.token = ""
	m.stored = false
	return nil
}

func validKey() []byte {
	return []byte("0123456789abcdef0123456789abcdef")
}

func TestIsValidTokenPrefix(t *testing.T) {
	tests := []struct {
		token string
		valid bool
	}{
		{"ghp_abc123", true},
		{"github_pat_abc123", true},
		{"gho_abc123", false},
		{"invalid", false},
		{"", false},
		{"ghp_", true},
		{"github_pat_", true},
	}

	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			result := IsValidTokenPrefix(tt.token)
			assert.Equal(t, tt.valid, result, "IsValidTokenPrefix(%q)", tt.token)
		})
	}
}

func TestService_SaveAndGetToken(t *testing.T) {
	repo := &mockTokenRepo{}
	svc := NewService(repo, validKey())

	err := svc.SaveToken(context.Background(), "ghp_testtoken123")
	require.NoError(t, err, "save")

	token := svc.GetToken()
	assert.Equal(t, "ghp_testtoken123", token)

	assert.True(t, svc.HasToken(), "expected HasToken to return true")
	assert.False(t, svc.IsFineGrained(), "expected IsFineGrained to return false for ghp_ token")

	// Verify the stored token is encrypted
	assert.NotEqual(t, "ghp_testtoken123", repo.token, "expected stored token to be encrypted")
}

func TestService_SaveFineGrainedToken(t *testing.T) {
	repo := &mockTokenRepo{}
	svc := NewService(repo, validKey())

	err := svc.SaveToken(context.Background(), "github_pat_testtoken123")
	require.NoError(t, err, "save")

	assert.True(t, svc.IsFineGrained(), "expected IsFineGrained to return true for github_pat_ token")
}

func TestService_GetClient(t *testing.T) {
	repo := &mockTokenRepo{}
	svc := NewService(repo, validKey())

	// Before save, client should be nil
	assert.Nil(t, svc.GetClient(), "expected nil client before save")

	_ = svc.SaveToken(context.Background(), "ghp_testtoken123")

	client := svc.GetClient()
	assert.NotNil(t, client, "expected non-nil client after save")
}

func TestService_DeleteToken(t *testing.T) {
	repo := &mockTokenRepo{}
	svc := NewService(repo, validKey())

	_ = svc.SaveToken(context.Background(), "ghp_testtoken123")
	require.True(t, svc.HasToken(), "expected token to be saved")

	err := svc.DeleteToken(context.Background())
	require.NoError(t, err, "delete")

	assert.False(t, svc.HasToken(), "expected HasToken to return false after delete")
	assert.Empty(t, svc.GetToken(), "expected empty token after delete")
	assert.Nil(t, svc.GetClient(), "expected nil client after delete")
}

func TestService_Load(t *testing.T) {
	key := validKey()
	repo := &mockTokenRepo{}

	// First, save a token
	encrypted, err := crypto.Encrypt(key, "ghp_loaded_token")
	require.NoError(t, err, "encrypt")
	repo.token = encrypted
	repo.stored = true

	// Now load it
	svc := NewService(repo, key)
	err = svc.Load(context.Background())
	require.NoError(t, err, "load")

	assert.Equal(t, "ghp_loaded_token", svc.GetToken())
	assert.True(t, svc.HasToken(), "expected HasToken to return true after load")
}

func TestService_Load_NoToken(t *testing.T) {
	repo := &mockTokenRepo{stored: false}
	svc := NewService(repo, validKey())

	err := svc.Load(context.Background())
	require.NoError(t, err, "load with no token should not error")

	assert.False(t, svc.HasToken(), "expected HasToken to return false when no token stored")
}

func TestService_Load_ReadError(t *testing.T) {
	repo := &mockTokenRepo{readErr: errors.New("db error")}
	svc := NewService(repo, validKey())

	err := svc.Load(context.Background())
	assert.Error(t, err, "expected error from load")
}

func TestService_SaveToken_RepoError(t *testing.T) {
	repo := &mockTokenRepo{upsertErr: errors.New("db error")}
	svc := NewService(repo, validKey())

	err := svc.SaveToken(context.Background(), "ghp_test")
	assert.Error(t, err, "expected error from save")
}

func TestService_DeleteToken_RepoError(t *testing.T) {
	repo := &mockTokenRepo{deleteErr: errors.New("db error")}
	svc := NewService(repo, validKey())

	_ = svc.SaveToken(context.Background(), "ghp_test")
	repo.deleteErr = errors.New("db error")

	err := svc.DeleteToken(context.Background())
	assert.Error(t, err, "expected error from delete")

	// Token should still be cached
	assert.True(t, svc.HasToken(), "expected token to remain cached on delete error")
}
