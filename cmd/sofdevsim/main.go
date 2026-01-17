package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/binaryphile/sofdevsim-2026/internal/api"
	"github.com/binaryphile/sofdevsim-2026/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	seed := flag.Int64("seed", 0, "Random seed for reproducibility (0 = use current time)")
	apiPort := flag.Int("api-port", 8080, "HTTP API port")
	flag.Parse()

	// Negative seeds treated as 0 (random)
	if *seed < 0 {
		*seed = 0
	}

	// Start HTTP API server in goroutine
	registry := api.NewSimRegistry()
	router := api.NewRouter(registry)
	go func() {
		addr := fmt.Sprintf(":%d", *apiPort)
		if err := http.ListenAndServe(addr, router); err != nil {
			fmt.Fprintf(os.Stderr, "API server error: %v\n", err)
		}
	}()

	// Run TUI on main goroutine (Bubbletea requirement)
	app := tui.NewAppWithSeed(*seed)
	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
