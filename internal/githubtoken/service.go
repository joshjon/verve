package githubtoken

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"verve/internal/crypto"
	"verve/internal/github"
)

const (
	// classicTokenPrefix is the prefix for classic personal access tokens.
	classicTokenPrefix = "ghp_"
	// fineGrainedTokenPrefix is the prefix for fine-grained personal access tokens.
	fineGrainedTokenPrefix = "github_pat_"
)

// IsValidTokenPrefix checks whether a token has a recognised GitHub PAT prefix.
func IsValidTokenPrefix(token string) bool {
	return strings.HasPrefix(token, classicTokenPrefix) || strings.HasPrefix(token, fineGrainedTokenPrefix)
}

// ErrTokenNotFound is returned when no GitHub token is stored.
var ErrTokenNotFound = errors.New("github token not found")

// Repository defines the data access methods for the encrypted GitHub token.
type Repository interface {
	UpsertGitHubToken(ctx context.Context, encryptedToken string, now time.Time) error
	ReadGitHubToken(ctx context.Context) (string, error)
	DeleteGitHubToken(ctx context.Context) error
}

// Service manages the GitHub token lifecycle: encryption, storage, and
// in-memory caching of the decrypted token and GitHub client.
type Service struct {
	repo Repository
	key  []byte

	mu     sync.RWMutex
	token  string
	client *github.Client
}

// NewService creates a new GitHubTokenService.
func NewService(repo Repository, encryptionKey []byte) *Service {
	return &Service{
		repo: repo,
		key:  encryptionKey,
	}
}

// Load reads the encrypted token from the database and hydrates the in-memory
// cache. Call this on server startup. If no token is stored, this is a no-op.
func (s *Service) Load(ctx context.Context) error {
	encrypted, err := s.repo.ReadGitHubToken(ctx)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return nil
		}
		return err
	}

	plaintext, err := crypto.Decrypt(s.key, encrypted)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = plaintext
	s.client = github.NewClient(plaintext)
	return nil
}

// SaveToken encrypts the token, stores it in the database, and updates the
// in-memory cache.
func (s *Service) SaveToken(ctx context.Context, plaintext string) error {
	encrypted, err := crypto.Encrypt(s.key, plaintext)
	if err != nil {
		return err
	}

	if err := s.repo.UpsertGitHubToken(ctx, encrypted, time.Now()); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = plaintext
	s.client = github.NewClient(plaintext)
	return nil
}

// GetToken returns the cached decrypted token. Returns empty string if no
// token is configured.
func (s *Service) GetToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.token
}

// GetClient returns the cached GitHub client. Returns nil if no token is
// configured.
func (s *Service) GetClient() *github.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client
}

// HasToken reports whether a GitHub token is currently configured.
func (s *Service) HasToken() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.token != ""
}

// IsFineGrained reports whether the configured token is a fine-grained PAT.
func (s *Service) IsFineGrained() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return strings.HasPrefix(s.token, fineGrainedTokenPrefix)
}

// DeleteToken removes the token from the database and clears the in-memory
// cache.
func (s *Service) DeleteToken(ctx context.Context) error {
	if err := s.repo.DeleteGitHubToken(ctx); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = ""
	s.client = nil
	return nil
}
