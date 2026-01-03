package tui

import (
	"fmt"
	"strings"

	"github.com/NimbleMarkets/ntcharts/sparkline"
	"github.com/charmbracelet/lipgloss"
)

func (a *App) metricsView() string {
	// DORA metrics
	dora := a.doraPanel()

	// Sprint history
	history := a.historyPanel()

	top := BoxStyle.Width(a.width - 2).Render(dora)
	bottom := BoxStyle.Width(a.width - 2).Render(history)

	return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
}

func (a *App) doraPanel() string {
	title := TitleStyle.Render("DORA Metrics")

	dora := a.tracker.DORA

	// Extract values for sparklines
	var leadTimes, deployFreqs, mttrs, cfrs []float64
	for _, h := range dora.History {
		leadTimes = append(leadTimes, h.LeadTimeAvg)
		deployFreqs = append(deployFreqs, h.DeployFrequency)
		mttrs = append(mttrs, h.MTTR)
		cfrs = append(cfrs, h.ChangeFailRate)
	}

	leadTimeSparkline := a.sparkline(leadTimes)
	deployFreqSparkline := a.sparkline(deployFreqs)
	mttrSparkline := a.sparkline(mttrs)
	cfrSparkline := a.sparkline(cfrs)

	// Format metrics
	col1 := lipgloss.JoinVertical(lipgloss.Left,
		HeaderStyle.Render("Lead Time"),
		fmt.Sprintf("%.1f days", dora.LeadTimeAvgDays()),
		leadTimeSparkline,
		MutedStyle.Render("↓ lower is better"),
	)

	col2 := lipgloss.JoinVertical(lipgloss.Left,
		HeaderStyle.Render("Deploy Freq"),
		fmt.Sprintf("%.2f/day", dora.DeployFrequency),
		deployFreqSparkline,
		MutedStyle.Render("↑ higher is better"),
	)

	col3 := lipgloss.JoinVertical(lipgloss.Left,
		HeaderStyle.Render("MTTR"),
		fmt.Sprintf("%.1f days", dora.MTTRAvgDays()),
		mttrSparkline,
		MutedStyle.Render("↓ lower is better"),
	)

	col4 := lipgloss.JoinVertical(lipgloss.Left,
		HeaderStyle.Render("Change Fail Rate"),
		fmt.Sprintf("%.1f%%", dora.ChangeFailRatePct()),
		cfrSparkline,
		MutedStyle.Render("↓ lower is better"),
	)

	colWidth := (a.width - 10) / 4
	row := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(colWidth).Render(col1),
		lipgloss.NewStyle().Width(colWidth).Render(col2),
		lipgloss.NewStyle().Width(colWidth).Render(col3),
		lipgloss.NewStyle().Width(colWidth).Render(col4),
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, row)
}

// sparkline creates a sparkline using ntcharts
func (a *App) sparkline(values []float64) string {
	if len(values) == 0 {
		return MutedStyle.Render("─────────────")
	}

	// Take last 15 values
	data := values
	if len(values) > 15 {
		data = values[len(values)-15:]
	}

	// Create ntcharts sparkline (width=15, height=1)
	sl := sparkline.New(15, 1)
	sl.PushAll(data)
	sl.Draw()

	return sl.View()
}

func (a *App) historyPanel() string {
	title := TitleStyle.Render("Completed Tickets")

	header := HeaderStyle.Render(fmt.Sprintf("%-10s %-25s %8s %8s %12s",
		"ID", "Title", "Est", "Actual", "Understanding"))

	var rows []string
	// Show last 10 completed
	start := 0
	if len(a.sim.CompletedTickets) > 10 {
		start = len(a.sim.CompletedTickets) - 10
	}

	for i := len(a.sim.CompletedTickets) - 1; i >= start; i-- {
		ticket := a.sim.CompletedTickets[i]
		title := ticket.Title
		if len(title) > 23 {
			title = title[:23] + ".."
		}

		row := fmt.Sprintf("%-10s %-25s %7.1fd %7.1fd %-12s",
			ticket.ID,
			title,
			ticket.EstimatedDays,
			ticket.ActualDays,
			ticket.UnderstandingLevel,
		)
		rows = append(rows, row)
	}

	content := strings.Join(rows, "\n")
	if len(rows) == 0 {
		content = MutedStyle.Render("No completed tickets yet")
	}

	stats := fmt.Sprintf("Total: %d completed | %d incidents | Policy: %s",
		len(a.sim.CompletedTickets),
		a.sim.TotalIncidents(),
		a.sim.SizingPolicy,
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, header, content, "", MutedStyle.Render(stats))
}
