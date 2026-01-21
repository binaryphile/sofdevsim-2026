package tui

import (
	"fmt"
	"strings"

	"github.com/NimbleMarkets/ntcharts/sparkline"
	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
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

	var leadTimeAvg, deployFreq, mttrAvg, cfrPct float64
	var leadTimeSparkline, deployFreqSparkline, mttrSparkline, cfrSparkline string

	if _, isClient := a.mode.Get(); isClient {
		// Client mode: use HTTP state with history for sparklines
		leadTimeAvg = a.state.Metrics.LeadTimeAvgDays
		deployFreq = a.state.Metrics.DeployFrequency
		mttrAvg = a.state.Metrics.MTTRAvgDays
		cfrPct = a.state.Metrics.ChangeFailRatePct

		// Extract history for sparklines
		var leadTimes, deployFreqs, mttrs, cfrs []float64
		for _, h := range a.state.Metrics.History {
			leadTimes = append(leadTimes, h.LeadTimeAvg)
			deployFreqs = append(deployFreqs, h.DeployFrequency)
			mttrs = append(mttrs, h.MTTR)
			cfrs = append(cfrs, h.ChangeFailRate)
		}

		leadTimeSparkline = a.sparkline(leadTimes)
		deployFreqSparkline = a.sparkline(deployFreqs)
		mttrSparkline = a.sparkline(mttrs)
		cfrSparkline = a.sparkline(cfrs)
	} else {
		// Engine mode: use local tracker with sparklines
		eng, _ := a.mode.GetLeft()
		dora := eng.Tracker.DORA

		leadTimeAvg = dora.LeadTimeAvgDays()
		deployFreq = dora.DeployFrequency
		mttrAvg = dora.MTTRAvgDays()
		cfrPct = dora.ChangeFailRatePct()

		// Extract values for sparklines
		leadTimes, deployFreqs, mttrs, cfrs := slice.Unzip4(dora.History,
			func(h metrics.DORASnapshot) float64 { return h.LeadTimeAvg },
			func(h metrics.DORASnapshot) float64 { return h.DeployFrequency },
			func(h metrics.DORASnapshot) float64 { return h.MTTR },
			func(h metrics.DORASnapshot) float64 { return h.ChangeFailRate },
		)

		leadTimeSparkline = a.sparkline(leadTimes)
		deployFreqSparkline = a.sparkline(deployFreqs)
		mttrSparkline = a.sparkline(mttrs)
		cfrSparkline = a.sparkline(cfrs)
	}

	// Format metrics
	col1 := lipgloss.JoinVertical(lipgloss.Left,
		HeaderStyle.Render("Lead Time"),
		fmt.Sprintf("%.1f days", leadTimeAvg),
		leadTimeSparkline,
		MutedStyle.Render("↓ lower is better"),
	)

	col2 := lipgloss.JoinVertical(lipgloss.Left,
		HeaderStyle.Render("Deploy Freq"),
		fmt.Sprintf("%.2f/day", deployFreq),
		deployFreqSparkline,
		MutedStyle.Render("↑ higher is better"),
	)

	col3 := lipgloss.JoinVertical(lipgloss.Left,
		HeaderStyle.Render("MTTR"),
		fmt.Sprintf("%.1f days", mttrAvg),
		mttrSparkline,
		MutedStyle.Render("↓ lower is better"),
	)

	col4 := lipgloss.JoinVertical(lipgloss.Left,
		HeaderStyle.Render("Change Fail Rate"),
		fmt.Sprintf("%.1f%%", cfrPct),
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
	var completedCount, totalIncidents int
	var policy string

	if _, isClient := a.mode.Get(); isClient {
		// Client mode: use HTTP state
		completedCount = len(a.state.CompletedTickets)
		totalIncidents = a.state.TotalIncidents
		policy = a.state.SizingPolicy

		// Show last 10 completed
		start := 0
		if completedCount > 10 {
			start = completedCount - 10
		}

		for i := completedCount - 1; i >= start; i-- {
			ticket := a.state.CompletedTickets[i]
			ticketTitle := ticket.Title
			if len(ticketTitle) > 23 {
				ticketTitle = ticketTitle[:23] + ".."
			}

			row := fmt.Sprintf("%-10s %-25s %7.1fd %7.1fd %-12s",
				ticket.ID,
				ticketTitle,
				ticket.Size,
				ticket.ActualDays,
				ticket.Understanding,
			)
			rows = append(rows, row)
		}
	} else {
		// Engine mode: use local simulation
		eng, _ := a.mode.GetLeft()
		sim := eng.Engine.Sim()
		completedCount = len(sim.CompletedTickets)
		totalIncidents = sim.TotalIncidents()
		policy = sim.SizingPolicy.String()

		// Show last 10 completed
		start := 0
		if completedCount > 10 {
			start = completedCount - 10
		}

		for i := completedCount - 1; i >= start; i-- {
			ticket := sim.CompletedTickets[i]
			ticketTitle := ticket.Title
			if len(ticketTitle) > 23 {
				ticketTitle = ticketTitle[:23] + ".."
			}

			row := fmt.Sprintf("%-10s %-25s %7.1fd %7.1fd %-12s",
				ticket.ID,
				ticketTitle,
				ticket.EstimatedDays,
				ticket.ActualDays,
				ticket.UnderstandingLevel,
			)
			rows = append(rows, row)
		}
	}

	content := strings.Join(rows, "\n")
	if len(rows) == 0 {
		content = MutedStyle.Render("No completed tickets yet")
	}

	stats := fmt.Sprintf("Total: %d completed | %d incidents | Policy: %s",
		completedCount,
		totalIncidents,
		policy,
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, header, content, "", MutedStyle.Render(stats))
}
