package tui

import (
	"fmt"
	"strings"

	"github.com/binaryphile/sofdevsim-2026/internal/lessons"
	"github.com/charmbracelet/lipgloss"
)

// Re-export types from lessons package for convenience within tui.
type (
	LessonID    = lessons.LessonID
	Lesson      = lessons.Lesson
	LessonState = lessons.State
)

// Re-export constants.
const (
	LessonOrientation      = lessons.Orientation
	LessonUnderstanding    = lessons.Understanding
	LessonFeverChart       = lessons.FeverChart
	LessonDORAMetrics      = lessons.DORAMetrics
	LessonPolicyComparison = lessons.PolicyComparison
	LessonVarianceExpected = lessons.VarianceExpected
	LessonPhaseProgress    = lessons.PhaseProgress
	LessonVarianceAnalysis = lessons.VarianceAnalysis
	TotalLessons           = lessons.TotalLessons
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

// SelectLesson wraps lessons.Select for TUI use.
func SelectLesson(view View, state LessonState, hasActiveSprint bool, hasComparisonResult bool) Lesson {
	return lessons.Select(toViewContext(view), state, hasActiveSprint, hasComparisonResult)
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
