package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (a *App) comparisonView() string {
	if a.comparisonResult == nil {
		return BoxStyle.Width(a.width - 2).Render(
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

	result := a.comparisonResult
	title := TitleStyle.Render("Policy Comparison Results")

	// Header info
	header := fmt.Sprintf("Seed: %d  |  Sprints: 3  |  Same backlog & team", a.comparisonSeed)

	// Results table
	tableHeader := HeaderStyle.Render(fmt.Sprintf("%-20s %15s %18s %10s",
		"Metric", result.PolicyA.String(), result.PolicyB.String(), "Winner"))

	var rows []string

	// Lead Time (lower is better)
	ltA := result.ResultsA.FinalMetrics.LeadTimeAvgDays()
	ltB := result.ResultsB.FinalMetrics.LeadTimeAvgDays()
	ltWinner := winnerStr(result.LeadTimeWinner, result.PolicyA, result.PolicyB)
	rows = append(rows, fmt.Sprintf("%-20s %14.1fd %17.1fd %10s",
		"Lead Time", ltA, ltB, ltWinner))

	// Deploy Frequency (higher is better)
	dfA := result.ResultsA.FinalMetrics.DeployFrequency
	dfB := result.ResultsB.FinalMetrics.DeployFrequency
	dfWinner := winnerStr(result.DeployFreqWinner, result.PolicyA, result.PolicyB)
	rows = append(rows, fmt.Sprintf("%-20s %13.2f/d %16.2f/d %10s",
		"Deploy Frequency", dfA, dfB, dfWinner))

	// MTTR (lower is better)
	mttrA := result.ResultsA.FinalMetrics.MTTRAvgDays()
	mttrB := result.ResultsB.FinalMetrics.MTTRAvgDays()
	mttrWinner := winnerStr(result.MTTRWinner, result.PolicyA, result.PolicyB)
	rows = append(rows, fmt.Sprintf("%-20s %14.1fd %17.1fd %10s",
		"MTTR", mttrA, mttrB, mttrWinner))

	// Change Fail Rate (lower is better)
	cfrA := result.ResultsA.FinalMetrics.ChangeFailRatePct()
	cfrB := result.ResultsB.FinalMetrics.ChangeFailRatePct()
	cfrWinner := winnerStr(result.CFRWinner, result.PolicyA, result.PolicyB)
	rows = append(rows, fmt.Sprintf("%-20s %14.1f%% %17.1f%% %10s",
		"Change Fail Rate", cfrA, cfrB, cfrWinner))

	// Tickets Complete
	tcA := result.ResultsA.TicketsComplete
	tcB := result.ResultsB.TicketsComplete
	tcWinner := ""
	if tcA > tcB {
		tcWinner = "DORA"
	} else if tcB > tcA {
		tcWinner = "TameFlow"
	} else {
		tcWinner = "TIE"
	}
	rows = append(rows, fmt.Sprintf("%-20s %15d %18d %10s",
		"Tickets Complete", tcA, tcB, tcWinner))

	// Incidents
	incA := result.ResultsA.IncidentCount
	incB := result.ResultsB.IncidentCount
	incWinner := ""
	if incA < incB {
		incWinner = "DORA"
	} else if incB < incA {
		incWinner = "TameFlow"
	} else {
		incWinner = "TIE"
	}
	rows = append(rows, fmt.Sprintf("%-20s %15d %18d %10s",
		"Incidents", incA, incB, incWinner))

	table := lipgloss.JoinVertical(lipgloss.Left,
		tableHeader,
		strings.Join(rows, "\n"),
	)

	// Overall winner
	var overallMsg string
	if result.IsTie() {
		overallMsg = YellowStyle.Render(fmt.Sprintf("RESULT: TIE (%d-%d) — Run more sprints for conclusive results",
			result.WinsA, result.WinsB))
	} else {
		winner := result.OverallWinner.String()
		margin := result.WinMargin()
		overallMsg = GreenStyle.Render(fmt.Sprintf("WINNER: %s (%d-%d on DORA metrics)",
			winner, result.WinsA+margin, result.WinsA))

		// Add insight about what the experiment revealed
		if result.OverallWinner == result.PolicyA {
			overallMsg += "\n\n" + a.experimentInsight("DORA-Strict", ltA, ltB, cfrA, cfrB)
		} else {
			overallMsg += "\n\n" + a.experimentInsight("TameFlow-Cognitive", ltB, ltA, cfrB, cfrA)
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

	return BoxStyle.Width(a.width - 2).Render(content)
}

func winnerStr(winner, policyA, policyB interface{}) string {
	if winner == policyA {
		return GreenStyle.Render("DORA ✓")
	} else if winner == policyB {
		return GreenStyle.Render("TameFlow ✓")
	}
	return MutedStyle.Render("TIE")
}

func (a *App) experimentInsight(winner string, winnerLT, loserLT, winnerCFR, loserCFR float64) string {
	var insight string

	if winner == "DORA-Strict" {
		insight = "Experiment suggests: TIME-BASED decomposition (>5 days) led to better outcomes.\n"
		if winnerLT < loserLT {
			insight += fmt.Sprintf("• Lead time improved by %.1f days (%.0f%% faster)\n",
				loserLT-winnerLT, (loserLT-winnerLT)/loserLT*100)
		}
		if winnerCFR < loserCFR {
			insight += fmt.Sprintf("• Change fail rate reduced by %.1f%% points\n",
				loserCFR-winnerCFR)
		}
		insight += "• Breaking large tickets by size ceiling improved predictability"
	} else {
		insight = "Experiment suggests: COGNITIVE LOAD decomposition (low understanding) led to better outcomes.\n"
		if winnerLT < loserLT {
			insight += fmt.Sprintf("• Lead time improved by %.1f days (%.0f%% faster)\n",
				loserLT-winnerLT, (loserLT-winnerLT)/loserLT*100)
		}
		if winnerCFR < loserCFR {
			insight += fmt.Sprintf("• Change fail rate reduced by %.1f%% points\n",
				loserCFR-winnerCFR)
		}
		insight += "• Understanding level is a stronger discriminant than time estimate"
	}

	return insight
}
