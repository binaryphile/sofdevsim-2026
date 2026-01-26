package tui

import (
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"
)

func init() {
	// Force ASCII color profile for consistent golden file output
	lipgloss.SetColorProfile(termenv.Ascii)
}

// TestView_PlanningInitial captures the initial planning view for golden file comparison.
func TestView_PlanningInitial(t *testing.T) {
	app := NewAppWithSeed(42)
	tm := teatest.NewTestModel(
		t, app,
		teatest.WithInitialTermSize(120, 40),
	)

	// Quit immediately to capture initial view
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	out, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatal(err)
	}
	teatest.RequireEqualOutput(t, out)
}
