package export

import (
	"fmt"
	"strconv"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// CSV Headers
var (
	MetadataHeader = []string{
		"seed", "policy", "sprints_run", "export_timestamp", "phase_effort_distribution",
	}

	TicketsHeader = []string{
		"ticket_id", "title", "understanding", "estimated_days", "actual_days",
		"variance_ratio", "expected_var_min", "expected_var_max", "within_expected",
		"policy", "sprint_number", "started_tick", "completed_tick", "lead_time_days",
		"phase_research_days", "phase_sizing_days", "phase_planning_days",
		"phase_implement_days", "phase_verify_days", "phase_cicd_days",
		"phase_review_days", "phase_done_days",
	}

	SprintsHeader = []string{
		"sprint_number", "duration_days", "buffer_days", "buffer_used",
		"buffer_pct", "fever_status", "tickets_started", "tickets_completed",
		"incidents_generated", "max_wip", "avg_wip",
	}

	IncidentsHeader = []string{
		"incident_id", "ticket_id", "severity", "created_at",
		"resolved_at", "mttr_days", "sprint_number",
	}

	MetricsHeader = []string{
		"policy", "lead_time_avg", "lead_time_stddev", "deploy_frequency",
		"mttr_avg", "change_fail_rate", "total_tickets", "total_incidents",
	}

	ComparisonHeader = []string{
		"seed", "sprints_run", "metric", "dora_strict_value",
		"tameflow_value", "winner", "difference", "difference_pct",
	}
)

// Phase effort distribution as used by simulation
var PhaseEffortDistribution = map[string]float64{
	"research": 0.05, "sizing": 0.02, "planning": 0.03,
	"implement": 0.55, "verify": 0.20, "cicd": 0.05,
	"review": 0.10, "done": 0.00,
}

// GetVarianceBounds returns the theoretical variance bounds for an understanding level
func GetVarianceBounds(level model.UnderstandingLevel) (min, max float64) {
	switch level {
	case model.HighUnderstanding:
		return 0.95, 1.05
	case model.MediumUnderstanding:
		return 0.80, 1.20
	case model.LowUnderstanding:
		return 0.50, 1.50
	default:
		return 0.80, 1.20 // default to medium
	}
}

// IsWithinExpected checks if actual variance falls within theoretical bounds
func IsWithinExpected(actual, estimated float64, level model.UnderstandingLevel) bool {
	if estimated == 0 {
		return actual == 0
	}
	ratio := actual / estimated
	min, max := GetVarianceBounds(level)
	return ratio >= min && ratio <= max
}

// formatTicketRow formats a ticket for CSV export
func formatTicketRow(t model.Ticket, policy model.SizingPolicy, sprintNum int) []string {
	varianceRatio := 0.0
	if t.EstimatedDays > 0 {
		varianceRatio = t.ActualDays / t.EstimatedDays
	}

	min, max := GetVarianceBounds(t.UnderstandingLevel)
	withinExpected := IsWithinExpected(t.ActualDays, t.EstimatedDays, t.UnderstandingLevel)

	leadTime := float64(t.CompletedTick - t.StartedTick)

	return []string{
		t.ID,
		t.Title,
		t.UnderstandingLevel.String(),
		fmt.Sprintf("%.2f", t.EstimatedDays),
		fmt.Sprintf("%.2f", t.ActualDays),
		fmt.Sprintf("%.2f", varianceRatio),
		fmt.Sprintf("%.2f", min),
		fmt.Sprintf("%.2f", max),
		strconv.FormatBool(withinExpected),
		policy.String(),
		strconv.Itoa(sprintNum),
		strconv.Itoa(t.StartedTick),
		strconv.Itoa(t.CompletedTick),
		fmt.Sprintf("%.2f", leadTime),
		fmt.Sprintf("%.2f", t.PhaseEffortSpent[model.PhaseResearch]),
		fmt.Sprintf("%.2f", t.PhaseEffortSpent[model.PhaseSizing]),
		fmt.Sprintf("%.2f", t.PhaseEffortSpent[model.PhasePlanning]),
		fmt.Sprintf("%.2f", t.PhaseEffortSpent[model.PhaseImplement]),
		fmt.Sprintf("%.2f", t.PhaseEffortSpent[model.PhaseVerify]),
		fmt.Sprintf("%.2f", t.PhaseEffortSpent[model.PhaseCICD]),
		fmt.Sprintf("%.2f", t.PhaseEffortSpent[model.PhaseReview]),
		fmt.Sprintf("%.2f", t.PhaseEffortSpent[model.PhaseDone]),
	}
}

// formatSprintRow formats a sprint for CSV export
func formatSprintRow(s model.Sprint, ticketsStarted, ticketsCompleted, incidents int) []string {
	return []string{
		strconv.Itoa(s.Number),
		strconv.Itoa(s.DurationDays),
		fmt.Sprintf("%.2f", s.BufferDays),
		fmt.Sprintf("%.2f", s.BufferConsumed),
		fmt.Sprintf("%.2f", s.BufferPctUsed()*100),
		s.FeverStatus.String(),
		strconv.Itoa(ticketsStarted),
		strconv.Itoa(ticketsCompleted),
		strconv.Itoa(incidents),
		strconv.Itoa(s.MaxWIP),
		fmt.Sprintf("%.2f", s.AvgWIP()),
	}
}

// formatIncidentRow formats an incident for CSV export
func formatIncidentRow(i model.Incident, sprintNum int) []string {
	resolvedAt := ""
	mttrDays := ""
	if i.ResolvedAt != nil {
		resolvedAt = i.ResolvedAt.Format("2006-01-02T15:04:05")
		mttrDays = fmt.Sprintf("%.2f", i.DaysToResolve())
	}

	return []string{
		i.ID,
		i.TicketID,
		i.Severity.String(),
		i.CreatedAt.Format("2006-01-02T15:04:05"),
		resolvedAt,
		mttrDays,
		strconv.Itoa(sprintNum),
	}
}
