// Package proxy implements the HTTP forward proxy for LLM API interception.
//
// Purpose: Route LLM API calls through local proxy to extract usage data.
// Public API: Server, Config, Handler
// Usage: Create Server with Config, call Start() to begin listening.
package proxy

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"plarix-action/internal/ledger"
	"plarix-action/internal/providers/openai"
)

// Config holds proxy configuration.
type Config struct {
	Providers            []string           // e.g., ["openai", "anthropic", "openrouter"]
	OnEntry              func(ledger.Entry) // Callback for each recorded entry
	StreamUsageInjection bool               // Opt-in for OpenAI stream usage injection
}

// Server is the HTTP forward proxy server.
type Server struct {
	config     Config
	listener   net.Listener
	httpServer *http.Server
	mu         sync.Mutex
	started    bool
}

// providerTargets maps provider names to their API base URLs.
var providerTargets = map[string]string{
	"openai":     "https://api.openai.com",
	"anthropic":  "https://api.anthropic.com",
	"openrouter": "https://openrouter.ai",
}

// NewServer creates a new proxy server.
func NewServer(config Config) *Server {
	s := &Server{config: config}
	s.httpServer = &http.Server{
		Handler:      s,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	return s
}

// Start begins listening on a random available port.
// Returns the port number.
func (s *Server) Start() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return 0, fmt.Errorf("server already started")
	}

	var err error
	s.listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("listen: %w", err)
	}

	port := s.listener.Addr().(*net.TCPAddr).Port
	s.started = true

	go s.httpServer.Serve(s.listener)

	return port, nil
}

// Stop shuts down the server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}
	s.started = false
	return s.httpServer.Close()
}

// Port returns the listening port, or 0 if not started.
func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener == nil {
		return 0
	}
	return s.listener.Addr().(*net.TCPAddr).Port
}

// ServeHTTP handles incoming proxy requests.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract provider from path prefix: /openai/v1/... -> openai
	pathParts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)
	if len(pathParts) < 1 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	provider := pathParts[0]
	targetBase, ok := providerTargets[provider]
	if !ok {
		http.Error(w, fmt.Sprintf("unknown provider: %s", provider), http.StatusBadRequest)
		return
	}

	// Reconstruct target path
	targetPath := "/"
	if len(pathParts) > 1 {
		targetPath = "/" + pathParts[1]
	}

	targetURL, _ := url.Parse(targetBase)

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.URL.Path = targetPath
			req.Host = targetURL.Host
		},
		ModifyResponse: func(resp *http.Response) error {
			return s.handleResponse(provider, targetPath, resp)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, fmt.Sprintf("proxy error: %v", err), http.StatusBadGateway)
		},
	}

	proxy.ServeHTTP(w, r)
}

// handleResponse processes the API response to extract usage data.
func (s *Server) handleResponse(provider, endpoint string, resp *http.Response) error {
	// Only process successful responses
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}

	// Check content type - only process JSON
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return nil
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil // Don't fail the request if we can't read
	}
	resp.Body.Close()

	// Create new reader for the client
	resp.Body = io.NopCloser(strings.NewReader(string(body)))
	resp.ContentLength = int64(len(body))

	// Parse usage based on provider
	entry := s.parseUsage(provider, endpoint, body)
	if s.config.OnEntry != nil {
		s.config.OnEntry(entry)
	}

	return nil
}

// parseUsage extracts usage data from the response body.
func (s *Server) parseUsage(provider, endpoint string, body []byte) ledger.Entry {
	entry := ledger.Entry{
		Provider:  provider,
		Endpoint:  endpoint,
		Streaming: false,
	}

	switch provider {
	case "openai":
		openai.ParseResponse(body, &entry)
	case "anthropic":
		// TODO: Milestone 6
		entry.CostKnown = false
		entry.UnknownReason = "anthropic parser not implemented"
	case "openrouter":
		// TODO: Milestone 6 - OpenRouter uses OpenAI-compatible format
		entry.CostKnown = false
		entry.UnknownReason = "openrouter parser not implemented"
	default:
		entry.CostKnown = false
		entry.UnknownReason = "unsupported provider"
	}

	return entry
}
