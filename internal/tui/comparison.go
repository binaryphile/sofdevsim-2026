package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// styleWinner applies green + checkmark to winners, muted to ties.
// Calculation: string → string
func styleWinner(winner string) string {
	if winner == "TIE" {
		return MutedStyle.Render("TIE")
	}
	return GreenStyle.Render(winner + " ✓")
}

// Calculation: ComparisonVM → string
func renderComparison(vm ComparisonVM) string {
	if !vm.HasResult {
		return BoxStyle.Width(vm.Width - 2).Render(
			lipgloss.JoinVertical(lipgloss.Left,
				TitleStyle.Render("Policy Comparison"),
				"",
				MutedStyle.Render("Press 'c' to run a comparison between DORA-Strict and TameFlow-Cognitive policies."),
				"",
				MutedStyle.Render("This will run 3 sprints with identical backlog and team under each policy,"),
				MutedStyle.Render("then compare the DORA metrics to determine which approach performs better."),
			),
		)
	}

	title := TitleStyle.Render("Policy Comparison Results")
	header := fmt.Sprintf("Seed: %d  |  Sprints: 3  |  Same backlog & team", vm.Seed)

	tableHeader := HeaderStyle.Render(fmt.Sprintf("%-20s %15s %18s %10s",
		"Metric", vm.PolicyAName, vm.PolicyBName, "Winner"))

	var rows []string
	rows = append(rows, fmt.Sprintf("%-20s %14.1fd %17.1fd %10s",
		"Lead Time", vm.LeadTimeA, vm.LeadTimeB, styleWinner(vm.LeadTimeWinner)))
	rows = append(rows, fmt.Sprintf("%-20s %13.2f/d %16.2f/d %10s",
		"Deploy Frequency", vm.DeployFreqA, vm.DeployFreqB, styleWinner(vm.DeployFreqWinner)))
	rows = append(rows, fmt.Sprintf("%-20s %14.1fd %17.1fd %10s",
		"MTTR", vm.MTTRA, vm.MTTRB, styleWinner(vm.MTTRWinner)))
	rows = append(rows, fmt.Sprintf("%-20s %14.1f%% %17.1f%% %10s",
		"Change Fail Rate", vm.CFRA, vm.CFRB, styleWinner(vm.CFRWinner)))
	rows = append(rows, fmt.Sprintf("%-20s %15d %18d %10s",
		"Tickets Complete", vm.TicketsA, vm.TicketsB, styleWinner(vm.TicketsWinner)))
	rows = append(rows, fmt.Sprintf("%-20s %15d %18d %10s",
		"Incidents", vm.IncidentsA, vm.IncidentsB, styleWinner(vm.IncidentsWinner)))

	table := lipgloss.JoinVertical(lipgloss.Left,
		tableHeader,
		strings.Join(rows, "\n"),
	)

	var overallMsg string
	if vm.IsTie {
		overallMsg = YellowStyle.Render(fmt.Sprintf("RESULT: %s — Run more sprints for conclusive results",
			vm.OverallWinner))
	} else {
		overallMsg = GreenStyle.Render(fmt.Sprintf("WINNER: %s on DORA metrics",
			vm.OverallWinner))
		if vm.Insight != "" {
			overallMsg += "\n\n" + vm.Insight
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		MutedStyle.Render(header),
		"",
		table,
		"",
		overallMsg,
		"",
		MutedStyle.Render("Press 'c' to run another comparison with a new seed."),
	)

	return BoxStyle.Width(vm.Width - 2).Render(content)
}
