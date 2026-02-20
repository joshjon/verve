package worker

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/joshjon/kit/log"
)

const defaultAnthropicBaseURL = "https://api.anthropic.com"

// BetaHeaderProxy is a reverse proxy that strips anthropic-beta headers from
// requests before forwarding them to the upstream Anthropic API. This is needed
// when using Bedrock proxies that don't support beta features — the Claude Code
// CLI sends these headers by default, causing errors with incompatible backends.
type BetaHeaderProxy struct {
	server   *http.Server
	listener net.Listener
	logger   log.Logger
}

// StartBetaHeaderProxy starts a local HTTP reverse proxy that strips
// anthropic-beta headers and forwards requests to the given upstream URL.
// If upstream is empty, it defaults to https://api.anthropic.com.
// The proxy listens on a random available port on 0.0.0.0 so agent containers
// can reach it via host.docker.internal.
func StartBetaHeaderProxy(ctx context.Context, upstream string, logger log.Logger) (*BetaHeaderProxy, error) {
	if upstream == "" {
		upstream = defaultAnthropicBaseURL
	}

	target, err := url.Parse(upstream)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream URL %q: %w", upstream, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Preserve the original Director and add header stripping.
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Del("anthropic-beta")
		req.Host = target.Host
	}

	// Listen on all interfaces so agent containers can reach the proxy via
	// host.docker.internal on the assigned port.
	lc := net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", "0.0.0.0:0") //nolint:gosec // must bind all interfaces for Docker container access
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	server := &http.Server{
		Handler:           proxy,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Error("beta header proxy error", "error", err)
		}
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	logger.Info("beta header proxy started", "port", port, "upstream", upstream)

	return &BetaHeaderProxy{
		server:   server,
		listener: listener,
		logger:   logger,
	}, nil
}

// URL returns the base URL of the proxy (e.g. http://host.docker.internal:12345).
// This is intended to be passed to agent containers as ANTHROPIC_BASE_URL so that
// Claude Code CLI routes API requests through the proxy.
func (p *BetaHeaderProxy) URL() string {
	port := p.listener.Addr().(*net.TCPAddr).Port
	return fmt.Sprintf("http://host.docker.internal:%d", port)
}

// Port returns the port the proxy is listening on.
func (p *BetaHeaderProxy) Port() int {
	return p.listener.Addr().(*net.TCPAddr).Port
}

// Stop gracefully shuts down the proxy.
func (p *BetaHeaderProxy) Stop(ctx context.Context) error {
	p.logger.Info("stopping beta header proxy")
	return p.server.Shutdown(ctx)
}
