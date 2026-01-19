package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (a *App) executionView() string {
	// Sprint progress
	sprintBar := a.sprintProgress()

	// Active work
	activeWork := a.activeWorkPanel()

	// Fever chart
	fever := a.feverPanel()

	// Events log
	events := a.eventsPanel()

	// Layout
	top := BoxStyle.Width(a.width - 2).Render(sprintBar)
	middle := BoxStyle.Width(a.width - 2).Render(activeWork)

	bottomLeft := BoxStyle.Width(a.width/2 - 2).Render(fever)
	bottomRight := BoxStyle.Width(a.width/2 - 2).Render(events)
	bottom := lipgloss.JoinHorizontal(lipgloss.Top, bottomLeft, bottomRight)

	return lipgloss.JoinVertical(lipgloss.Left, top, middle, bottom)
}

func (a *App) sprintProgress() string {
	sim := a.engine.Sim()
	sprint, ok := sim.CurrentSprintOption.Get()
	if !ok {
		return MutedStyle.Render("No active sprint")
	}

	progress := sprint.ProgressPct(sim.CurrentTick)
	bar := RenderProgressBar(progress, 50)

	title := TitleStyle.Render(fmt.Sprintf("Sprint %d", sprint.Number))
	info := fmt.Sprintf("Day %d/%d  %s  %.0f%%",
		sprint.DaysElapsed(sim.CurrentTick),
		sprint.DurationDays,
		bar,
		progress*100,
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, info)
}

func (a *App) activeWorkPanel() string {
	sim := a.engine.Sim()
	title := TitleStyle.Render("Active Work")

	var rows []string
	for _, dev := range sim.Developers {
		if dev.IsIdle() {
			row := fmt.Sprintf("%-8s %s", dev.Name, MutedStyle.Render("[idle]"))
			rows = append(rows, row)
			continue
		}

		ticketIdx := sim.FindActiveTicketIndex(dev.CurrentTicket)
		if ticketIdx == -1 {
			continue
		}
		ticket := sim.ActiveTickets[ticketIdx]

		// Calculate progress within current phase
		phaseEffort := ticket.CalculatePhaseEffort(ticket.Phase)
		spent := ticket.PhaseEffortSpent[ticket.Phase]
		progress := 0.0
		if phaseEffort > 0 {
			progress = spent / phaseEffort
			if progress > 1 {
				progress = 1
			}
		}

		bar := RenderProgressBar(progress, 20)
		row := fmt.Sprintf("%-8s → %-10s %s %.0f%% (%s)",
			dev.Name,
			ticket.ID,
			bar,
			progress*100,
			ticket.Phase,
		)
		rows = append(rows, row)
	}

	content := strings.Join(rows, "\n")
	if len(rows) == 0 {
		content = MutedStyle.Render("No active work")
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, content)
}

func (a *App) feverPanel() string {
	sim := a.engine.Sim()
	title := TitleStyle.Render("Fever Chart")

	sprint, ok := sim.CurrentSprintOption.Get()
	if !ok {
		return lipgloss.JoinVertical(lipgloss.Left, title, MutedStyle.Render("No active sprint"))
	}

	pctUsed := sprint.BufferPctUsed() * 100

	bar := RenderProgressBar(sprint.BufferPctUsed(), 20)
	statusStyle := FeverColor(pctUsed)
	statusLabel := FeverLabel(pctUsed)

	info := fmt.Sprintf("Buffer: %s %.0f%% %s",
		bar,
		pctUsed,
		statusStyle.Render(statusLabel),
	)

	remaining := fmt.Sprintf("%.1f / %.1f days remaining",
		sprint.BufferRemaining(),
		sprint.BufferDays,
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, info, MutedStyle.Render(remaining))
}

func (a *App) eventsPanel() string {
	title := TitleStyle.Render("Events")

	// Show last 8 events
	start := 0
	if len(a.modelEvents) > 8 {
		start = len(a.modelEvents) - 8
	}

	var rows []string
	for i := len(a.modelEvents) - 1; i >= start; i-- {
		evt := a.modelEvents[i]
		row := fmt.Sprintf("Day %d: %s", evt.Day, evt.Message)
		if len(row) > 40 {
			row = row[:40] + "..."
		}
		rows = append(rows, MutedStyle.Render(row))
	}

	content := strings.Join(rows, "\n")
	if len(rows) == 0 {
		content = MutedStyle.Render("No events yet")
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, content)
}
