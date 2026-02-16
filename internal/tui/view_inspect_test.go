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

// TestView_InspectLessons shows each triggered lesson (UC19-UC23) rendered in TUI panel
func TestView_InspectLessons(t *testing.T) {
	// Helper to set up app and render with specific lesson state
	showRenderedLesson := func(label string, lessonState LessonState, view View, triggers TriggerState, comparison ComparisonSummary) {
		app := NewAppWithSeed(42)
		app.lessonState = lessonState.WithVisible(true)
		app.currentView = view
		app.width = 100
		app.height = 40

		// Get the lesson that would be selected
		lesson := SelectLesson(view, app.lessonState, true, comparison.HasResult, triggers, comparison)

		// Render the lesson panel
		panel := stripansi.Strip(renderLesson(app.buildLessonVM(lesson)))

		fmt.Printf("\n=== %s ===\n", label)
		fmt.Printf("Lesson ID: %s\n", lesson.ID)
		fmt.Printf("Rendered Panel:\n%s\n", panel)
	}

	// UC19: UncertaintyConstraint - red buffer + LOW ticket
	t.Run("UC19_UncertaintyConstraint", func(t *testing.T) {
		state := LessonState{}
		state = state.WithSeen(LessonOrientation)

		triggers := TriggerState{HasRedBufferWithLowTicket: true}
		showRenderedLesson("UC19: UncertaintyConstraint", state, ViewExecution, triggers, ComparisonSummary{})
	})

	// UC20: ConstraintHunt - queue imbalance + UC19 seen
	t.Run("UC20_ConstraintHunt", func(t *testing.T) {
		state := LessonState{}
		state = state.WithSeen(LessonOrientation)
		state = state.WithSeen(LessonUncertaintyConstraint)

		triggers := TriggerState{HasQueueImbalance: true}
		showRenderedLesson("UC20: ConstraintHunt", state, ViewExecution, triggers, ComparisonSummary{})
	})

	// UC21: ExploitFirst - high child variance + UC19 seen
	t.Run("UC21_ExploitFirst", func(t *testing.T) {
		state := LessonState{}
		state = state.WithSeen(LessonOrientation)
		state = state.WithSeen(LessonUncertaintyConstraint)

		triggers := TriggerState{HasHighChildVariance: true}
		showRenderedLesson("UC21: ExploitFirst", state, ViewExecution, triggers, ComparisonSummary{})
	})

	// UC22: FiveFocusing - 3+ sprints + UC20/21 seen
	t.Run("UC22_FiveFocusing", func(t *testing.T) {
		state := LessonState{}
		state = state.WithSeen(LessonOrientation)
		state = state.WithSeen(LessonUncertaintyConstraint)
		state = state.WithSeen(LessonConstraintHunt)

		triggers := TriggerState{SprintCount: 3}
		showRenderedLesson("UC22: FiveFocusing", state, ViewExecution, triggers, ComparisonSummary{})
	})

	// UC23: ManagerTakeaways - comparison result + UC22 seen
	t.Run("UC23_ManagerTakeaways", func(t *testing.T) {
		state := LessonState{}
		state = state.WithSeen(LessonOrientation)
		state = state.WithSeen(LessonFiveFocusing)

		comparison := ComparisonSummary{
			HasResult:     true,
			WinnerPolicy:  "TameFlow-Cognitive",
			LeadTimeA:     5.2,
			LeadTimeB:     3.8,
			LeadTimeDelta: 1.4,
			CFRA:          0.15,
			CFRB:          0.08,
			CFRDelta:      0.07,
			WinsA:         1,
			WinsB:         3,
		}
		showRenderedLesson("UC23: ManagerTakeaways", state, ViewComparison, TriggerState{}, comparison)
	})
}
