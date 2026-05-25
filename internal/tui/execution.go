package tui

import (
	"fmt"
	"strings"

	"github.com/binaryphile/fluentfp/slice"
	"github.com/charmbracelet/lipgloss"
)

// Calculation: ExecutionVM → string
func renderExecution(vm ExecutionVM) string {
	sprintBar := renderSprintProgress(vm.Sprint)
	activeWork := renderActiveWork(vm.ActiveWork)
	fever := renderFever(vm.Fever)
	events := renderEvents(vm.Events)
	phaseQueues := renderPhaseQueues(vm.PhaseQueues) // UC38 Phase Queues panel
	office := RenderOffice(vm.OfficeState, vm.DevNames, vm.Width, vm.Height)

	top := BoxStyle.Width(vm.Width - 2).Render(sprintBar)
	middle := BoxStyle.Width(vm.Width - 2).Render(activeWork)

	// Bottom row: Fever | Phase Queues | Events (three columns of equal width).
	colWidth := vm.Width/3 - 2
	bottomLeft := BoxStyle.Width(colWidth).Render(fever)
	bottomCenter := BoxStyle.Width(colWidth).Render(phaseQueues)
	bottomRight := BoxStyle.Width(colWidth).Render(events)
	bottom := lipgloss.JoinHorizontal(lipgloss.Top, bottomLeft, bottomCenter, bottomRight)

	return lipgloss.JoinVertical(lipgloss.Left, top, middle, bottom, office)
}

// Calculation: []PhaseQueueRow → string (UC38 Phase Queues panel).
// One row per phase showing "Phase: depth/cap [bar]" — head-of-line
// blocking is observable when depth > cap-attainable or queues backlog.
func renderPhaseQueues(rows []PhaseQueueRow) string {
	title := TitleStyle.Render("Phase Queues")

	if len(rows) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, MutedStyle.Render("No data"))
	}

	formatRow := func(row PhaseQueueRow) string {
		// 12-char fill bar; "█" per active+queued ticket, capped to 12.
		barLen := row.Depth
		if barLen > 12 {
			barLen = 12
		}
		bar := strings.Repeat("█", barLen) + strings.Repeat("·", 12-barLen)
		return fmt.Sprintf("%-9s %2d/%-3s %s", row.Phase, row.Depth, row.CapDisplay, bar)
	}
	lines := slice.From(rows).ToString(formatRow)
	content := strings.Join(lines, "\n")

	return lipgloss.JoinVertical(lipgloss.Left, title, content)
}

// Calculation: SprintProgressVM → string
func renderSprintProgress(vm SprintProgressVM) string {
	if !vm.HasSprint {
		return MutedStyle.Render("No active sprint")
	}

	bar := RenderProgressBar(vm.Progress, 50)
	title := TitleStyle.Render(fmt.Sprintf("Sprint %d", vm.SprintNumber))
	info := fmt.Sprintf("Day %d/%d  %s  %.0f%%",
		vm.DaysElapsed,
		vm.DurationDays,
		bar,
		vm.Progress*100,
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, info)
}

// Calculation: []ActiveWorkRowVM → string
func renderActiveWork(rows []ActiveWorkRowVM) string {
	title := TitleStyle.Render("Active Work")

	// formatWorkRow formats an active work row for display.
	formatWorkRow := func(row ActiveWorkRowVM) string {
		if row.IsIdle {
			return fmt.Sprintf("%-8s %s", row.DevName, MutedStyle.Render("[idle]"))
		}
		bar := RenderProgressBar(row.Progress, 20)
		return fmt.Sprintf("%-8s → %-10s %s %.0f%% (%s)",
			row.DevName, row.TicketID, bar, row.Progress*100, row.Phase)
	}
	lines := slice.From(rows).ToString(formatWorkRow)

	content := strings.Join(lines, "\n")
	if len(lines) == 0 {
		content = MutedStyle.Render("No active work")
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, content)
}

// Calculation: FeverVM → string
func renderFever(vm FeverVM) string {
	title := TitleStyle.Render("Fever Chart")

	if !vm.HasSprint {
		return lipgloss.JoinVertical(lipgloss.Left, title, MutedStyle.Render("No active sprint"))
	}

	statusStyle := FeverColor(vm.BufferPct)
	info := fmt.Sprintf("Work: %.0f%%  Buffer: %.0f%%  Ratio: %s %s",
		vm.WorkPct,
		vm.BufferPct,
		vm.RatioStr,
		statusStyle.Render(feverEmojiFromString(vm.Zone)),
	)

	remaining := fmt.Sprintf("%.1f / %.1f days remaining",
		vm.Remaining,
		vm.TotalBuffer,
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, info, MutedStyle.Render(remaining))
}

// Calculation: []EventVM → string
func renderEvents(events []EventVM) string {
	title := TitleStyle.Render("Events")

	// formatEvent formats and truncates an event for display.
	formatEvent := func(evt EventVM) string {
		return MutedStyle.Render(truncate(fmt.Sprintf("Day %d: %s", evt.Day, evt.Message), 40))
	}
	rows := slice.From(events).ToString(formatEvent)

	content := strings.Join(rows, "\n")
	if len(rows) == 0 {
		content = MutedStyle.Render("No events yet")
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, content)
}
