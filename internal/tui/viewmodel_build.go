package tui

import (
	"fmt"

	"github.com/binaryphile/fluentfp/either"
	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// Calculation: App → HeaderVM
func (a *App) buildHeaderVM() HeaderVM {
	h := either.Fold(a.mode,
		func(eng EngineMode) HeaderVM {
			sim := eng.Engine.Sim()
			return HeaderVM{
				Policy:         sim.SizingPolicy.String(),
				CurrentTick:    sim.CurrentTick,
				BacklogCount:   len(sim.Backlog),
				CompletedCount: len(sim.CompletedTickets),
				Seed:           sim.Seed,
			}
		},
		func(_ ClientMode) HeaderVM {
			return HeaderVM{
				Policy:         a.state.SizingPolicy,
				CurrentTick:    a.state.CurrentTick,
				BacklogCount:   a.state.BacklogCount,
				CompletedCount: a.state.CompletedTicketCount,
				Seed:           a.state.Seed,
			}
		},
	)
	h.CurrentView = a.currentView
	h.Paused = a.paused
	h.Width = a.width
	return h
}

// Calculation: App → PlanningVM
func (a *App) buildPlanningVM() PlanningVM {
	// engineTickets builds backlog ticket VMs from engine simulation.
	engineTickets := func(eng EngineMode) []BacklogTicketVM {
		sim := eng.Engine.Sim()
		var tickets []BacklogTicketVM
		for i, ticket := range sim.Backlog { // justified:IX
			tickets = append(tickets, BacklogTicketVM{
				ID:            ticket.ID,
				Title:         ticket.Title,
				EstimatedDays: ticket.EstimatedDays,
				Understanding: ticket.UnderstandingLevel.String(),
				Phase:         ticket.Phase.String(),
				Selected:      i == a.selected,
			})
		}
		return tickets
	}
	// clientTickets builds backlog ticket VMs from client state.
	clientTickets := func(_ ClientMode) []BacklogTicketVM {
		var tickets []BacklogTicketVM
		for i, ticket := range a.state.Backlog { // justified:IX
			tickets = append(tickets, BacklogTicketVM{
				ID:            ticket.ID,
				Title:         ticket.Title,
				EstimatedDays: ticket.Size,
				Understanding: ticket.Understanding,
				Phase:         ticket.Phase,
				Selected:      i == a.selected,
			})
		}
		return tickets
	}
	tickets := either.Fold(a.mode, engineTickets, clientTickets)

	// Clamp selected locally — no write-back to a.selected
	selected := a.selected
	if selected >= len(tickets) && len(tickets) > 0 {
		selected = len(tickets) - 1
	}
	// Fix selection if clamped
	if selected != a.selected && len(tickets) > 0 {
		for i := range tickets { // justified:IX
			tickets[i].Selected = i == selected
		}
	}

	return PlanningVM{
		Tickets:     tickets,
		OfficeState: a.officeProjection.State(),
		DevNames:    a.getDeveloperNames(),
		Width:       a.width,
		Height:      a.height,
	}
}

// Calculation: App → ExecutionVM
func (a *App) buildExecutionVM() ExecutionVM {
	return ExecutionVM{
		Sprint:      a.buildSprintProgressVM(),
		ActiveWork:  a.buildActiveWorkVM(),
		Fever:       a.buildFeverVM(),
		Events:      a.buildEventsVM(),
		OfficeState: a.officeProjection.State(),
		DevNames:    a.getDeveloperNames(),
		Width:       a.width,
		Height:      a.height,
	}
}

// Calculation: App → SprintProgressVM
func (a *App) buildSprintProgressVM() SprintProgressVM {
	// engineSprint builds sprint progress from engine simulation state.
	engineSprint := func(eng EngineMode) SprintProgressVM {
		sim := eng.Engine.Sim()
		sprint, ok := sim.CurrentSprintOption.Get()
		if !ok {
			return SprintProgressVM{}
		}
		return SprintProgressVM{
			HasSprint:    true,
			SprintNumber: sprint.Number,
			DaysElapsed:  sprint.DaysElapsed(sim.CurrentTick),
			DurationDays: sprint.DurationDays,
			Progress:     sprint.ProgressPct(sim.CurrentTick),
		}
	}
	// clientSprint computes sprint progress manually from client state.
	clientSprint := func(_ ClientMode) SprintProgressVM {
		sprint, ok := a.state.SprintOption.Get()
		if !ok {
			return SprintProgressVM{}
		}
		daysElapsed := a.state.CurrentTick - sprint.StartDay
		progress := 0.0
		if sprint.DurationDays > 0 {
			progress = float64(daysElapsed) / float64(sprint.DurationDays)
			if progress > 1 {
				progress = 1
			}
		}
		return SprintProgressVM{
			HasSprint:    true,
			SprintNumber: sprint.Number,
			DaysElapsed:  daysElapsed,
			DurationDays: sprint.DurationDays,
			Progress:     progress,
		}
	}
	return either.Fold(a.mode, engineSprint, clientSprint)
}

// Calculation: App → []ActiveWorkRowVM
func (a *App) buildActiveWorkVM() []ActiveWorkRowVM {
	// engineActiveWork builds active work rows from engine simulation state.
	engineActiveWork := func(eng EngineMode) []ActiveWorkRowVM {
		sim := eng.Engine.Sim()
		var rows []ActiveWorkRowVM
		for _, dev := range sim.Developers { // justified:CF
			if dev.IsIdle() {
				rows = append(rows, ActiveWorkRowVM{
					DevName: dev.Name,
					IsIdle:  true,
				})
				continue
			}
			ticketIdx := sim.FindActiveTicketIndex(dev.CurrentTicket)
			if ticketIdx == -1 {
				continue
			}
			ticket := sim.ActiveTickets[ticketIdx]
			phaseEffort := ticket.CalculatePhaseEffort(ticket.Phase)
			spent := ticket.PhaseEffortSpent[ticket.Phase]
			progress := 0.0
			if phaseEffort > 0 {
				progress = spent / phaseEffort
				if progress > 1 {
					progress = 1
				}
			}
			rows = append(rows, ActiveWorkRowVM{
				DevName:  dev.Name,
				TicketID: ticket.ID,
				Progress: progress,
				Phase:    ticket.Phase.String(),
			})
		}
		return rows
	}
	// clientActiveWork builds active work rows from client state.
	clientActiveWork := func(_ ClientMode) []ActiveWorkRowVM {
		var rows []ActiveWorkRowVM
		for _, dev := range a.state.Developers { // justified:CF
			if dev.IsIdle {
				rows = append(rows, ActiveWorkRowVM{
					DevName: dev.Name,
					IsIdle:  true,
				})
				continue
			}
			var ticket *TicketState
			for i := range a.state.ActiveTickets { // justified:FL
				if a.state.ActiveTickets[i].AssignedTo == dev.ID {
					ticket = &a.state.ActiveTickets[i]
					break
				}
			}
			if ticket == nil {
				continue
			}
			rows = append(rows, ActiveWorkRowVM{
				DevName:  dev.Name,
				TicketID: ticket.ID,
				Progress: ticket.Progress / 100.0,
				Phase:    ticket.Phase,
			})
		}
		return rows
	}
	return either.Fold(a.mode, engineActiveWork, clientActiveWork)
}

// Calculation: App → FeverVM
func (a *App) buildFeverVM() FeverVM {
	// engineFever builds fever chart data from engine simulation state.
	engineFever := func(eng EngineMode) FeverVM {
		sim := eng.Engine.Sim()
		sprint, ok := sim.CurrentSprintOption.Get()
		if !ok {
			return FeverVM{}
		}
		bufferPct := sprint.BufferPctUsed() * 100
		progressPct := sprint.Progress * 100
		var ratioStr string
		if sprint.Progress > 0.05 {
			ratio := sprint.BufferPctUsed() / sprint.Progress
			ratioStr = fmt.Sprintf("%.1f", ratio)
		} else {
			ratioStr = "-"
		}
		return FeverVM{
			HasSprint:   true,
			WorkPct:     progressPct,
			BufferPct:   bufferPct,
			RatioStr:    ratioStr,
			Zone:        sprint.FeverStatus.String(),
			Remaining:   sprint.BufferRemaining(),
			TotalBuffer: sprint.BufferDays,
		}
	}
	// clientFever builds fever chart data from client state.
	clientFever := func(_ ClientMode) FeverVM {
		sprint, ok := a.state.SprintOption.Get()
		if !ok {
			return FeverVM{}
		}
		bufferPct := (sprint.BufferConsumed / sprint.BufferDays) * 100
		progressPct := sprint.Progress * 100
		var ratioStr string
		if sprint.Progress > 0.05 {
			ratio := (sprint.BufferConsumed / sprint.BufferDays) / sprint.Progress
			ratioStr = fmt.Sprintf("%.1f", ratio)
		} else {
			ratioStr = "-"
		}
		return FeverVM{
			HasSprint:   true,
			WorkPct:     progressPct,
			BufferPct:   bufferPct,
			RatioStr:    ratioStr,
			Zone:        sprint.FeverStatus,
			Remaining:   sprint.BufferDays - sprint.BufferConsumed,
			TotalBuffer: sprint.BufferDays,
		}
	}
	return either.Fold(a.mode, engineFever, clientFever)
}

// Calculation: App → []EventVM
func (a *App) buildEventsVM() []EventVM {
	start := 0
	if len(a.modelEvents) > 8 {
		start = len(a.modelEvents) - 8
	}
	var events []EventVM
	for i := len(a.modelEvents) - 1; i >= start; i-- { // justified:SM
		evt := a.modelEvents[i]
		events = append(events, EventVM{
			Day:     evt.Day,
			Message: evt.Message,
		})
	}
	return events
}

// Calculation: App → MetricsVM
func (a *App) buildMetricsVM() MetricsVM {
	// engineMetrics builds metrics from engine tracker and simulation.
	engineMetrics := func(eng EngineMode) MetricsVM {
		dora := eng.Tracker.DORA
		vm := MetricsVM{
			DORA: DORAMetricsVM{
				LeadTimeAvg:     dora.LeadTimeAvgDays(),
				DeployFrequency: dora.DeployFrequency,
				MTTRAvg:         dora.MTTRAvgDays(),
				ChangeFailRate:  dora.ChangeFailRatePct(),
			},
		}
		leadTimes, deployFreqs, mttrs, cfrs := slice.Unzip4(dora.History,
			metrics.DORASnapshot.GetLeadTimeAvg,
			metrics.DORASnapshot.GetDeployFrequency,
			metrics.DORASnapshot.GetMTTR,
			metrics.DORASnapshot.GetChangeFailRate,
		)
		vm.DORA.LeadTimeHistory = leadTimes
		vm.DORA.DeployHistory = deployFreqs
		vm.DORA.MTTRHistory = mttrs
		vm.DORA.CFRHistory = cfrs

		sim := eng.Engine.Sim()
		vm.CompletedCount = len(sim.CompletedTickets)
		vm.TotalIncidents = sim.TotalIncidents()
		vm.Policy = sim.SizingPolicy.String()

		start := 0
		if vm.CompletedCount > 10 {
			start = vm.CompletedCount - 10
		}
		for i := vm.CompletedCount - 1; i >= start; i-- { // justified:SM
			ticket := sim.CompletedTickets[i]
			vm.CompletedTickets = append(vm.CompletedTickets, CompletedTicketVM{
				ID:            ticket.ID,
				Title:         ticket.Title,
				EstimatedDays: ticket.EstimatedDays,
				ActualDays:    ticket.ActualDays,
				Understanding: ticket.UnderstandingLevel.String(),
			})
		}
		return vm
	}
	// clientMetrics builds metrics from client state.
	clientMetrics := func(_ ClientMode) MetricsVM {
		vm := MetricsVM{
			DORA: DORAMetricsVM{
				LeadTimeAvg:     a.state.Metrics.LeadTimeAvgDays,
				DeployFrequency: a.state.Metrics.DeployFrequency,
				MTTRAvg:         a.state.Metrics.MTTRAvgDays,
				ChangeFailRate:  a.state.Metrics.ChangeFailRatePct,
			},
		}
		leadTimes, deployFreqs, mttrs, cfrs := slice.Unzip4(
			a.state.Metrics.History,
			DORAHistoryPoint.GetLeadTimeAvg,
			DORAHistoryPoint.GetDeployFrequency,
			DORAHistoryPoint.GetMTTR,
			DORAHistoryPoint.GetChangeFailRate,
		)
		vm.DORA.LeadTimeHistory = leadTimes
		vm.DORA.DeployHistory = deployFreqs
		vm.DORA.MTTRHistory = mttrs
		vm.DORA.CFRHistory = cfrs

		vm.CompletedCount = len(a.state.CompletedTickets)
		vm.TotalIncidents = a.state.TotalIncidents
		vm.Policy = a.state.SizingPolicy

		start := 0
		if vm.CompletedCount > 10 {
			start = vm.CompletedCount - 10
		}
		for i := vm.CompletedCount - 1; i >= start; i-- { // justified:SM
			ticket := a.state.CompletedTickets[i]
			vm.CompletedTickets = append(vm.CompletedTickets, CompletedTicketVM{
				ID:            ticket.ID,
				Title:         ticket.Title,
				EstimatedDays: ticket.Size,
				ActualDays:    ticket.ActualDays,
				Understanding: ticket.Understanding,
			})
		}
		return vm
	}
	vm := either.Fold(a.mode, engineMetrics, clientMetrics)
	vm.Width = a.width
	return vm
}

// comparisonWinnerStr returns a plain winner label for a metric.
// Calculation: (SizingPolicy, SizingPolicy, SizingPolicy) → string
func comparisonWinnerStr(winner, policyA, policyB model.SizingPolicy) string {
	if winner == policyA {
		return "DORA"
	} else if winner == policyB {
		return "TameFlow"
	}
	return "TIE"
}

// comparisonInsight generates insight text for the winning policy.
// Calculation: (string, float64, float64, float64, float64) → string
func comparisonInsight(winner string, winnerLT, loserLT, winnerCFR, loserCFR float64) string {
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

// Calculation: App → ComparisonVM
func (a *App) buildComparisonVM() ComparisonVM {
	if !a.comparisonResult.IsOk() {
		return ComparisonVM{Width: a.width}
	}

	result := a.comparisonResult.MustGet()

	ltA := result.ResultsA.FinalMetrics.LeadTimeAvgDays()
	ltB := result.ResultsB.FinalMetrics.LeadTimeAvgDays()
	dfA := result.ResultsA.FinalMetrics.DeployFrequency
	dfB := result.ResultsB.FinalMetrics.DeployFrequency
	mttrA := result.ResultsA.FinalMetrics.MTTRAvgDays()
	mttrB := result.ResultsB.FinalMetrics.MTTRAvgDays()
	cfrA := result.ResultsA.FinalMetrics.ChangeFailRatePct()
	cfrB := result.ResultsB.FinalMetrics.ChangeFailRatePct()
	tcA := result.ResultsA.TicketsComplete
	tcB := result.ResultsB.TicketsComplete
	incA := result.ResultsA.IncidentCount
	incB := result.ResultsB.IncidentCount

	tcWinner := "TIE"
	if tcA > tcB {
		tcWinner = "DORA"
	} else if tcB > tcA {
		tcWinner = "TameFlow"
	}

	incWinner := "TIE"
	if incA < incB {
		incWinner = "DORA"
	} else if incB < incA {
		incWinner = "TameFlow"
	}

	var overallWinner string
	if result.IsTie() {
		overallWinner = fmt.Sprintf("TIE (%d-%d)", result.WinsA, result.WinsB)
	} else {
		winner := result.OverallWinner.String()
		margin := result.WinMargin()
		overallWinner = fmt.Sprintf("%s (%d-%d)", winner, result.WinsA+margin, result.WinsA)
	}

	var insight string
	if !result.IsTie() {
		if result.OverallWinner == result.PolicyA {
			insight = comparisonInsight("DORA-Strict", ltA, ltB, cfrA, cfrB)
		} else {
			insight = comparisonInsight("TameFlow-Cognitive", ltB, ltA, cfrB, cfrA)
		}
	}

	return ComparisonVM{
		HasResult:      true,
		Seed:           a.comparisonSeed,
		PolicyAName:    result.PolicyA.String(),
		PolicyBName:    result.PolicyB.String(),
		LeadTimeA:      ltA,
		LeadTimeB:      ltB,
		DeployFreqA:    dfA,
		DeployFreqB:    dfB,
		MTTRA:          mttrA,
		MTTRB:          mttrB,
		CFRA:           cfrA,
		CFRB:           cfrB,
		TicketsA:       tcA,
		TicketsB:       tcB,
		IncidentsA:     incA,
		IncidentsB:     incB,
		LeadTimeWinner: comparisonWinnerStr(result.LeadTimeWinner, result.PolicyA, result.PolicyB),
		DeployFreqWinner: comparisonWinnerStr(result.DeployFreqWinner, result.PolicyA, result.PolicyB),
		MTTRWinner:       comparisonWinnerStr(result.MTTRWinner, result.PolicyA, result.PolicyB),
		CFRWinner:        comparisonWinnerStr(result.CFRWinner, result.PolicyA, result.PolicyB),
		TicketsWinner:  tcWinner,
		IncidentsWinner: incWinner,
		OverallWinner:  overallWinner,
		IsTie:          result.IsTie(),
		WinsA:          result.WinsA,
		WinsB:          result.WinsB,
		Insight:        insight,
		Width:          a.width,
	}
}

// Calculation: (App, Lesson) → LessonVM
func (a *App) buildLessonVM(lesson Lesson) LessonVM {
	return LessonVM{
		Title:    lesson.Title,
		Content:  lesson.Content,
		Tips:     lesson.Tips,
		Progress: a.lessonState.SeenCount(),
		Total:    TotalLessons,
		Width:    a.width,
	}
}
