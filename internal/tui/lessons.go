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
	LessonConstraintHunt        = lessons.ConstraintHunt
	LessonExploitFirst          = lessons.ExploitFirst
	TotalLessons                = lessons.TotalLessons
)

// toViewContext converts tui.View to lessons.ViewContext.
// Calculation: View → ViewContext
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
// Calculation: SimulationState → TriggerState
// Used by both app.go and tests to avoid duplicating trigger logic.
func BuildTriggersFromClientState(state SimulationState) TriggerState {
	var triggers TriggerState

	// UC19: Red buffer with LOW ticket
	state.SprintOption.Call(func(sprint SprintState) {
		// Boundary defense: avoid division by zero
		if sprint.BufferDays <= 0 {
			return
		}
		pctUsed := (sprint.BufferConsumed / sprint.BufferDays) * 100
		isRed := pctUsed >= BufferRedThreshold
		var understandings []string
		for _, t := range state.ActiveTickets {
			understandings = append(understandings, t.Understanding)
		}
		triggers.HasRedBufferWithLowTicket = lessons.HasRedBufferWithLowTicketFromStrings(isRed, understandings)
	})

	// UC20: Queue imbalance
	triggers.HasQueueImbalance = HasQueueImbalanceFromTickets(state.ActiveTickets)

	// UC21: High child variance
	triggers.HasHighChildVariance = HasHighChildVarianceFromTickets(state.CompletedTickets)

	return triggers
}

// HasQueueImbalanceFromTickets detects UC20 trigger (client mode).
// Calculation: []TicketState → bool
// Uses Phase field to calculate queue depths.
func HasQueueImbalanceFromTickets(activeTickets []TicketState) bool {
	depths := make(map[string]int)
	for _, t := range activeTickets {
		if t.Phase != "" {
			depths[t.Phase]++
		}
	}
	if len(depths) == 0 {
		return false
	}
	var sum int
	for _, d := range depths {
		sum += d
	}
	avg := float64(sum) / float64(len(depths))
	for _, d := range depths {
		if float64(d) > 2*avg {
			return true
		}
	}
	return false
}

// HasHighChildVarianceFromTickets detects UC21 trigger (client mode).
// Calculation: []TicketState → bool
func HasHighChildVarianceFromTickets(completedTickets []TicketState) bool {
	byID := make(map[string]TicketState)
	for _, t := range completedTickets {
		byID[t.ID] = t
	}
	for _, parent := range completedTickets {
		if len(parent.ChildIDs) == 0 {
			continue
		}
		for _, childID := range parent.ChildIDs {
			child, ok := byID[childID]
			if !ok {
				continue // child not yet completed
			}
			// Boundary defense: skip if EstimatedDays is zero or negative
			if child.EstimatedDays <= 0 {
				continue
			}
			ratio := child.ActualDays / child.EstimatedDays
			if ratio > 1.3 {
				return true
			}
		}
	}
	return false
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
