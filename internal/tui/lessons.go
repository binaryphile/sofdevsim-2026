package tui

import (
	"fmt"
	"strings"

	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/sofdevsim-2026/internal/lessons"
	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
	"github.com/charmbracelet/lipgloss"
)

// Re-export types from lessons package for convenience within tui.
type (
	LessonID    = lessons.LessonID
	Lesson      = lessons.Lesson
	LessonState = lessons.State
)

// SprintOption is a type alias for optional sprint state.
// Per FP Guide §14: Option types for "value may not exist" semantics.
// Sprint is absent between sprints or before first sprint starts.
type SprintOption = option.Basic[SprintState]

// ComparisonOption is a type alias for optional comparison result.
// Per FP Guide §14: Option types for "value may not exist" semantics.
// Comparison result is absent until user runs comparison mode.
type ComparisonOption = option.Basic[metrics.ComparisonResult]

// ComparisonSummary re-exports lessons.ComparisonSummary for TUI use.
type ComparisonSummary = lessons.ComparisonSummary

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
	LessonFiveFocusing          = lessons.FiveFocusing
	LessonManagerTakeaways      = lessons.ManagerTakeaways
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

	// UC22: Sprint count for Five Focusing Steps
	triggers.SprintCount = state.SprintNumber

	// UC19: Red buffer with LOW ticket
	checkRedBufferWithLowTicket := option.Lift(func(sprint SprintState) {
		// Boundary defense: avoid division by zero
		if sprint.BufferDays <= 0 {
			return
		}
		pctUsed := (sprint.BufferConsumed / sprint.BufferDays) * 100
		isRed := pctUsed >= BufferRedThreshold
		understandings := slice.From(state.ActiveTickets).ToString(TicketState.GetUnderstanding)
		triggers.HasRedBufferWithLowTicket = lessons.HasRedBufferWithLowTicketFromStrings(isRed, understandings)
	})
	checkRedBufferWithLowTicket(state.SprintOption)

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

// BuildComparisonSummary converts option.Basic[metrics.ComparisonResult] to lessons.ComparisonSummary.
// Calculation: ComparisonOption → ComparisonSummary
// Returns empty ComparisonSummary{} when option is not-ok (boundary defense).
func BuildComparisonSummary(opt ComparisonOption) ComparisonSummary {
	if !opt.IsOk() {
		return ComparisonSummary{}
	}

	result, _ := opt.Get()

	// Determine winner policy string
	var winnerPolicy string
	switch result.OverallWinner {
	case model.PolicyDORAStrict:
		winnerPolicy = lessons.WinnerDORAStrict
	case model.PolicyTameFlowCognitive:
		winnerPolicy = lessons.WinnerTameFlowCognitive
	default:
		winnerPolicy = lessons.WinnerTie
	}

	// Convert lead time from Duration to days (float64)
	leadTimeA := result.ResultsA.FinalMetrics.LeadTimeAvg.Hours() / 24
	leadTimeB := result.ResultsB.FinalMetrics.LeadTimeAvg.Hours() / 24

	return ComparisonSummary{
		HasResult:     true,
		WinnerPolicy:  winnerPolicy,
		LeadTimeA:     leadTimeA,
		LeadTimeB:     leadTimeB,
		LeadTimeDelta: leadTimeA - leadTimeB,
		CFRA:          result.ResultsA.FinalMetrics.ChangeFailRate,
		CFRB:          result.ResultsB.FinalMetrics.ChangeFailRate,
		CFRDelta:      result.ResultsA.FinalMetrics.ChangeFailRate - result.ResultsB.FinalMetrics.ChangeFailRate,
		WinsA:         result.WinsA,
		WinsB:         result.WinsB,
	}
}

// SelectLesson wraps lessons.Select for TUI use.
// Calculation: (View, LessonState, bool, bool, TriggerState, ComparisonSummary) → Lesson
func SelectLesson(view View, state LessonState, hasActiveSprint bool, hasComparisonResult bool, triggers TriggerState, comparison ComparisonSummary) Lesson {
	return lessons.Select(toViewContext(view), state, hasActiveSprint, hasComparisonResult, triggers, comparison)
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
