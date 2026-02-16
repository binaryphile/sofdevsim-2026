package tui

import (
	"fmt"
	"strings"

	"github.com/NimbleMarkets/ntcharts/sparkline"
	"github.com/binaryphile/fluentfp/slice"
	"github.com/charmbracelet/lipgloss"
)

// Calculation: MetricsVM → string
func renderMetrics(vm MetricsVM) string {
	dora := renderDORA(vm.DORA, vm.Width)
	history := renderHistory(vm)

	top := BoxStyle.Width(vm.Width - 2).Render(dora)
	bottom := BoxStyle.Width(vm.Width - 2).Render(history)

	return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
}

// Calculation: (DORAMetricsVM, int) → string
func renderDORA(vm DORAMetricsVM, width int) string {
	title := TitleStyle.Render("DORA Metrics")

	leadTimeSparkline := renderSparkline(vm.LeadTimeHistory)
	deployFreqSparkline := renderSparkline(vm.DeployHistory)
	mttrSparkline := renderSparkline(vm.MTTRHistory)
	cfrSparkline := renderSparkline(vm.CFRHistory)

	col1 := lipgloss.JoinVertical(lipgloss.Left,
		HeaderStyle.Render("Lead Time"),
		fmt.Sprintf("%.1f days", vm.LeadTimeAvg),
		leadTimeSparkline,
		MutedStyle.Render("↓ lower is better"),
	)

	col2 := lipgloss.JoinVertical(lipgloss.Left,
		HeaderStyle.Render("Deploy Freq"),
		fmt.Sprintf("%.2f/day", vm.DeployFrequency),
		deployFreqSparkline,
		MutedStyle.Render("↑ higher is better"),
	)

	col3 := lipgloss.JoinVertical(lipgloss.Left,
		HeaderStyle.Render("MTTR"),
		fmt.Sprintf("%.1f days", vm.MTTRAvg),
		mttrSparkline,
		MutedStyle.Render("↓ lower is better"),
	)

	col4 := lipgloss.JoinVertical(lipgloss.Left,
		HeaderStyle.Render("Change Fail Rate"),
		fmt.Sprintf("%.1f%%", vm.ChangeFailRate),
		cfrSparkline,
		MutedStyle.Render("↓ lower is better"),
	)

	colWidth := (width - 10) / 4
	row := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(colWidth).Render(col1),
		lipgloss.NewStyle().Width(colWidth).Render(col2),
		lipgloss.NewStyle().Width(colWidth).Render(col3),
		lipgloss.NewStyle().Width(colWidth).Render(col4),
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, row)
}

// Calculation: []float64 → string
func renderSparkline(values []float64) string {
	if len(values) == 0 {
		return MutedStyle.Render("─────────────")
	}

	data := values
	if len(values) > 15 {
		data = values[len(values)-15:]
	}

	sl := sparkline.New(15, 1)
	sl.PushAll(data)
	sl.Draw()

	return sl.View()
}

// Calculation: MetricsVM → string
func renderHistory(vm MetricsVM) string {
	title := TitleStyle.Render("Completed Tickets")

	header := HeaderStyle.Render(fmt.Sprintf("%-10s %-25s %8s %8s %12s",
		"ID", "Title", "Est", "Actual", "Understanding"))

	// formatCompletedTicket formats a completed ticket as a table row.
	formatCompletedTicket := func(t CompletedTicketVM) string {
		return fmt.Sprintf("%-10s %-25s %7.1fd %7.1fd %-12s",
			t.ID, truncate(t.Title, 23), t.EstimatedDays, t.ActualDays, t.Understanding)
	}
	rows := slice.From(vm.CompletedTickets).ToString(formatCompletedTicket)

	content := strings.Join(rows, "\n")
	if len(rows) == 0 {
		content = MutedStyle.Render("No completed tickets yet")
	}

	stats := fmt.Sprintf("Total: %d completed | %d incidents | Policy: %s",
		vm.CompletedCount,
		vm.TotalIncidents,
		vm.Policy,
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, header, content, "", MutedStyle.Render(stats))
}
