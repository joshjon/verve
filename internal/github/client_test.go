package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_EmptyToken(t *testing.T) {
	c := NewClient("", false)
	assert.Nil(t, c, "expected nil client for empty token")
}

func TestNewClient_ValidToken(t *testing.T) {
	c := NewClient("ghp_test", false)
	assert.NotNil(t, c, "expected non-nil client for valid token")
}

func TestNewClient_InsecureSkipVerify(t *testing.T) {
	c := NewClient("ghp_test", true)
	require.NotNil(t, c, "expected non-nil client")

	transport, ok := c.httpClient.Transport.(*http.Transport)
	require.True(t, ok, "expected *http.Transport")
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify, "expected InsecureSkipVerify=true")
}

func TestNewClient_SecureByDefault(t *testing.T) {
	c := NewClient("ghp_test", false)
	require.NotNil(t, c, "expected non-nil client")

	// Default http.Client has nil Transport, which uses http.DefaultTransport
	assert.Nil(t, c.httpClient.Transport, "expected nil transport (default) when insecureSkipVerify=false")
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

func TestExtractStepLogs(t *testing.T) {
	tests := []struct {
		name     string
		rawLog   string
		stepName string
		want     string
	}{
		{
			name:     "empty step name returns full log",
			rawLog:   "line1\nline2\nline3",
			stepName: "",
			want:     "line1\nline2\nline3",
		},
		{
			name: "extracts specific step",
			rawLog: "2024-01-01T00:00:00.0000000Z ##[group]Set up job\n" +
				"setting up...\n" +
				"2024-01-01T00:00:01.0000000Z ##[group]Run tests\n" +
				"running test 1\n" +
				"running test 2\n" +
				"FAIL: test 2 failed\n" +
				"2024-01-01T00:00:02.0000000Z ##[group]Post cleanup\n" +
				"cleaning up...",
			stepName: "Run tests",
			want:     "running test 1\nrunning test 2\nFAIL: test 2 failed",
		},
		{
			name: "step not found returns full log",
			rawLog: "2024-01-01T00:00:00.0000000Z ##[group]Set up job\n" +
				"setting up...\n" +
				"2024-01-01T00:00:01.0000000Z ##[group]Build\n" +
				"building...",
			stepName: "Run tests",
			want: "2024-01-01T00:00:00.0000000Z ##[group]Set up job\n" +
				"setting up...\n" +
				"2024-01-01T00:00:01.0000000Z ##[group]Build\n" +
				"building...",
		},
		{
			name: "last step in log without trailing group marker",
			rawLog: "2024-01-01T00:00:00.0000000Z ##[group]Set up job\n" +
				"setting up...\n" +
				"2024-01-01T00:00:01.0000000Z ##[group]Take UI screenshots\n" +
				"screenshot 1 ok\n" +
				"screenshot 2 FAILED\n" +
				"Error: element not found",
			stepName: "Take UI screenshots",
			want:     "screenshot 1 ok\nscreenshot 2 FAILED\nError: element not found",
		},
		{
			name: "handles endgroup markers within step",
			rawLog: "2024-01-01T00:00:00.0000000Z ##[group]Run tests\n" +
				"test output line 1\n" +
				"2024-01-01T00:00:00.5000000Z ##[endgroup]\n" +
				"test output line 2\n" +
				"2024-01-01T00:00:01.0000000Z ##[group]Cleanup\n" +
				"cleanup...",
			stepName: "Run tests",
			want:     "test output line 1\ntest output line 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractStepLogs(tt.rawLog, tt.stepName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetFailedCheckLogs_StepExtraction(t *testing.T) {
	failureConclusion := "failure"
	successConclusion := "success"

	// Simulate a GitHub API server that serves:
	// 1. PR details (head SHA)
	// 2. Check runs (one failed job)
	// 3. Commit statuses (empty)
	// 4. Job details with step info
	// 5. Job logs with step markers
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/pulls/1"):
			json.NewEncoder(w).Encode(map[string]any{
				"head": map[string]string{"sha": "abc123"},
			})
		case strings.Contains(path, "/check-runs") && !strings.Contains(path, "/actions/jobs/"):
			json.NewEncoder(w).Encode(map[string]any{
				"check_runs": []map[string]any{
					{
						"id":         42,
						"name":       "Capture UI screenshots",
						"status":     "completed",
						"conclusion": &failureConclusion,
						"html_url":   "https://github.com/owner/repo/actions/runs/1/jobs/42",
					},
				},
			})
		case strings.HasSuffix(path, "/status"):
			json.NewEncoder(w).Encode(map[string]any{
				"state":    "success",
				"statuses": []any{},
			})
		case strings.HasSuffix(path, "/actions/jobs/42") && !strings.HasSuffix(path, "/logs"):
			json.NewEncoder(w).Encode(map[string]any{
				"steps": []map[string]any{
					{"name": "Set up job", "status": "completed", "conclusion": &successConclusion, "number": 1},
					{"name": "Checkout code", "status": "completed", "conclusion": &successConclusion, "number": 2},
					{"name": "Take UI screenshots", "status": "completed", "conclusion": &failureConclusion, "number": 3},
					{"name": "Upload artifacts", "status": "completed", "conclusion": nil, "number": 4},
				},
			})
		case strings.HasSuffix(path, "/actions/jobs/42/logs"):
			w.Write([]byte(
				"2024-01-01T00:00:00.0000000Z ##[group]Set up job\n" +
					"setup line 1\n" +
					"setup line 2\n" +
					"2024-01-01T00:00:01.0000000Z ##[group]Checkout code\n" +
					"checkout line 1\n" +
					"2024-01-01T00:00:02.0000000Z ##[group]Take UI screenshots\n" +
					"screenshot 1 ok\n" +
					"screenshot 2 ok\n" +
					"screenshot 3 FAILED\n" +
					"Error: element not visible on page\n" +
					"2024-01-01T00:00:03.0000000Z ##[group]Upload artifacts\n" +
					"uploading...",
			))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
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

	result, err := c.GetFailedCheckLogs(context.Background(), "owner", "repo", 1)
	require.NoError(t, err)

	// Should contain the failed step name in the header
	assert.Contains(t, result, "=== Failed: Capture UI screenshots > Take UI screenshots ===")
	// Should contain the failed step's logs
	assert.Contains(t, result, "screenshot 3 FAILED")
	assert.Contains(t, result, "Error: element not visible on page")
	// Should NOT contain logs from other steps
	assert.NotContains(t, result, "setup line 1")
	assert.NotContains(t, result, "checkout line 1")
	assert.NotContains(t, result, "uploading...")
}

// roundTripFunc is a helper to override HTTP transport for tests.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
