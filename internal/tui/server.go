package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/api"
)

// ServerConfig holds configuration for server discovery and startup.
type ServerConfig struct {
	Port    int
	Timeout time.Duration
}

// DefaultServerConfig returns the default server configuration.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Port:    8080,
		Timeout: 5 * time.Second,
	}
}

// HealthResponse mirrors api.HealthResponse for client-side decoding.
type HealthResponse struct {
	Service string `json:"service"`
	Version string `json:"version"`
}

// discoverServer checks if a sofdevsim server is running at the given port.
// Returns true only if /health returns {"service":"sofdevsim"}.
func discoverServer(port int) bool {
	url := fmt.Sprintf("http://localhost:%d/health", port)
	client := &http.Client{Timeout: 1 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return false
	}

	return health.Service == "sofdevsim"
}

// EmbeddedServer manages an embedded API server instance.
//
// Pointer receiver: wraps *http.Server; Shutdown() requires the same instance
// that called ListenAndServe() to properly close active connections.
type EmbeddedServer struct {
	server   *http.Server
	registry api.SimRegistry
	port     int
}

// startEmbeddedServer starts an embedded API server on the given port.
// Returns an EmbeddedServer for later shutdown.
func startEmbeddedServer(port int) (*EmbeddedServer, error) {
	registry := api.NewSimRegistry()
	router := api.NewRouter(registry)

	addr := fmt.Sprintf(":%d", port)
	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Check if port is available
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("port %d in use: %w", port, err)
	}

	// Start server in goroutine
	go func() {
		server.Serve(listener)
	}()

	return &EmbeddedServer{
		server:   server,
		registry: registry,
		port:     port,
	}, nil
}

// Shutdown gracefully shuts down the embedded server.
func (e *EmbeddedServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	return e.server.Shutdown(ctx)
}

// Registry returns the server's registry for shared access.
func (e *EmbeddedServer) Registry() api.SimRegistry {
	return e.registry
}

// Port returns the server's port.
func (e *EmbeddedServer) Port() int {
	return e.port
}

// waitForServer polls the health endpoint until the server is ready.
func waitForServer(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if discoverServer(port) {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("server not ready after %v", timeout)
}

// DiscoverOrStart checks for an existing server and starts one if needed.
// Returns the base URL and an optional EmbeddedServer (nil if connected to existing).
func DiscoverOrStart(port int) (string, *EmbeddedServer, error) {
	baseURL := fmt.Sprintf("http://localhost:%d", port)

	// Check for existing server
	if discoverServer(port) {
		return baseURL, nil, nil
	}

	// Try to start embedded server
	embedded, err := startEmbeddedServer(port)
	if err != nil {
		return "", nil, fmt.Errorf("port %d in use by another service. Try: sofdevsim -port %d", port, port+1)
	}

	// Wait for server to be ready
	if err := waitForServer(port, 5*time.Second); err != nil {
		embedded.Shutdown()
		return "", nil, err
	}

	return baseURL, embedded, nil
}
