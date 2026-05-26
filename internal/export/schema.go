package export

import (
	"encoding/json"
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
		"ticket_id", "title", "understanding", "ticket_type", "estimated_days", "actual_days",
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
		// UC38: per-phase WIP context. phase_wip_caps holds the JSON-serialized
		// cap config (e.g. {"Implement":4,"CICD":1}); phase_avg_wip holds
		// per-phase avg WIP across the sprint (currently emitted as JSON null
		// since sprint-level per-phase WIP averaging is deferred — column is
		// present for forward-compat with the documented schema).
		"phase_wip_caps", "phase_avg_wip",
		// UC39: release mode + TOC analyzer snapshot. release_mode is the
		// sim's mode at sprint time ("push"|"demand"). constraint_phase is
		// the TOC analyzer's currently-locked constraint phase name (or
		// "none" if no constraint locked). buffer_penetration is the
		// constraint buffer's penetration value [0,1] (or "null" if no
		// constraint locked).
		"release_mode", "constraint_phase", "buffer_penetration",
		// UC40: investment-window state. budget_remaining is the sim's
		// Budget value at sprint export time (int). investment_applied is
		// the option name of the most-recent InvestmentApplied event since
		// the last sprint started ("hire"|"cicd-slot"|"review-tool"|
		// "verify-paydown"), or "none" if no investment occurred.
		"budget_remaining", "investment_applied",
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
		t.Type.String(), // UC37: ticket_type column (positioned between understanding and estimated_days per plan §Commit 9)
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

// formatSprintRow formats a sprint for CSV export. phaseWIPConfig is the
// sim's UC38 PhaseWIPConfig (serialized as JSON with string keys for
// portability). phaseAvgWIP is emitted as `null` JSON in UC38 — sprint-
// level per-phase WIP averaging is deferred (schema reserves the column
// for forward-compat). UC39 appends releaseMode + constraintPhase +
// bufferPenetration columns; bufferPenetration emits "null" when no
// constraint is locked. UC40 appends budgetRemaining + investmentApplied
// columns; investmentApplied is "none" if no investment fired since the
// last sprint.
func formatSprintRow(s model.Sprint, ticketsStarted, ticketsCompleted, incidents int, phaseWIPConfig map[model.WorkflowPhase]int, releaseMode string, constraintPhase string, bufferPenetration string, budgetRemaining int, investmentApplied string) []string {
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
		serializePhaseWIPConfig(phaseWIPConfig), // UC38 phase_wip_caps
		"null",                                   // UC38 phase_avg_wip (deferred)
		releaseMode,                              // UC39 release_mode
		constraintPhase,                          // UC39 constraint_phase
		bufferPenetration,                        // UC39 buffer_penetration
		strconv.Itoa(budgetRemaining),            // UC40 budget_remaining
		investmentApplied,                        // UC40 investment_applied
	}
}

// serializePhaseWIPConfig emits the UC38 cap config as JSON with string
// keys (canonical phase names; "CI/CD" included verbatim — not the
// slash-free CICD alias, since CSV consumers re-key by exact match).
// Returns "{}" for nil/empty so the column is always a valid JSON value.
func serializePhaseWIPConfig(cfg map[model.WorkflowPhase]int) string {
	out := make(map[string]int, len(cfg))
	for k, v := range cfg {
		out[k.String()] = v
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "{}"
	}
	return string(b)
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
