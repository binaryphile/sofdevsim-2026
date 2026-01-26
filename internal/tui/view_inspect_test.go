package tui

import (
	"fmt"
	"testing"

	"github.com/acarl005/stripansi"
	tea "github.com/charmbracelet/bubbletea"
)

// TestView_Inspect lets Claude see the TUI output at each step
func TestView_Inspect(t *testing.T) {
	app := NewAppWithSeed(42)

	// Helper to send key and update app
	send := func(msg tea.Msg) {
		m, _ := app.Update(msg)
		app = m.(*App)
	}

	// Set window size
	send(tea.WindowSizeMsg{Width: 100, Height: 35})

	// Helper to show current view
	show := func(label string) {
		output := stripansi.Strip(app.View())
		fmt.Printf("\n=== %s ===\n%s\n", label, output)
	}

	show("1. INITIAL (Planning View)")

	// Navigate with Tab
	send(tea.KeyMsg{Type: tea.KeyTab})
	show("2. After TAB (Execution View)")

	// Back to planning
	send(tea.KeyMsg{Type: tea.KeyTab})
	send(tea.KeyMsg{Type: tea.KeyTab})
	send(tea.KeyMsg{Type: tea.KeyTab})

	// Select ticket
	send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	show("3. After j j (Third ticket selected)")

	// Show lessons
	send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	show("4. After h (Lessons panel visible)")

	// Start sprint
	send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	show("5. After s (Sprint started - Execution view)")

	// Show UIProjection state
	state := app.uiProjection.State()
	fmt.Printf("\n=== UIProjection State ===\n")
	fmt.Printf("CurrentView: %v\n", state.CurrentView)
	fmt.Printf("SelectedTicket: %q\n", state.SelectedTicket)
	fmt.Printf("LessonVisible: %v\n", state.LessonVisible)
	fmt.Printf("ErrorMessage: %q\n", state.ErrorMessage)
}
