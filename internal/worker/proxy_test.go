package worker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joshjon/kit/log"
)

func TestBetaHeaderProxy_StripsAnthropicBetaHeader(t *testing.T) {
	var receivedHeaders http.Header

	// Upstream server that records incoming headers.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	logger := log.NewLogger(log.WithDevelopment())
	proxy, err := StartBetaHeaderProxy(context.Background(), upstream.URL, logger)
	if err != nil {
		t.Fatalf("failed to start proxy: %v", err)
	}
	defer func() { _ = proxy.Stop(context.Background()) }()

	// Make a request with anthropic-beta header through the proxy.
	proxyURL := fmt.Sprintf("http://localhost:%d/v1/messages", proxy.Port())
	req, err := http.NewRequest(http.MethodPost, proxyURL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("anthropic-beta", "some-beta-feature-2024-01-01")
	req.Header.Set("x-api-key", "test-key")
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request through proxy failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify anthropic-beta was stripped.
	if got := receivedHeaders.Get("anthropic-beta"); got != "" {
		t.Errorf("expected anthropic-beta header to be stripped, but got %q", got)
	}

	// Verify other headers were preserved.
	if got := receivedHeaders.Get("x-api-key"); got != "test-key" {
		t.Errorf("expected x-api-key to be preserved, got %q", got)
	}
	if got := receivedHeaders.Get("content-type"); got != "application/json" {
		t.Errorf("expected content-type to be preserved, got %q", got)
	}
}

func TestBetaHeaderProxy_PreservesOtherHeaders(t *testing.T) {
	var receivedHeaders http.Header

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	logger := log.NewLogger(log.WithDevelopment())
	proxy, err := StartBetaHeaderProxy(context.Background(), upstream.URL, logger)
	if err != nil {
		t.Fatalf("failed to start proxy: %v", err)
	}
	defer func() { _ = proxy.Stop(context.Background()) }()

	proxyURL := fmt.Sprintf("http://localhost:%d/v1/messages", proxy.Port())
	req, err := http.NewRequest(http.MethodPost, proxyURL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("authorization", "Bearer test-token")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request through proxy failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if receivedHeaders.Get("authorization") != "Bearer test-token" {
		t.Errorf("authorization header not preserved")
	}
	if receivedHeaders.Get("anthropic-version") != "2023-06-01" {
		t.Errorf("anthropic-version header not preserved")
	}
}

func TestBetaHeaderProxy_DefaultUpstream(t *testing.T) {
	logger := log.NewLogger(log.WithDevelopment())
	proxy, err := StartBetaHeaderProxy(context.Background(), "", logger)
	if err != nil {
		t.Fatalf("failed to start proxy: %v", err)
	}
	defer func() { _ = proxy.Stop(context.Background()) }()

	// Just verify the proxy starts and URL is well-formed.
	url := proxy.URL()
	if url == "" {
		t.Error("proxy URL is empty")
	}
	if proxy.Port() <= 0 {
		t.Error("proxy port is not positive")
	}
}
