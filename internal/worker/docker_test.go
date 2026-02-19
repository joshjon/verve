package worker

import "testing"

func TestRewriteLocalhostURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "localhost with port",
			input:    "http://localhost:7400",
			expected: "http://host.docker.internal:7400",
		},
		{
			name:     "127.0.0.1 with port",
			input:    "http://127.0.0.1:7400",
			expected: "http://host.docker.internal:7400",
		},
		{
			name:     "localhost without port",
			input:    "http://localhost",
			expected: "http://host.docker.internal",
		},
		{
			name:     "localhost with path",
			input:    "http://localhost:7400/api/v1",
			expected: "http://host.docker.internal:7400/api/v1",
		},
		{
			name:     "https localhost",
			input:    "https://localhost:7400",
			expected: "https://host.docker.internal:7400",
		},
		{
			name:     "non-localhost unchanged",
			input:    "http://server:7400",
			expected: "http://server:7400",
		},
		{
			name:     "external host unchanged",
			input:    "https://api.example.com",
			expected: "https://api.example.com",
		},
		{
			name:     "invalid URL unchanged",
			input:    "://bad",
			expected: "://bad",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteLocalhostURL(tt.input)
			if got != tt.expected {
				t.Errorf("rewriteLocalhostURL(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
