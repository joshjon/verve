package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_EmptyToken(t *testing.T) {
	c := NewClient("")
	assert.Nil(t, c, "expected nil client for empty token")
}

func TestNewClient_ValidToken(t *testing.T) {
	c := NewClient("ghp_test")
	assert.NotNil(t, c, "expected non-nil client for valid token")
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
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"), "expected Bearer auth header")
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
			if tt.wantErr {
				assert.Error(t, err, "IsPRMerged() expected error")
			} else {
				assert.NoError(t, err, "IsPRMerged() unexpected error")
			}
			assert.Equal(t, tt.wantMerged, merged, "IsPRMerged() result mismatch")
		})
	}
}

func TestClient_ListAccessibleRepos(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"), "expected Bearer auth header")
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
	require.NoError(t, err)
	require.Len(t, repos, 2)
	assert.Equal(t, "owner/repo1", repos[0].FullName)
	assert.Equal(t, "owner", repos[0].Owner)
	assert.True(t, repos[1].Private, "expected second repo to be private")
}

func TestClient_ClosePR(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method, "expected PATCH method")
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
	require.NoError(t, err)
	assert.Equal(t, "feature-branch", branch)
}

func TestClient_DeleteBranch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method, "expected DELETE method")
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
	require.NoError(t, err)
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
	assert.Error(t, err, "expected error for 404")
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
	require.NoError(t, err)
	assert.Equal(t, 42, num)
	assert.Equal(t, "https://github.com/owner/repo/pull/42", url)
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
	require.NoError(t, err)
	assert.Equal(t, 0, num)
	assert.Empty(t, url)
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
	require.NoError(t, err)
	require.NotNil(t, result.Mergeable, "expected mergeable to be non-nil")
	assert.True(t, *result.Mergeable, "expected mergeable=true")
	assert.Equal(t, "clean", result.MergeableState)
	assert.False(t, result.HasConflicts, "expected HasConflicts=false for clean state")
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
	require.NoError(t, err)
	assert.True(t, result.HasConflicts, "expected HasConflicts=true for dirty state")
}

func TestCheckStatusConstants(t *testing.T) {
	assert.Equal(t, CheckStatus("pending"), CheckStatusPending)
	assert.Equal(t, CheckStatus("success"), CheckStatusSuccess)
	assert.Equal(t, CheckStatus("failure"), CheckStatusFailure)
}

// roundTripFunc is a helper to override HTTP transport for tests.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
