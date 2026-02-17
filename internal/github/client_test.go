package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient_EmptyToken(t *testing.T) {
	c := NewClient("")
	if c != nil {
		t.Error("expected nil client for empty token")
	}
}

func TestNewClient_ValidToken(t *testing.T) {
	c := NewClient("ghp_test")
	if c == nil {
		t.Error("expected non-nil client for valid token")
	}
}

func TestClient_IsPRMerged(t *testing.T) {
	tests := []struct {
		name       string
		merged     bool
		statusCode int
		wantMerged bool
		wantErr    bool
	}{
		{"merged PR", true, 200, true, false},
		{"unmerged PR", false, 200, false, false},
		{"API error", false, 404, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("expected Bearer auth header")
				}
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == 200 {
					json.NewEncoder(w).Encode(map[string]any{
						"merged": tt.merged,
						"state":  "closed",
					})
				}
			}))
			defer server.Close()

			c := &Client{
				token:      "test-token",
				httpClient: server.Client(),
			}
			// Override the URL by hijacking the transport
			origTransport := server.Client().Transport
			server.Client().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				r.URL.Scheme = "http"
				r.URL.Host = server.Listener.Addr().String()
				return origTransport.RoundTrip(r)
			})

			merged, err := c.IsPRMerged(context.Background(), "owner", "repo", 1)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsPRMerged() error = %v, wantErr %v", err, tt.wantErr)
			}
			if merged != tt.wantMerged {
				t.Errorf("IsPRMerged() = %v, want %v", merged, tt.wantMerged)
			}
		})
	}
}

func TestClient_ListAccessibleRepos(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("expected Bearer auth header")
		}
		json.NewEncoder(w).Encode([]map[string]any{
			{
				"full_name":   "owner/repo1",
				"name":        "repo1",
				"description": "first repo",
				"private":     false,
				"html_url":    "https://github.com/owner/repo1",
				"owner":       map[string]string{"login": "owner"},
			},
			{
				"full_name":   "owner/repo2",
				"name":        "repo2",
				"description": "second repo",
				"private":     true,
				"html_url":    "https://github.com/owner/repo2",
				"owner":       map[string]string{"login": "owner"},
			},
		})
	}))
	defer server.Close()

	c := &Client{
		token:      "test-token",
		httpClient: server.Client(),
	}
	server.Client().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = server.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(r)
	})

	repos, err := c.ListAccessibleRepos(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
	if repos[0].FullName != "owner/repo1" {
		t.Errorf("expected 'owner/repo1', got %s", repos[0].FullName)
	}
	if repos[0].Owner != "owner" {
		t.Errorf("expected owner 'owner', got %s", repos[0].Owner)
	}
	if repos[1].Private != true {
		t.Error("expected second repo to be private")
	}
}

func TestClient_ClosePR(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("expected PATCH method, got %s", r.Method)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"head": map[string]string{"ref": "feature-branch"},
		})
	}))
	defer server.Close()

	c := &Client{
		token:      "test-token",
		httpClient: server.Client(),
	}
	server.Client().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = server.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(r)
	})

	branch, err := c.ClosePR(context.Background(), "owner", "repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "feature-branch" {
		t.Errorf("expected 'feature-branch', got %s", branch)
	}
}

func TestClient_DeleteBranch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE method, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := &Client{
		token:      "test-token",
		httpClient: server.Client(),
	}
	server.Client().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = server.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(r)
	})

	err := c.DeleteBranch(context.Background(), "owner", "repo", "feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_DeleteBranch_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := &Client{
		token:      "test-token",
		httpClient: server.Client(),
	}
	server.Client().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = server.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(r)
	})

	err := c.DeleteBranch(context.Background(), "owner", "repo", "feature")
	if err == nil {
		t.Error("expected error for 404")
	}
}

func TestClient_FindPRForBranch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{"number": 42, "html_url": "https://github.com/owner/repo/pull/42"},
		})
	}))
	defer server.Close()

	c := &Client{
		token:      "test-token",
		httpClient: server.Client(),
	}
	server.Client().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = server.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(r)
	})

	url, num, err := c.FindPRForBranch(context.Background(), "owner", "repo", "feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if num != 42 {
		t.Errorf("expected PR number 42, got %d", num)
	}
	if url != "https://github.com/owner/repo/pull/42" {
		t.Errorf("expected PR URL, got %s", url)
	}
}

func TestClient_FindPRForBranch_NoPR(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer server.Close()

	c := &Client{
		token:      "test-token",
		httpClient: server.Client(),
	}
	server.Client().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = server.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(r)
	})

	url, num, err := c.FindPRForBranch(context.Background(), "owner", "repo", "feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if num != 0 {
		t.Errorf("expected PR number 0, got %d", num)
	}
	if url != "" {
		t.Errorf("expected empty URL, got %s", url)
	}
}

func TestClient_GetPRMergeability(t *testing.T) {
	boolTrue := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"mergeable":       boolTrue,
			"mergeable_state": "clean",
		})
	}))
	defer server.Close()

	c := &Client{
		token:      "test-token",
		httpClient: server.Client(),
	}
	server.Client().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = server.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(r)
	})

	result, err := c.GetPRMergeability(context.Background(), "owner", "repo", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Mergeable == nil || !*result.Mergeable {
		t.Error("expected mergeable=true")
	}
	if result.MergeableState != "clean" {
		t.Errorf("expected state 'clean', got %s", result.MergeableState)
	}
	if result.HasConflicts {
		t.Error("expected HasConflicts=false for clean state")
	}
}

func TestClient_GetPRMergeability_Dirty(t *testing.T) {
	boolFalse := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"mergeable":       boolFalse,
			"mergeable_state": "dirty",
		})
	}))
	defer server.Close()

	c := &Client{
		token:      "test-token",
		httpClient: server.Client(),
	}
	server.Client().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = server.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(r)
	})

	result, err := c.GetPRMergeability(context.Background(), "owner", "repo", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasConflicts {
		t.Error("expected HasConflicts=true for dirty state")
	}
}

func TestCheckStatusConstants(t *testing.T) {
	if CheckStatusPending != "pending" {
		t.Errorf("expected 'pending', got %s", CheckStatusPending)
	}
	if CheckStatusSuccess != "success" {
		t.Errorf("expected 'success', got %s", CheckStatusSuccess)
	}
	if CheckStatusFailure != "failure" {
		t.Errorf("expected 'failure', got %s", CheckStatusFailure)
	}
}

// roundTripFunc is a helper to override HTTP transport for tests.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
