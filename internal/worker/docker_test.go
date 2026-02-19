package worker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeAPIURLForHostNetwork(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Docker Compose service name 'server'",
			input:    "http://server:7400",
			expected: "http://localhost:7400",
		},
		{
			name:     "Docker Compose service name 'server' with HTTPS",
			input:    "https://server:7400",
			expected: "https://localhost:7400",
		},
		{
			name:     "Docker Compose service name 'server' with path",
			input:    "http://server:7400/api/v1",
			expected: "http://localhost:7400/api/v1",
		},
		{
			name:     "Docker Compose service name 'api'",
			input:    "http://api:8080",
			expected: "http://localhost:8080",
		},
		{
			name:     "Docker Compose service name 'backend'",
			input:    "http://backend:3000",
			expected: "http://localhost:3000",
		},
		{
			name:     "Docker Compose service name 'verve'",
			input:    "http://verve:7400",
			expected: "http://localhost:7400",
		},
		{
			name:     "Already localhost - no change",
			input:    "http://localhost:7400",
			expected: "http://localhost:7400",
		},
		{
			name:     "External domain - no change",
			input:    "http://example.com:7400",
			expected: "http://example.com:7400",
		},
		{
			name:     "External domain with HTTPS - no change",
			input:    "https://api.example.com:443",
			expected: "https://api.example.com:443",
		},
		{
			name:     "IP address - no change",
			input:    "http://192.168.1.1:7400",
			expected: "http://192.168.1.1:7400",
		},
		{
			name:     "Localhost with HTTPS - no change",
			input:    "https://localhost:7400",
			expected: "https://localhost:7400",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeAPIURLForHostNetwork(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
