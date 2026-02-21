package setting

import (
	"context"
	"errors"
	"strings"
	"sync"
)

// Well-known setting keys.
const KeyDefaultModel = "default_model"

// ErrNotFound is returned when a setting key does not exist.
var ErrNotFound = errors.New("setting not found")

// DefaultModels is the built-in list of Claude model options when CLAUDE_MODELS
// is not set.
var DefaultModels = []ModelOption{
	{Value: "haiku", Label: "Haiku"},
	{Value: "sonnet", Label: "Sonnet"},
	{Value: "opus", Label: "Opus"},
}

// ModelOption describes an available model for the UI.
type ModelOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ParseModelsEnv parses a comma-separated CLAUDE_MODELS environment variable
// into a slice of ModelOption. Each entry is "value" or "value:label".
// Example: "haiku,sonnet,opus" or "claude-3-haiku:Haiku,claude-3-sonnet:Sonnet".
func ParseModelsEnv(s string) []ModelOption {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	models := make([]ModelOption, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		value := part
		label := part
		if idx := strings.Index(part, ":"); idx > 0 {
			value = part[:idx]
			label = part[idx+1:]
		}
		models = append(models, ModelOption{Value: value, Label: label})
	}
	return models
}

// Repository defines data access for key-value settings.
type Repository interface {
	UpsertSetting(ctx context.Context, key, value string) error
	ReadSetting(ctx context.Context, key string) (string, error)
	DeleteSetting(ctx context.Context, key string) error
	ListSettings(ctx context.Context) (map[string]string, error)
}

// Service provides cached access to key-value settings.
type Service struct {
	repo  Repository
	mu    sync.RWMutex
	cache map[string]string
}

// NewService creates a new settings service.
func NewService(repo Repository) *Service {
	return &Service{
		repo:  repo,
		cache: make(map[string]string),
	}
}

// Load reads all settings from the database into the cache.
func (s *Service) Load(ctx context.Context) error {
	settings, err := s.repo.ListSettings(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = settings
	return nil
}

// Get returns the cached value for a key, or empty string if not set.
func (s *Service) Get(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache[key]
}

// Set writes a setting to the database and updates the cache.
func (s *Service) Set(ctx context.Context, key, value string) error {
	if err := s.repo.UpsertSetting(ctx, key, value); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[key] = value
	return nil
}

// Delete removes a setting from the database and cache.
func (s *Service) Delete(ctx context.Context, key string) error {
	if err := s.repo.DeleteSetting(ctx, key); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cache, key)
	return nil
}
