package tui

import (
	"fmt"
	"strings"

	"github.com/binaryphile/fluentfp/ternary"
	"github.com/charmbracelet/lipgloss"
)

func (a *App) planningView() string {
	// Backlog table
	backlog := a.backlogTable()

	// Developers status
	devs := a.developersPanel()

	// Combine
	left := BoxStyle.Width(a.width*2/3 - 2).Render(backlog)
	right := BoxStyle.Width(a.width/3 - 2).Render(devs)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (a *App) backlogTable() string {
	header := HeaderStyle.Render(fmt.Sprintf("%-8s %-30s %6s %12s %10s",
		"ID", "Title", "Est", "Understanding", "Phase"))

	var rows []string
	var backlogLen int

	if a.client != nil {
		// Client mode: use HTTP state
		backlogLen = len(a.state.Backlog)
		for i, ticket := range a.state.Backlog {
			style := TableRowStyle
			if i == a.selected {
				style = TableSelectedStyle
			}

			ticketTitle := ticket.Title
			if len(ticketTitle) > 28 {
				ticketTitle = ticketTitle[:28] + ".."
			}

			row := style.Render(fmt.Sprintf("%-8s %-30s %5.1fd %-12s %-10s",
				ticket.ID,
				ticketTitle,
				ticket.Size,
				ticket.Understanding,
				ticket.Phase,
			))
			rows = append(rows, row)
		}
	} else {
		// Engine mode: use local simulation
		sim := a.engine.Sim()
		backlogLen = len(sim.Backlog)
		for i, ticket := range sim.Backlog {
			style := TableRowStyle
			if i == a.selected {
				style = TableSelectedStyle
			}

			ticketTitle := ticket.Title
			if len(ticketTitle) > 28 {
				ticketTitle = ticketTitle[:28] + ".."
			}

			row := style.Render(fmt.Sprintf("%-8s %-30s %5.1fd %-12s %-10s",
				ticket.ID,
				ticketTitle,
				ticket.EstimatedDays,
				ticket.UnderstandingLevel,
				ticket.Phase,
			))
			rows = append(rows, row)
		}
	}

	// Clamp selected
	if a.selected >= backlogLen && backlogLen > 0 {
		a.selected = backlogLen - 1
	}

	title := TitleStyle.Render(fmt.Sprintf("Backlog (%d tickets)", backlogLen))
	content := strings.Join(rows, "\n")
	if len(rows) == 0 {
		content = MutedStyle.Render("No tickets in backlog")
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, header, content)
}

func (a *App) developersPanel() string {
	title := TitleStyle.Render("Team")

	var rows []string
	if a.client != nil {
		// Client mode: use HTTP state
		for _, dev := range a.state.Developers {
			status := ternary.If[string](dev.IsIdle).Then(GreenStyle.Render("[idle]")).Else(YellowStyle.Render("[busy]"))
			assignment := ""
			if !dev.IsIdle {
				assignment = MutedStyle.Render(" → " + dev.CurrentTicket)
			}

			row := fmt.Sprintf("%s (v:%.1f) %s%s", dev.Name, dev.Velocity, status, assignment)
			rows = append(rows, row)
		}
	} else {
		// Engine mode: use local simulation
		sim := a.engine.Sim()
		for _, dev := range sim.Developers {
			status := ternary.If[string](dev.IsIdle()).Then(GreenStyle.Render("[idle]")).Else(YellowStyle.Render("[busy]"))
			assignment := ""
			if !dev.IsIdle() {
				assignment = MutedStyle.Render(" → " + dev.CurrentTicket)
			}

			row := fmt.Sprintf("%s (v:%.1f) %s%s", dev.Name, dev.Velocity, status, assignment)
			rows = append(rows, row)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, strings.Join(rows, "\n"))
}
