package tui

import (
	"fmt"
	"strings"

	"github.com/binaryphile/fluentfp/slice"
	"github.com/charmbracelet/lipgloss"
)

// Calculation: PlanningVM → string
func renderPlanning(vm PlanningVM) string {
	backlog := renderBacklogTable(vm.Tickets)
	office := RenderOffice(vm.OfficeState, vm.DevNames, vm.Width, vm.Height)

	top := BoxStyle.Width(vm.Width - 2).Render(backlog)
	return lipgloss.JoinVertical(lipgloss.Left, top, office)
}

// Calculation: []BacklogTicketVM → string
func renderBacklogTable(tickets []BacklogTicketVM) string {
	header := HeaderStyle.Render(fmt.Sprintf("%-8s %-30s %6s %12s %10s",
		"ID", "Title", "Est", "Understanding", "Phase"))

	// formatTicketRow renders a backlog ticket with selection styling.
	formatTicketRow := func(ticket BacklogTicketVM) string {
		style := TableRowStyle
		if ticket.Selected {
			style = TableSelectedStyle
		}
		return style.Render(fmt.Sprintf("%-8s %-30s %5.1fd %-12s %-10s",
			ticket.ID, truncate(ticket.Title, 28), ticket.EstimatedDays,
			ticket.Understanding, ticket.Phase))
	}
	rows := slice.From(tickets).ToString(formatTicketRow)

	title := TitleStyle.Render(fmt.Sprintf("Backlog (%d tickets)", len(tickets)))
	content := strings.Join(rows, "\n")
	if len(rows) == 0 {
		content = MutedStyle.Render("No tickets in backlog")
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, header, content)
}
