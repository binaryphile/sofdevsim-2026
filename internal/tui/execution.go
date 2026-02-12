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
	if _, isClient := a.mode.Get(); isClient {
		// Client mode: use HTTP state
		sprint, ok := a.state.SprintOption.Get()
		if !ok {
			return MutedStyle.Render("No active sprint")
		}
		daysElapsed := a.state.CurrentTick - sprint.StartDay
		progress := 0.0
		if sprint.DurationDays > 0 {
			progress = float64(daysElapsed) / float64(sprint.DurationDays)
			if progress > 1 {
				progress = 1
			}
		}
		bar := RenderProgressBar(progress, 50)

		title := TitleStyle.Render(fmt.Sprintf("Sprint %d", sprint.Number))
		info := fmt.Sprintf("Day %d/%d  %s  %.0f%%",
			daysElapsed,
			sprint.DurationDays,
			bar,
			progress*100,
		)

		return lipgloss.JoinVertical(lipgloss.Left, title, info)
	}

	// Engine mode: use local simulation
	eng, _ := a.mode.GetLeft()
	sim := eng.Engine.Sim()
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
	title := TitleStyle.Render("Active Work")

	var rows []string

	if _, isClient := a.mode.Get(); isClient {
		// Client mode: use HTTP state
		for _, dev := range a.state.Developers {
			if dev.IsIdle {
				row := fmt.Sprintf("%-8s %s", dev.Name, MutedStyle.Render("[idle]"))
				rows = append(rows, row)
				continue
			}

			// Find the active ticket for this developer
			var ticket *TicketState
			for i := range a.state.ActiveTickets {
				if a.state.ActiveTickets[i].AssignedTo == dev.ID {
					ticket = &a.state.ActiveTickets[i]
					break
				}
			}
			if ticket == nil {
				continue
			}

			// Use pre-calculated progress from server
			progress := ticket.Progress / 100.0 // Convert percentage to 0-1
			bar := RenderProgressBar(progress, 20)
			row := fmt.Sprintf("%-8s → %-10s %s %.0f%% (%s)",
				dev.Name,
				ticket.ID,
				bar,
				ticket.Progress,
				ticket.Phase,
			)
			rows = append(rows, row)
		}
	} else {
		// Engine mode: use local simulation
		eng, _ := a.mode.GetLeft()
		sim := eng.Engine.Sim()
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
	}

	content := strings.Join(rows, "\n")
	if len(rows) == 0 {
		content = MutedStyle.Render("No active work")
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, content)
}

func (a *App) feverPanel() string {
	title := TitleStyle.Render("Fever Chart")

	if _, isClient := a.mode.Get(); isClient {
		// Client mode: use HTTP state
		sprint, ok := a.state.SprintOption.Get()
		if !ok {
			return lipgloss.JoinVertical(lipgloss.Left, title, MutedStyle.Render("No active sprint"))
		}

		bufferPct := (sprint.BufferConsumed / sprint.BufferDays) * 100
		progressPct := sprint.Progress * 100

		// Ratio: buffer% / progress% (only meaningful when progress > 5%)
		var ratioStr string
		if sprint.Progress > 0.05 {
			ratio := (sprint.BufferConsumed / sprint.BufferDays) / sprint.Progress
			ratioStr = fmt.Sprintf("%.1f", ratio)
		} else {
			ratioStr = "-"
		}

		statusStyle := FeverColor(bufferPct)
		zone := feverEmojiFromString(sprint.FeverStatus)

		info := fmt.Sprintf("Work: %.0f%%  Buffer: %.0f%%  Ratio: %s %s",
			progressPct,
			bufferPct,
			ratioStr,
			statusStyle.Render(zone),
		)

		remaining := fmt.Sprintf("%.1f / %.1f days remaining",
			sprint.BufferDays-sprint.BufferConsumed,
			sprint.BufferDays,
		)

		return lipgloss.JoinVertical(lipgloss.Left, title, info, MutedStyle.Render(remaining))
	}

	// Engine mode: use local simulation
	eng, _ := a.mode.GetLeft()
	sim := eng.Engine.Sim()
	sprint, ok := sim.CurrentSprintOption.Get()
	if !ok {
		return lipgloss.JoinVertical(lipgloss.Left, title, MutedStyle.Render("No active sprint"))
	}

	bufferPct := sprint.BufferPctUsed() * 100
	progressPct := sprint.Progress * 100

	// Ratio: buffer% / progress% (only meaningful when progress > 5%)
	var ratioStr string
	if sprint.Progress > 0.05 {
		ratio := sprint.BufferPctUsed() / sprint.Progress
		ratioStr = fmt.Sprintf("%.1f", ratio)
	} else {
		ratioStr = "-"
	}

	statusStyle := FeverColor(bufferPct)
	zone := FeverEmoji(sprint.FeverStatus)

	info := fmt.Sprintf("Work: %.0f%%  Buffer: %.0f%%  Ratio: %s %s",
		progressPct,
		bufferPct,
		ratioStr,
		statusStyle.Render(zone),
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
