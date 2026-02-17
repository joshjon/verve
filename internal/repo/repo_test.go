package repo

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRepo_Valid(t *testing.T) {
	r, err := NewRepo("octocat/hello-world")
	require.NoError(t, err)
	assert.Equal(t, "octocat", r.Owner)
	assert.Equal(t, "hello-world", r.Name)
	assert.Equal(t, "octocat/hello-world", r.FullName)
	assert.NotEmpty(t, r.ID.String(), "expected non-empty ID")
	assert.True(t, strings.HasPrefix(r.ID.String(), "repo_"), "expected repo_ prefix, got %s", r.ID.String())
	assert.False(t, r.CreatedAt.IsZero(), "expected non-zero CreatedAt")
}

func TestNewRepo_InvalidNoSlash(t *testing.T) {
	_, err := NewRepo("justname")
	assert.Error(t, err, "expected error for repo name without slash")
}

func TestNewRepo_EmptyOwner(t *testing.T) {
	_, err := NewRepo("/reponame")
	assert.Error(t, err, "expected error for empty owner")
}

func TestNewRepo_EmptyName(t *testing.T) {
	_, err := NewRepo("owner/")
	assert.Error(t, err, "expected error for empty name")
}

func TestNewRepo_EmptyString(t *testing.T) {
	_, err := NewRepo("")
	assert.Error(t, err, "expected error for empty string")
}

func TestNewRepo_MultipleSlashes(t *testing.T) {
	// SplitN with n=2 should handle this correctly: "owner" and "repo/subpath"
	r, err := NewRepo("owner/repo/subpath")
	require.NoError(t, err)
	assert.Equal(t, "owner", r.Owner)
	assert.Equal(t, "repo/subpath", r.Name)
}

// --- RepoID tests ---

func TestNewRepoID(t *testing.T) {
	id := NewRepoID()
	s := id.String()
	assert.NotEmpty(t, s, "expected non-empty string")
	assert.True(t, strings.HasPrefix(s, "repo_"), "expected repo_ prefix, got %s", s)
}

func TestParseRepoID_Valid(t *testing.T) {
	original := NewRepoID()
	parsed, err := ParseRepoID(original.String())
	require.NoError(t, err)
	assert.Equal(t, original.String(), parsed.String())
}

func TestParseRepoID_InvalidPrefix(t *testing.T) {
	_, err := ParseRepoID("tsk_01h2xcejqtf2nbrexx3vqjhp41")
	assert.Error(t, err, "expected error for wrong prefix")
}

func TestParseRepoID_Empty(t *testing.T) {
	_, err := ParseRepoID("")
	assert.Error(t, err, "expected error for empty string")
}

func TestMustParseRepoID_Panics(t *testing.T) {
	assert.Panics(t, func() {
		MustParseRepoID("invalid")
	}, "expected panic for invalid repo ID")
}

// --- Store tests ---

type mockRepoRepository struct {
	repos         map[string]*Repo
	createErr     error
	readErr       error
	readByNameErr error
	deleteErr     error
}

func newMockRepoRepository() *mockRepoRepository {
	return &mockRepoRepository{repos: make(map[string]*Repo)}
}

func (m *mockRepoRepository) CreateRepo(_ context.Context, r *Repo) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.repos[r.ID.String()] = r
	return nil
}

func (m *mockRepoRepository) ReadRepo(_ context.Context, id RepoID) (*Repo, error) {
	if m.readErr != nil {
		return nil, m.readErr
	}
	r, ok := m.repos[id.String()]
	if !ok {
		return nil, errors.New("not found")
	}
	return r, nil
}

func (m *mockRepoRepository) ReadRepoByFullName(_ context.Context, fullName string) (*Repo, error) {
	if m.readByNameErr != nil {
		return nil, m.readByNameErr
	}
	for _, r := range m.repos {
		if r.FullName == fullName {
			return r, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockRepoRepository) ListRepos(_ context.Context) ([]*Repo, error) {
	var result []*Repo
	for _, r := range m.repos {
		result = append(result, r)
	}
	return result, nil
}

func (m *mockRepoRepository) DeleteRepo(_ context.Context, id RepoID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.repos, id.String())
	return nil
}

type mockTaskChecker struct {
	hasTasks bool
	err      error
}

func (m *mockTaskChecker) HasTasksForRepo(_ context.Context, _ string) (bool, error) {
	return m.hasTasks, m.err
}

func TestStore_DeleteRepo_NoTasks(t *testing.T) {
	repoRepo := newMockRepoRepository()
	checker := &mockTaskChecker{hasTasks: false}
	store := NewStore(repoRepo, checker)

	r, _ := NewRepo("owner/name")
	_ = store.CreateRepo(context.Background(), r)

	err := store.DeleteRepo(context.Background(), r.ID)
	require.NoError(t, err)
}

func TestStore_DeleteRepo_WithTasks(t *testing.T) {
	repoRepo := newMockRepoRepository()
	checker := &mockTaskChecker{hasTasks: true}
	store := NewStore(repoRepo, checker)

	r, _ := NewRepo("owner/name")
	_ = store.CreateRepo(context.Background(), r)

	err := store.DeleteRepo(context.Background(), r.ID)
	assert.Error(t, err, "expected error when tasks exist")
	assert.Contains(t, err.Error(), "tasks still exist")
}

func TestStore_CreateAndReadRepo(t *testing.T) {
	repoRepo := newMockRepoRepository()
	checker := &mockTaskChecker{}
	store := NewStore(repoRepo, checker)

	r, _ := NewRepo("owner/name")
	err := store.CreateRepo(context.Background(), r)
	require.NoError(t, err)

	read, err := store.ReadRepo(context.Background(), r.ID)
	require.NoError(t, err)
	assert.Equal(t, r.FullName, read.FullName)
}

func TestStore_ListRepos(t *testing.T) {
	repoRepo := newMockRepoRepository()
	checker := &mockTaskChecker{}
	store := NewStore(repoRepo, checker)

	r1, _ := NewRepo("owner/repo1")
	r2, _ := NewRepo("owner/repo2")
	_ = store.CreateRepo(context.Background(), r1)
	_ = store.CreateRepo(context.Background(), r2)

	repos, err := store.ListRepos(context.Background())
	require.NoError(t, err)
	assert.Len(t, repos, 2)
}

func TestStore_ReadRepoByFullName(t *testing.T) {
	repoRepo := newMockRepoRepository()
	checker := &mockTaskChecker{}
	store := NewStore(repoRepo, checker)

	r, _ := NewRepo("owner/name")
	_ = store.CreateRepo(context.Background(), r)

	read, err := store.ReadRepoByFullName(context.Background(), "owner/name")
	require.NoError(t, err)
	assert.Equal(t, r.ID.String(), read.ID.String())
}
