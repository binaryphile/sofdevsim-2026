package tui

import (
	"strings"
	"testing"

	"github.com/acarl005/stripansi"
	tea "github.com/charmbracelet/bubbletea"
)

// TestFeverPanel_DisplaysMetrics verifies fever chart shows Work%, Buffer%, Ratio, Zone.
// Per design.md §"Testing via Event Replay": TUI is a projection, test pure transformations.
func TestFeverPanel_DisplaysMetrics(t *testing.T) {
	app := NewAppWithSeed(42)

	// Helper to send key and update app
	send := func(msg tea.Msg) {
		m, _ := app.Update(msg)
		app = m.(*App)
	}

	// Set window size
	send(tea.WindowSizeMsg{Width: 100, Height: 35})

	// Start sprint to activate fever panel
	send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})

	// Get fever panel output via build + render split
	output := stripansi.Strip(renderFever(app.buildExecutionVM().Fever))

	// Verify expected elements present
	checks := []struct {
		contains string
		desc     string
	}{
		{"Fever Chart", "title"},
		{"Work:", "work progress label"},
		{"Buffer:", "buffer consumed label"},
		{"Ratio:", "ratio label"},
		{"days remaining", "buffer remaining"},
	}

	for _, check := range checks {
		if !strings.Contains(output, check.contains) {
			t.Errorf("feverPanel missing %s: expected %q in output:\n%s", check.desc, check.contains, output)
		}
	}
}

// TestFeverPanel_NoSprint shows muted message when no sprint active.
func TestFeverPanel_NoSprint(t *testing.T) {
	app := NewAppWithSeed(42)

	// Set window size but don't start sprint
	m, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 35})
	app = m.(*App)

	output := stripansi.Strip(renderFever(app.buildExecutionVM().Fever))

	if !strings.Contains(output, "No active sprint") {
		t.Errorf("expected 'No active sprint' in output:\n%s", output)
	}
}

// TestFeverPanel_ZoneEmoji verifies zone indicator appears in output.
func TestFeverPanel_ZoneEmoji(t *testing.T) {
	app := NewAppWithSeed(42)

	send := func(msg tea.Msg) {
		m, _ := app.Update(msg)
		app = m.(*App)
	}

	send(tea.WindowSizeMsg{Width: 100, Height: 35})
	send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}) // Start sprint

	output := renderFever(app.buildExecutionVM().Fever) // Keep ANSI to check emoji

	// Should have one of the zone emojis
	hasZone := strings.Contains(output, "🟢") ||
		strings.Contains(output, "🟡") ||
		strings.Contains(output, "🔴")

	if !hasZone {
		t.Errorf("expected zone emoji (🟢🟡🔴) in output:\n%s", stripansi.Strip(output))
	}
}
