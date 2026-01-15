package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/binaryphile/sofdevsim-2026/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	seed := flag.Int64("seed", 0, "Random seed for reproducibility (0 = use current time)")
	flag.Parse()

	// Negative seeds treated as 0 (random)
	if *seed < 0 {
		*seed = 0
	}

	app := tui.NewAppWithSeed(*seed)
	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
