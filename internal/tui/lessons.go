package tui

import (
	"fmt"
	"strings"

	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/sofdevsim-2026/internal/lessons"
	"github.com/charmbracelet/lipgloss"
)

// Re-export types from lessons package for convenience within tui.
type (
	LessonID    = lessons.LessonID
	Lesson      = lessons.Lesson
	LessonState = lessons.State
)

// SprintOption is a type alias for optional sprint state.
type SprintOption = option.Basic[SprintState]

// Re-export constants.
const (
	LessonOrientation           = lessons.Orientation
	LessonUnderstanding         = lessons.Understanding
	LessonFeverChart            = lessons.FeverChart
	LessonDORAMetrics           = lessons.DORAMetrics
	LessonPolicyComparison      = lessons.PolicyComparison
	LessonVarianceExpected      = lessons.VarianceExpected
	LessonPhaseProgress         = lessons.PhaseProgress
	LessonVarianceAnalysis      = lessons.VarianceAnalysis
	LessonUncertaintyConstraint = lessons.UncertaintyConstraint
	TotalLessons                = lessons.TotalLessons
)

// toViewContext converts tui.View to lessons.ViewContext.
func toViewContext(v View) lessons.ViewContext {
	switch v {
	case ViewPlanning:
		return lessons.ViewPlanning
	case ViewExecution:
		return lessons.ViewExecution
	case ViewMetrics:
		return lessons.ViewMetrics
	case ViewComparison:
		return lessons.ViewComparison
	default:
		return lessons.ViewPlanning
	}
}

// TriggerState re-exports lessons.TriggerState for TUI use.
type TriggerState = lessons.TriggerState

// HasRedBufferWithLowTicketFromStrings re-exports the client-mode trigger detector.
// Two implementations exist in lessons package to avoid import cycles:
//   - HasRedBufferWithLowTicket: Uses model.FeverStatus + []model.Ticket (engine mode)
//   - HasRedBufferWithLowTicketFromStrings: Uses bool + []string (client mode)
//
// Client mode can't use model types because lessons would need to import tui
// to get TicketState, creating a cycle (tui already imports lessons).
var HasRedBufferWithLowTicketFromStrings = lessons.HasRedBufferWithLowTicketFromStrings

// BufferRedThreshold is the percentage at which buffer is considered "red" (danger zone).
const BufferRedThreshold = 66

// BuildTriggersFromClientState constructs TriggerState from client-mode state.
// This is a pure function: (sprintOpt, activeTickets) → TriggerState.
// Used by both app.go and tests to avoid duplicating trigger logic.
func BuildTriggersFromClientState(sprintOption SprintOption, activeTickets []TicketState) TriggerState {
	var triggers TriggerState
	sprintOption.Call(func(sprint SprintState) {
		pctUsed := (sprint.BufferConsumed / sprint.BufferDays) * 100
		isRed := pctUsed >= BufferRedThreshold
		var understandings []string
		for _, t := range activeTickets {
			understandings = append(understandings, t.Understanding)
		}
		triggers.HasRedBufferWithLowTicket = lessons.HasRedBufferWithLowTicketFromStrings(isRed, understandings)
	})
	return triggers
}

// SelectLesson wraps lessons.Select for TUI use.
func SelectLesson(view View, state LessonState, hasActiveSprint bool, hasComparisonResult bool, triggers TriggerState) Lesson {
	return lessons.Select(toViewContext(view), state, hasActiveSprint, hasComparisonResult, triggers)
}

// lessonsPanel renders the lesson panel with current lesson content.
func (a *App) lessonsPanel(lesson Lesson) string {
	title := TitleStyle.Render("💡 " + lesson.Title)
	progress := MutedStyle.Render(fmt.Sprintf("Progress: %d/%d concepts", a.lessonState.SeenCount(), TotalLessons))

	// Build tips section
	var tipsSection string
	if len(lesson.Tips) > 0 {
		tips := make([]string, len(lesson.Tips))
		for i, tip := range lesson.Tips {
			tips[i] = MutedStyle.Render("• " + tip)
		}
		tipsSection = strings.Join(tips, "\n")
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		lesson.Content,
		"",
		tipsSection,
		"",
		progress,
	)

	return BoxStyle.Width(a.width/3 - 2).BorderForeground(ColorSecondary).Render(content)
}
