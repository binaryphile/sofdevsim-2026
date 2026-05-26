package tui

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/office"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

// TestResizeInvestigation drives the App through a WindowSizeMsg sequence
// and dumps each frame's bytes — investigation step for UC34 success-guarantee
// breach (post-resize partial redraw observed in tmux during UC40 closeout
// demo).
//
// Per plan §criterion 5: this test produces FALSIFIABLE evidence whether
// teatest reproduces the symptom. Failure mode being investigated:
//   - User observed only animation-delta row rendering; surrounding rows
//     blanked after tmux pane resize.
//
// teatest uses its own pseudo-terminal (no tmux), so it can reproduce the
// symptom ONLY if the root cause is in the model→bubbletea-renderer layer.
// If teatest does NOT reproduce, that points root cause at tmux+alt-screen
// SIGWINCH propagation (live-tmux-only — outcome (b) or (c) per plan).
func TestResizeInvestigation(t *testing.T) {
	app := NewAppWithSeed(42)
	tm := teatest.NewTestModel(
		t, app,
		teatest.WithInitialTermSize(120, 40),
	)

	// Sequence: large → small → narrow (triggers RenderOffice narrow-stub) →
	// medium → large. Matches the user-reported workflow of starting in a
	// split pane (narrow) then widening to full screen.
	tm.Send(tea.WindowSizeMsg{Width: 120, Height: 40})
	tm.Send(tea.WindowSizeMsg{Width: 60, Height: 20})
	tm.Send(tea.WindowSizeMsg{Width: 35, Height: 15}) // <40 → narrow-stub
	tm.Send(tea.WindowSizeMsg{Width: 75, Height: 30}) // 40-79 → simple layout
	tm.Send(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Quit; capture cumulative terminal output.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	out, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatal(err)
	}

	// Diagnostic: dump output size + a heuristic on blank-row detection.
	// teatest's FinalOutput is the cumulative byte stream including ANSI
	// cursor controls. A partial-redraw symptom would manifest as ANSI
	// erase-screen-below sequences leaving regions unfilled.
	output := string(out)
	t.Logf("Total bytes captured: %d", len(output))
	t.Logf("First 200 bytes:\n%s", truncForLog(output, 200))
	tailStart := len(output) - 200
	if tailStart < 0 {
		tailStart = 0
	}
	t.Logf("Last 200 bytes:\n%s", truncForLog(output[tailStart:], 200))

	// Heuristic: count consecutive blank rows after the final resize.
	// If the symptom reproduces, expect a streak of empty/whitespace lines.
	lines := strings.Split(output, "\n")
	maxBlankStreak := 0
	curStreak := 0
	for _, line := range lines {
		stripped := strings.TrimSpace(office.StripANSI(line))
		if stripped == "" {
			curStreak++
			if curStreak > maxBlankStreak {
				maxBlankStreak = curStreak
			}
		} else {
			curStreak = 0
		}
	}
	t.Logf("Max consecutive blank-row streak after ANSI strip: %d", maxBlankStreak)

	// Investigation observation (not an assertion — this test is a probe,
	// not a regression test). The "fix landed" criterion 5 will get its
	// own narrow-scoped test once root cause is localized.
}

func truncForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...[truncated]"
}

