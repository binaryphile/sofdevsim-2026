package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestApp_KeypressMutations_ReadOnlyModeGate — UC10 single-writer
// enforcement at the keypress layer. Closes #18913 (FU1 deferral from
// cycle #18911). Per plan §criterion 5.
//
// Five mutation actions are gated when coTenantWriteObserved is true:
//   - p / SetPolicy      → records PolicySetAttempted{Failed{Conflict}}
//   - s / StartSprint    → records SprintStartAttempted{Failed{Conflict}}
//   - a / AssignTicket   → records AssignmentAttempted{Failed{Conflict}}
//   - d / TryDecompose   → records DecomposeAttempted{Failed{Conflict}}
//   - 1-4 / SpendInvestment → status-message only (no Attempted event;
//                             SpendInvestment has no event in pre-cycle
//                             taxonomy and Decision C added only 2 new
//                             events, not 3)
//
// All 5 cases assert engine.ProjectionVersion() unchanged (no engine write).
// Precondition setup intentionally skipped — gate fires at top of engine-mode
// branch BEFORE precondition checks (per plan criterion 1 + 5).
func TestApp_KeypressMutations_ReadOnlyModeGate(t *testing.T) {
	type expectedEvent struct {
		// nil when status-message-only (SpendInvestment).
		isType func(InputEvent) bool
	}

	cases := []struct {
		name     string
		key      tea.KeyMsg
		expected expectedEvent
	}{
		{
			name: "SetPolicy",
			key:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")},
			expected: expectedEvent{
				isType: func(e InputEvent) bool { _, ok := e.(PolicySetAttempted); return ok },
			},
		},
		{
			name: "StartSprint",
			key:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")},
			expected: expectedEvent{
				isType: func(e InputEvent) bool { _, ok := e.(SprintStartAttempted); return ok },
			},
		},
		{
			name: "AssignTicket",
			key:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")},
			expected: expectedEvent{
				isType: func(e InputEvent) bool { _, ok := e.(AssignmentAttempted); return ok },
			},
		},
		{
			name: "TryDecompose",
			key:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")},
			expected: expectedEvent{
				isType: func(e InputEvent) bool { _, ok := e.(DecomposeAttempted); return ok },
			},
		},
		{
			name: "SpendInvestment_1",
			key:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")},
			expected: expectedEvent{
				isType: nil, // status-message only
			},
		},
	}

	for _, tc := range cases { // justified:CF
		t.Run(tc.name, func(t *testing.T) {
			app := NewAppWithSeed(42)
			app.openingAnimation = false // bypass animation block
			app.coTenantWriteObserved = true

			// Gate-order-vs-precondition proof note (/grade IMPL R1 F3):
			// NewAppWithSeed(42) leaves SprintNumber=0 → investment window
			// CLOSED → the SpendInvestment precondition (IsInvestmentWindowOpen)
			// would normally cause an early-return. The gate fires BEFORE
			// that precondition check, so the SpendInvestment case in this
			// test proves the gate runs first. If a future fixture change
			// opens the investment window at construction time, this implicit
			// proof would weaken; reassert by checking gate ordering directly.

			eng, ok := app.mode.GetLeft()
			if !ok {
				t.Fatal("expected engine mode")
			}
			versionBefore := eng.Engine.ProjectionVersion()
			eventsBefore := len(app.uiProjection.events)

			_, _ = app.handleKey(tc.key)

			// Assert no engine write (load-bearing UC10 contract).
			engAfter, _ := app.mode.GetLeft()
			if engAfter.Engine.ProjectionVersion() != versionBefore {
				t.Errorf("%s: engine ProjectionVersion advanced (before=%d, after=%d) — read-only gate failed to suppress engine write",
					tc.name, versionBefore, engAfter.Engine.ProjectionVersion())
			}

			// Status message must mention [READ-ONLY] (operator-visible signal).
			if !strings.Contains(app.statusMessage, "[READ-ONLY]") {
				t.Errorf("%s: statusMessage missing [READ-ONLY] indicator, got: %q", tc.name, app.statusMessage)
			}

			eventsAfter := app.uiProjection.events

			// For event-recording actions: assert exactly ONE new event of
			// the appropriate type was recorded with Failed{Conflict} outcome
			// whose Reason contains [READ-ONLY].
			if tc.expected.isType != nil {
				if len(eventsAfter) != eventsBefore+1 {
					t.Fatalf("%s: expected exactly 1 new InputEvent, got %d (before=%d, after=%d)",
						tc.name, len(eventsAfter)-eventsBefore, eventsBefore, len(eventsAfter))
				}
				last := eventsAfter[len(eventsAfter)-1]
				if !tc.expected.isType(last) {
					t.Errorf("%s: last recorded event wrong type: %T", tc.name, last)
				}
				if !outcomeIsReadOnlyConflict(last) {
					t.Errorf("%s: last event's Outcome not Failed{Conflict, contains [READ-ONLY]}: %+v", tc.name, last)
				}
			} else {
				// SpendInvestment case: status-message-only path. Assert NO
				// new InputEvent was recorded (/grade IMPL R1 F2 negative
				// assertion — catches accidental future event emission drift).
				if len(eventsAfter) != eventsBefore {
					t.Errorf("%s: status-message-only path recorded %d new InputEvent(s); expected 0",
						tc.name, len(eventsAfter)-eventsBefore)
				}
			}
		})
	}
}

// outcomeIsReadOnlyConflict returns true when the event carries a
// Failed{Category: Conflict, Reason: contains "[READ-ONLY]"} Outcome.
// Type-switches over the 4 event-recording variants this cycle gates.
func outcomeIsReadOnlyConflict(e InputEvent) bool {
	var outcome Outcome
	switch ev := e.(type) {
	case PolicySetAttempted:
		outcome = ev.Outcome
	case SprintStartAttempted:
		outcome = ev.Outcome
	case AssignmentAttempted:
		outcome = ev.Outcome
	case DecomposeAttempted:
		outcome = ev.Outcome
	default:
		return false
	}
	failed, ok := outcome.(Failed)
	if !ok {
		return false
	}
	return failed.Category == Conflict && strings.Contains(failed.Reason, "[READ-ONLY]")
}
