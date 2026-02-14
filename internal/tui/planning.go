package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (a *App) planningView() string {
	// Backlog table on left (~60% width)
	backlog := a.backlogTable()

	// Office visualization on right (~40% width)
	// Shows both conference room and cubicle grid
	names := a.getDeveloperNames()
	officeWidth := computeOfficeWidth(ViewPlanning, a.width)
	office := RenderOffice(a.officeProjection.State(), names, officeWidth, a.height)

	// Combine: ticket list left, office right
	ticketWidth := a.width - officeWidth - 4 // Account for box borders
	left := BoxStyle.Width(ticketWidth).Render(backlog)
	right := office // No box - office renders its own borders

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (a *App) backlogTable() string {
	header := HeaderStyle.Render(fmt.Sprintf("%-8s %-30s %6s %12s %10s",
		"ID", "Title", "Est", "Understanding", "Phase"))

	var rows []string
	var backlogLen int

	if _, isClient := a.mode.Get(); isClient {
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
		eng, _ := a.mode.GetLeft()
		sim := eng.Engine.Sim()
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

