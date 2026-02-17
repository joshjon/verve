package repo

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNewRepo_Valid(t *testing.T) {
	r, err := NewRepo("octocat/hello-world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Owner != "octocat" {
		t.Errorf("expected owner 'octocat', got %s", r.Owner)
	}
	if r.Name != "hello-world" {
		t.Errorf("expected name 'hello-world', got %s", r.Name)
	}
	if r.FullName != "octocat/hello-world" {
		t.Errorf("expected full_name 'octocat/hello-world', got %s", r.FullName)
	}
	if r.ID.String() == "" {
		t.Error("expected non-empty ID")
	}
	if !strings.HasPrefix(r.ID.String(), "repo_") {
		t.Errorf("expected repo_ prefix, got %s", r.ID.String())
	}
	if r.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestNewRepo_InvalidNoSlash(t *testing.T) {
	_, err := NewRepo("justname")
	if err == nil {
		t.Error("expected error for repo name without slash")
	}
}

func TestNewRepo_EmptyOwner(t *testing.T) {
	_, err := NewRepo("/reponame")
	if err == nil {
		t.Error("expected error for empty owner")
	}
}

func TestNewRepo_EmptyName(t *testing.T) {
	_, err := NewRepo("owner/")
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestNewRepo_EmptyString(t *testing.T) {
	_, err := NewRepo("")
	if err == nil {
		t.Error("expected error for empty string")
	}
}

func TestNewRepo_MultipleSlashes(t *testing.T) {
	// SplitN with n=2 should handle this correctly: "owner" and "repo/subpath"
	r, err := NewRepo("owner/repo/subpath")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Owner != "owner" {
		t.Errorf("expected owner 'owner', got %s", r.Owner)
	}
	if r.Name != "repo/subpath" {
		t.Errorf("expected name 'repo/subpath', got %s", r.Name)
	}
}

// --- RepoID tests ---

func TestNewRepoID(t *testing.T) {
	id := NewRepoID()
	s := id.String()
	if s == "" {
		t.Error("expected non-empty string")
	}
	if !strings.HasPrefix(s, "repo_") {
		t.Errorf("expected repo_ prefix, got %s", s)
	}
}

func TestParseRepoID_Valid(t *testing.T) {
	original := NewRepoID()
	parsed, err := ParseRepoID(original.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.String() != original.String() {
		t.Errorf("expected %s, got %s", original.String(), parsed.String())
	}
}

func TestParseRepoID_InvalidPrefix(t *testing.T) {
	_, err := ParseRepoID("tsk_01h2xcejqtf2nbrexx3vqjhp41")
	if err == nil {
		t.Error("expected error for wrong prefix")
	}
}

func TestParseRepoID_Empty(t *testing.T) {
	_, err := ParseRepoID("")
	if err == nil {
		t.Error("expected error for empty string")
	}
}

func TestMustParseRepoID_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid repo ID")
		}
	}()
	MustParseRepoID("invalid")
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStore_DeleteRepo_WithTasks(t *testing.T) {
	repoRepo := newMockRepoRepository()
	checker := &mockTaskChecker{hasTasks: true}
	store := NewStore(repoRepo, checker)

	r, _ := NewRepo("owner/name")
	_ = store.CreateRepo(context.Background(), r)

	err := store.DeleteRepo(context.Background(), r.ID)
	if err == nil {
		t.Error("expected error when tasks exist")
	}
	if !strings.Contains(err.Error(), "tasks still exist") {
		t.Errorf("expected 'tasks still exist' error, got %s", err.Error())
	}
}

func TestStore_CreateAndReadRepo(t *testing.T) {
	repoRepo := newMockRepoRepository()
	checker := &mockTaskChecker{}
	store := NewStore(repoRepo, checker)

	r, _ := NewRepo("owner/name")
	err := store.CreateRepo(context.Background(), r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	read, err := store.ReadRepo(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if read.FullName != r.FullName {
		t.Errorf("expected full_name %s, got %s", r.FullName, read.FullName)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(repos))
	}
}

func TestStore_ReadRepoByFullName(t *testing.T) {
	repoRepo := newMockRepoRepository()
	checker := &mockTaskChecker{}
	store := NewStore(repoRepo, checker)

	r, _ := NewRepo("owner/name")
	_ = store.CreateRepo(context.Background(), r)

	read, err := store.ReadRepoByFullName(context.Background(), "owner/name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if read.ID.String() != r.ID.String() {
		t.Errorf("expected ID %s, got %s", r.ID.String(), read.ID.String())
	}
}
