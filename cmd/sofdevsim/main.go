package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/api"
	"github.com/binaryphile/sofdevsim-2026/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	seed := flag.Int64("seed", 0, "Random seed for reproducibility (0 = use current time)")
	port := flag.Int("port", 8080, "HTTP API port")
	clientMode := flag.Bool("client", false, "Use HTTP client mode (creates simulation via API)")
	localMode := flag.Bool("local", false, "Force local engine mode (for export, save/load, comparison)")
	flag.Parse()

	// Negative seeds treated as 0 (random)
	if *seed < 0 {
		*seed = 0
	}

	baseURL := fmt.Sprintf("http://localhost:%d", *port)

	// Auto-discovery: check if server already running
	existingServer := serverRunning(baseURL)

	// Determine mode: --local forces engine mode, --client forces HTTP mode
	// Otherwise, use client mode if server already running
	useClientMode := *clientMode || (existingServer && !*localMode)

	var registry api.SimRegistry
	if !existingServer {
		// Start our own server
		registry = api.NewSimRegistry()
		router := api.NewRouter(registry)
		go func() {
			addr := fmt.Sprintf(":%d", *port)
			if err := http.ListenAndServe(addr, router); err != nil {
				fmt.Fprintf(os.Stderr, "API server error: %v\n", err)
			}
		}()
		// Wait briefly for server to start
		time.Sleep(50 * time.Millisecond)
	}

	var app *tui.App

	if useClientMode {
		// HTTP client mode: create simulation via API
		client := tui.NewClient(baseURL)

		resp, err := client.CreateSimulation(*seed, "dora-strict")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create simulation: %v\n", err)
			os.Exit(1)
		}

		app = tui.NewAppWithClient(client, resp.Simulation)
	} else {
		// Local engine mode: direct registry access (supports export, save/load, comparison)
		if registry.SimRegistry.IsZero() {
			// No server started, create standalone registry
			registry = api.NewSimRegistry()
		}
		app = tui.NewAppWithRegistry(*seed, registry.SimRegistry)
	}

	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// serverRunning checks if a sofdevsim server is already running on the given URL.
// Verifies the service name to avoid connecting to unrelated servers.
func serverRunning(baseURL string) bool {
	client := &http.Client{Timeout: 100 * time.Millisecond}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var health struct {
		Service string `json:"service"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return false
	}

	return health.Service == "sofdevsim"
}
