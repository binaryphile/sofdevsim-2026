package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/binaryphile/fluentfp/either"
	"github.com/binaryphile/fluentfp/must"
	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/fluentfp/rslt"
	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/fluentfp/web"
	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/lessons"
	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
	"github.com/binaryphile/sofdevsim-2026/internal/office"
	"github.com/binaryphile/sofdevsim-2026/internal/registry"
)

// HealthResponse is returned by the /health endpoint for service identification.
type HealthResponse struct {
	Service string `json:"service"`
	Version string `json:"version"`
}

// handleHealth returns service identification for discovery.
// TUI uses this to verify it's connecting to a sofdevsim server. Stays as
// stdlib http.HandlerFunc (no domain-error mapping; bypasses web.Adapt).
func handleHealth(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(HealthResponse{
		Service: "sofdevsim",
		Version: "1.0",
	})
}

// EntryPointResponse is the HATEOAS discovery response.
type EntryPointResponse struct {
	Links map[string]string `json:"_links"`
}

// HandleEntryPoint returns HATEOAS discovery links for API navigation.
// This is the API root - clients start here and follow links.
func handleEntryPoint(r SimRegistry) web.Handler {
	_ = r // unused; included for consistency with the routes-table factory pattern
	return func(req *http.Request) rslt.Result[web.Response] {
		return rslt.Ok(web.Response{
			Status: http.StatusOK,
			Body: EntryPointResponse{
				Links: map[string]string{
					"self":        "/",
					"simulations": "/simulations",
					"comparisons": "/comparisons",
				},
			},
		})
	}
}

// SimulationListItem is a simulation entry in the list response.
type SimulationListItem struct {
	ID    string            `json:"id"`
	Links map[string]string `json:"_links"`
}

// toSimulationListItem converts registry.SimulationSummary to SimulationListItem.
func toSimulationListItem(s registry.SimulationSummary) SimulationListItem {
	return SimulationListItem{
		ID:    s.ID,
		Links: map[string]string{"self": "/simulations/" + s.ID},
	}
}

// SimulationListResponse is the response for GET /simulations.
type SimulationListResponse struct {
	Simulations []SimulationListItem `json:"simulations"`
	Links       map[string]string    `json:"_links"`
}

// handleListSimulations returns all active simulations with their IDs and links.
// Per UC10: "API client lists active simulations to discover available IDs"
func handleListSimulations(r SimRegistry) web.Handler {
	return func(req *http.Request) rslt.Result[web.Response] {
		summaries := r.ListSimulations()
		items := slice.Map(summaries, toSimulationListItem)
		return rslt.Ok(web.Response{
			Status: http.StatusOK,
			Body: SimulationListResponse{
				Simulations: items,
				Links:       map[string]string{"self": "/simulations"},
			},
		})
	}
}

// CreateSimulationRequest is the request body for creating a simulation.
type CreateSimulationRequest struct {
	Seed         int64  `json:"seed"`
	Policy       string `json:"policy,omitempty"`       // "none", "dora-strict", "tameflow-cognitive"
	ScenarioName string `json:"scenarioName,omitempty"` // UC37: backlog mix profile; default "healthy"
	// UC38: per-phase WIP caps; nil/omitted → unlimited (regression-safe).
	// Keys are canonical WorkflowPhase.String() output ("Research", "Sizing",
	// "Planning", "Implement", "Verify", "CI/CD", "Review"). Validation errors
	// map to HTTP 422 (Unprocessable Entity).
	PhaseWIPConfig map[string]int `json:"phaseWIPConfig,omitempty"`
	// UC39: release mode; "push" (default) or "demand". Empty/omitted → push
	// (regression-safe). Invalid values → HTTP 422 via ErrInvalidReleaseMode.
	ReleaseMode string `json:"releaseMode,omitempty"`
}

// handleCreateSimulation creates a new simulation with the given seed and policy.
// Returns the initial state with links to start a sprint.
// Error mapping: validation/parsing errors return typed *web.Error directly;
// domain sentinels (ErrAlreadyExists/ErrUnknownScenario/ErrCap*) flow through
// domainErrorMapper at the Adapt boundary.
func handleCreateSimulation(r SimRegistry) web.Handler {
	return func(req *http.Request) rslt.Result[web.Response] {
		var body CreateSimulationRequest
		if err := decodeJSON(req, &body); err != nil {
			return rslt.Err[web.Response](decodeError(err))
		}
		if err := isValidSeed(body.Seed); err != nil {
			return rslt.Err[web.Response](web.BadRequest(err.Error()))
		}
		policy := model.PolicyDORAStrict
		switch body.Policy {
		case "none":
			policy = model.PolicyNone
		case "tameflow-cognitive":
			policy = model.PolicyTameFlowCognitive
		case "dora-strict", "":
			policy = model.PolicyDORAStrict
		default:
			return rslt.Err[web.Response](web.BadRequest("invalid policy"))
		}
		phaseWIPConfig, parseErr := parsePhaseWIPConfig(body.PhaseWIPConfig)
		if parseErr != nil {
			return rslt.Err[web.Response](web.BadRequest(parseErr.Error()))
		}
		releaseMode, modeErr := model.ParseReleaseMode(body.ReleaseMode)
		if modeErr != nil {
			return rslt.Err[web.Response](&web.Error{Status: http.StatusUnprocessableEntity, Message: modeErr.Error(), Code: "INVALID_RELEASE_MODE"})
		}
		id, err := r.CreateSimulation(body.Seed, policy, body.ScenarioName, phaseWIPConfig, releaseMode)
		if err != nil {
			// Domain sentinels (ErrAlreadyExists / ErrUnknownScenario / ErrCap*)
			// flow through domainErrorMapper at the Adapt boundary. Other
			// errors fall through to the 500 default per Adapt's error flow.
			return rslt.Err[web.Response](err)
		}
		inst, _ := r.GetInstanceOption(id).Get()
		return rslt.Ok(simulationResponse(inst, http.StatusCreated))
	}
}

// parsePhaseWIPConfig translates a string-keyed PhaseWIPConfig map (REST
// surface form) into a WorkflowPhase-keyed map (domain form). Returns nil
// for nil/empty input (preserves regression-safe default). Accepts the
// canonical WorkflowPhase.String() output AND the slash-free "CICD" alias
// for "CI/CD"; matching is case-insensitive (consistent with the CLI
// parser per design.md §"Per-Phase WIP Caps").
func parsePhaseWIPConfig(in map[string]int) (map[model.WorkflowPhase]int, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make(map[model.WorkflowPhase]int, len(in))
	for k, v := range in {
		phase, ok := parsePhaseName(k)
		if !ok {
			return nil, fmt.Errorf("unknown phase %q (valid: Research, Sizing, Planning, Implement, Verify, CI/CD or CICD, Review)", k)
		}
		out[phase] = v
	}
	return out, nil
}

// parsePhaseName maps an operator-supplied phase string to WorkflowPhase.
// Case-insensitive; accepts both "CI/CD" and "CICD" for the CI/CD phase.
func parsePhaseName(s string) (model.WorkflowPhase, bool) {
	switch strings.ToUpper(s) {
	case "RESEARCH":
		return model.PhaseResearch, true
	case "SIZING":
		return model.PhaseSizing, true
	case "PLANNING":
		return model.PhasePlanning, true
	case "IMPLEMENT":
		return model.PhaseImplement, true
	case "VERIFY":
		return model.PhaseVerify, true
	case "CI/CD", "CICD":
		return model.PhaseCICD, true
	case "REVIEW":
		return model.PhaseReview, true
	}
	return 0, false
}

// handleGetSimulation returns the current state of a simulation.
// Includes context-appropriate links based on whether a sprint is active.
func handleGetSimulation(r SimRegistry) web.Handler {
	return func(req *http.Request) rslt.Result[web.Response] {
		inst, ok := r.GetInstanceOption(req.PathValue("id")).Get()
		if !ok {
			return rslt.Err[web.Response](web.NotFound("simulation not found"))
		}
		return rslt.Ok(simulationResponse(inst, http.StatusOK))
	}
}

// handleStartSprint starts a new sprint for the simulation.
// Returns 409 Conflict if a sprint is already active OR the 3-retry
// optimistic-concurrency loop is exhausted.
func handleStartSprint(r SimRegistry) web.Handler {
	return func(req *http.Request) rslt.Result[web.Response] {
		id := req.PathValue("id")
		const maxRetries = 3
		for attempt := 0; attempt < maxRetries; attempt++ { // justified:CF
			inst, ok := r.GetInstanceOption(id).Get()
			if !ok {
				return rslt.Err[web.Response](web.NotFound("simulation not found"))
			}
			sim := inst.Engine.Sim()
			if _, active := sim.CurrentSprintOption.Get(); active {
				return rslt.Err[web.Response](web.Conflict("sprint already active"))
			}
			var err error
			if inst.Engine, err = inst.Engine.StartSprint(); err != nil {
				continue
			}
			r.SetInstance(id, inst)
			return rslt.Ok(simulationResponse(inst, http.StatusOK))
		}
		return rslt.Err[web.Response](web.Conflict("conflict"))
	}
}

// handleTick advances the simulation by one tick.
// Returns updated state with context-appropriate links (tick disappears when sprint ends).
func handleTick(r SimRegistry) web.Handler {
	return func(req *http.Request) rslt.Result[web.Response] {
		id := req.PathValue("id")
		const maxRetries = 3
		for attempt := 0; attempt < maxRetries; attempt++ { // justified:CF
			inst, ok := r.GetInstanceOption(id).Get()
			if !ok {
				return rslt.Err[web.Response](web.NotFound("simulation not found"))
			}
			oldSim := inst.Engine.Sim()
			if _, active := oldSim.CurrentSprintOption.Get(); !active {
				return rslt.Err[web.Response](web.Conflict("no active sprint"))
			}
			var err error
			if inst.Engine, _, err = inst.Engine.Tick(); err != nil {
				continue
			}
			newSim := inst.Engine.Sim()
			inst.Tracker = inst.Tracker.Updated(newSim)
			inst.Office = deriveOfficeEvents(inst.Office, oldSim, newSim)
			r.SetInstance(id, inst)
			return rslt.Ok(simulationResponse(inst, http.StatusOK))
		}
		return rslt.Err[web.Response](web.Conflict("conflict"))
	}
}

// AssignTicketRequest is the request body for assigning a ticket.
type AssignTicketRequest struct {
	TicketID    string `json:"ticketId"`
	DeveloperID string `json:"developerId"`
}

// handleAssignTicket assigns a ticket to a developer.
// If developerId is omitted, auto-assigns to first idle developer.
// Returns 400 if ticket/developer not found or developer is busy.
func handleAssignTicket(r SimRegistry) web.Handler {
	return func(req *http.Request) rslt.Result[web.Response] {
		id := req.PathValue("id")
		inst, ok := r.GetInstanceOption(id).Get()
		if !ok {
			return rslt.Err[web.Response](web.NotFound("simulation not found"))
		}
		var body AssignTicketRequest
		if err := decodeJSON(req, &body); err != nil {
			return rslt.Err[web.Response](decodeError(err))
		}
		devID := body.DeveloperID
		if devID == "" {
			sim := inst.Engine.Sim()
			idle := sim.IdleDevelopers()
			if len(idle) == 0 {
				return rslt.Err[web.Response](web.BadRequest("no idle developers"))
			}
			devID = idle[0].ID
		}
		var err error
		inst.Engine, err = inst.Engine.AssignTicket(body.TicketID, devID)
		if err != nil {
			return rslt.Err[web.Response](web.BadRequest(err.Error()))
		}
		sim := inst.Engine.Sim()
		devIdx := findDeveloperIndex(sim.Developers, devID)
		now := time.Now()
		if devIdx >= 0 {
			target := office.CubicleLayout(len(sim.Developers))[devIdx]
			inst.Office = inst.Office.Record(office.DevAssignedToTicket{
				DevID:    devID,
				TicketID: body.TicketID,
				Target:   target,
			}, sim.CurrentTick, now)
			inst.Office = inst.Office.Record(office.DevStartedWorking{DevID: devID}, sim.CurrentTick, now)
		}
		r.SetInstance(id, inst)
		return rslt.Ok(simulationResponse(inst, http.StatusOK))
	}
}

// findDeveloperIndex returns the index of a developer by ID, or -1 if not found.
func findDeveloperIndex(devs []model.Developer, id string) int {
	for i := range devs { // justified:IX
		if devs[i].ID == id {
			return i
		}
	}
	return -1
}

// handleCompare runs two simulations with different policies and compares them.
// Returns DORA metrics and per-metric winners. The comparison-orchestration
// helpers (addStandardTeam, addStandardBacklog, runSprintsWithTracking,
// runComparison, autoAssignForComparison, decomposeEligibleTickets,
// buildCompareResponse, buildPolicyResult, buildWinners) live below the
// handler at file scope per the plan §"Out of scope" decision (not refactored
// into a separate package; transport-shape migration only).
func handleCompare(r SimRegistry) web.Handler {
	_ = r // unused; comparison runs are stateless (no registry interaction)
	return func(req *http.Request) rslt.Result[web.Response] {
		var body CompareRequest
		if err := decodeJSON(req, &body); err != nil {
			return rslt.Err[web.Response](decodeError(err))
		}
		seed := body.Seed
		if seed == 0 {
			seed = time.Now().UnixNano()
		}
		if err := isValidSeed(seed); err != nil {
			return rslt.Err[web.Response](web.BadRequest(err.Error()))
		}
		sprints := body.Sprints
		if sprints == 0 {
			sprints = 3
		}
		if sprints < 0 {
			return rslt.Err[web.Response](web.BadRequest("invalid sprints count"))
		}
		resultA := runComparison(model.PolicyDORAStrict, seed, sprints)
		resultB := runComparison(model.PolicyTameFlowCognitive, seed, sprints)
		comparison := metrics.Compare(resultA, resultB, seed)
		return rslt.Ok(web.Response{
			Status: http.StatusOK,
			Body:   buildCompareResponse(seed, sprints, comparison),
		})
	}
}

// addStandardTeam adds the fixed 3-developer team for comparison runs.
// Calculation: pure transformation (engine in → engine out).
func addStandardTeam(eng engine.Engine) engine.Engine {
	eng = must.Get(eng.AddDeveloper("dev-1", "Alice", 1.0))
	eng = must.Get(eng.AddDeveloper("dev-2", "Bob", 0.8))
	eng = must.Get(eng.AddDeveloper("dev-3", "Carol", 1.2))
	return eng
}

// addStandardBacklog adds the fixed 5-ticket backlog for comparison runs.
// Calculation: pure transformation (engine in → engine out).
func addStandardBacklog(eng engine.Engine) engine.Engine {
	eng = must.Get(eng.AddTicket(model.NewTicket("TKT-001", "Small clear", 2, model.HighUnderstanding)))
	eng = must.Get(eng.AddTicket(model.NewTicket("TKT-002", "Medium clear", 4, model.HighUnderstanding)))
	eng = must.Get(eng.AddTicket(model.NewTicket("TKT-003", "Small unclear", 2, model.LowUnderstanding)))
	eng = must.Get(eng.AddTicket(model.NewTicket("TKT-004", "Large unclear", 8, model.LowUnderstanding)))
	eng = must.Get(eng.AddTicket(model.NewTicket("TKT-005", "Medium mixed", 5, model.MediumUnderstanding)))
	return eng
}

// runSprintsWithTracking runs N sprints and returns final metrics result.
// Calculation: pure transformation (engine in → metrics out).
func runSprintsWithTracking(eng engine.Engine, policy model.SizingPolicy, sprints int) metrics.SimulationResult {
	tracker := metrics.NewTracker()
	for i := 0; i < sprints; i++ {
		// Decompose eligible tickets before sprint start (while still in backlog)
		eng = decomposeEligibleTickets(eng)
		// StartSprint triages + commits tickets
		eng = must.Get(eng.StartSprint())
		// Auto-assign committed tickets to idle developers
		eng = autoAssignForComparison(eng)
		// Run sprint ticks
		sprint := eng.Sim().CurrentSprintOption.MustGet()
		for eng.Sim().CurrentTick < sprint.EndDay { // justified:CF
			eng, _ = must.Get2(eng.Tick())
			// Mid-sprint auto-assign for newly idle devs
			eng = autoAssignForComparison(eng)
		}
		tracker = tracker.Updated(eng.Sim())
	}
	state := eng.Sim()
	return tracker.GetResult(policy, state)
}

// runComparison runs a single simulation with the given policy.
// Calculation: pure transformation (policy, seed, sprints → metrics).
func runComparison(policy model.SizingPolicy, seed int64, sprints int) metrics.SimulationResult {
	simID := fmt.Sprintf("cmp-%d", seed)
	sim := model.NewSimulation(simID, policy, seed)

	eng := engine.NewEngine(sim.Seed)
	eng = must.Get(eng.EmitCreated(sim.ID, sim.CurrentTick, events.SimConfig{
		TeamSize:     3,
		SprintLength: sim.SprintLength,
		Seed:         sim.Seed,
		Policy:       policy,
	}))

	eng = addStandardTeam(eng)
	eng = addStandardBacklog(eng)
	return runSprintsWithTracking(eng, policy, sprints)
}

// autoAssignForComparison assigns backlog tickets to idle developers.
// Calculation: pure transformation (engine in → engine out).
// No event store, so errors are impossible (in-memory only).
func autoAssignForComparison(eng engine.Engine) engine.Engine {
	// First: decompose tickets that match policy criteria
	eng = decomposeEligibleTickets(eng)

	// Then: assign to idle developers from committed queue (or backlog for backward compat)
	state := eng.Sim()
	idleDevs := state.IdleDevelopers()
	for i := 0; i < len(idleDevs); i++ {
		state = eng.Sim()
		// Prefer committed tickets; fall back to backlog
		var ticketID string
		if len(state.CommittedTickets) > 0 {
			ticketID = state.CommittedTickets[0].ID
		} else if len(state.Backlog) > 0 {
			ticketID = state.Backlog[0].ID
		} else {
			break
		}
		dev := idleDevs[i]
		eng = must.Get(eng.AssignTicket(ticketID, dev.ID))
	}
	return eng
}

// decomposeEligibleTickets decomposes all backlog tickets matching policy criteria.
// Calculation: pure transformation (engine in → engine out).
//
// The loop continues until no tickets qualify, handling children created by
// earlier decompositions. Called once per sprint before ticket assignment.
// Idempotent: TryDecompose returns NotDecomposable for already-decomposed tickets.
//
// Error ignored: TryDecompose uses Either for domain outcomes (NotDecomposable vs children).
// Infrastructure errors are impossible in comparison mode (in-memory only).
func decomposeEligibleTickets(eng engine.Engine) engine.Engine {
	// Raw loop with break: exit as soon as ANY ticket decomposes since backlog
	// structure changed. Per FP Guide §16, fluentfp not suitable for early exit.
	for {
		state := eng.Sim()
		anyDecomposed := false
		for _, ticket := range state.Backlog {
			var result either.Either[engine.NotDecomposable, []model.Ticket]
			eng, result = must.Get2(eng.TryDecompose(ticket.ID))
			_, decomposed := result.Get()
			if decomposed {
				anyDecomposed = true
				break // Backlog changed, re-scan from start
			}
		}
		if !anyDecomposed {
			break
		}
	}
	return eng
}

// buildCompareResponse converts ComparisonResult to API response.
func buildCompareResponse(seed int64, sprints int, c metrics.ComparisonResult) CompareResponse {
	return CompareResponse{
		Seed:    seed,
		Sprints: sprints,
		PolicyA: buildPolicyResult(c.ResultsA),
		PolicyB: buildPolicyResult(c.ResultsB),
		Winners: buildWinners(c),
		WinsA:   c.WinsA,
		WinsB:   c.WinsB,
		Links: map[string]string{
			"self": "/comparisons",
		},
	}
}

// buildPolicyResult converts SimulationResult to PolicyResult.
func buildPolicyResult(r metrics.SimulationResult) PolicyResult {
	return PolicyResult{
		Name:            r.Policy.String(),
		TicketsComplete: r.TicketsComplete,
		IncidentCount:   r.IncidentCount,
		Metrics: DORAResponse{
			LeadTimeAvgDays:   r.FinalMetrics.LeadTimeAvgDays(),
			DeployFrequency:   r.FinalMetrics.DeployFrequency,
			MTTRAvgDays:       r.FinalMetrics.MTTRAvgDays(),
			ChangeFailRatePct: r.FinalMetrics.ChangeFailRatePct(),
		},
	}
}

// buildWinners converts ComparisonResult winners to MetricWinners.
func buildWinners(c metrics.ComparisonResult) MetricWinners {
	// policyName returns the policy name or "tie" for zero value.
	policyName := func(p model.SizingPolicy) string {
		if p == model.PolicyNone {
			return "tie"
		}
		return p.String()
	}

	overall := "tie"
	if !c.IsTie() {
		overall = c.OverallWinner.String()
	}

	return MetricWinners{
		LeadTime:        policyName(c.LeadTimeWinner),
		DeployFrequency: policyName(c.DeployFreqWinner),
		MTTR:            policyName(c.MTTRWinner),
		ChangeFailRate:  policyName(c.CFRWinner),
		Overall:         overall,
	}
}

// UpdateSimulationRequest is the request body for updating simulation settings.
type UpdateSimulationRequest struct {
	Policy string `json:"policy,omitempty"`
}

// handleUpdateSimulation updates simulation settings (currently just policy).
// Returns 400 if invalid policy, 404 if simulation not found.
func handleUpdateSimulation(r SimRegistry) web.Handler {
	return func(req *http.Request) rslt.Result[web.Response] {
		id := req.PathValue("id")
		var body UpdateSimulationRequest
		if err := decodeJSON(req, &body); err != nil {
			return rslt.Err[web.Response](decodeError(err))
		}
		var policy model.SizingPolicy
		switch body.Policy {
		case "none":
			policy = model.PolicyNone
		case "dora-strict":
			policy = model.PolicyDORAStrict
		case "tameflow-cognitive":
			policy = model.PolicyTameFlowCognitive
		default:
			return rslt.Err[web.Response](web.BadRequest("invalid policy"))
		}
		const maxRetries = 3
		for attempt := 0; attempt < maxRetries; attempt++ { // justified:CF
			inst, ok := r.GetInstanceOption(id).Get()
			if !ok {
				return rslt.Err[web.Response](web.NotFound("simulation not found"))
			}
			var err error
			if inst.Engine, err = inst.Engine.SetPolicy(policy); err != nil {
				continue
			}
			r.SetInstance(id, inst)
			return rslt.Ok(simulationResponse(inst, http.StatusOK))
		}
		return rslt.Err[web.Response](web.Conflict("conflict"))
	}
}

// DecomposeRequest is the request body for ticket decomposition.
type DecomposeRequest struct {
	TicketID string `json:"ticketId"`
}

// DecomposeResponse is the response from decomposing a ticket.
type DecomposeResponse struct {
	Decomposed bool              `json:"decomposed"`
	Children   []TicketState     `json:"children,omitempty"`
	Simulation SimulationState   `json:"simulation"`
	Links      map[string]string `json:"_links"`
}

// handleDecompose attempts to decompose a ticket into smaller tasks.
// Returns decomposed=false if ticket not found or policy doesn't allow decomposition.
func handleDecompose(r SimRegistry) web.Handler {
	toTicketStates := func(tickets []model.Ticket) []TicketState {
		return slice.Map(tickets, ToTicketState)
	}
	return func(req *http.Request) rslt.Result[web.Response] {
		id := req.PathValue("id")
		var body DecomposeRequest
		if err := decodeJSON(req, &body); err != nil {
			return rslt.Err[web.Response](decodeError(err))
		}
		const maxRetries = 3
		for attempt := 0; attempt < maxRetries; attempt++ { // justified:CF
			inst, ok := r.GetInstanceOption(id).Get()
			if !ok {
				return rslt.Err[web.Response](web.NotFound("simulation not found"))
			}
			var result either.Either[engine.NotDecomposable, []model.Ticket]
			var err error
			if inst.Engine, result, err = inst.Engine.TryDecompose(body.TicketID); err != nil {
				continue
			}
			r.SetInstance(id, inst)
			children, decomposed := result.Get()
			state := ToState(inst.Engine.Sim(), inst.Tracker)
			return rslt.Ok(web.Response{
				Status: http.StatusOK,
				Body: DecomposeResponse{
					Decomposed: decomposed,
					Children:   toTicketStates(children),
					Simulation: state,
					Links:      LinksFor(state),
				},
			})
		}
		return rslt.Err[web.Response](web.Conflict("conflict"))
	}
}

// handleGetLessons returns contextual lessons for a simulation.
// Reuses shared lesson selection logic - API is stateless so always starts fresh.
func handleGetLessons(r SimRegistry) web.Handler {
	return func(req *http.Request) rslt.Result[web.Response] {
		id := req.PathValue("id")
		inst, ok := r.GetInstanceOption(id).Get()
		if !ok {
			return rslt.Err[web.Response](web.NotFound("simulation not found"))
		}
		sim := inst.Engine.Sim()
		_, hasActiveSprint := sim.CurrentSprintOption.Get()
		var view lessons.ViewContext
		if hasActiveSprint {
			view = lessons.ViewExecution
		} else if len(sim.CompletedTickets) > 0 {
			view = lessons.ViewMetrics
		} else {
			view = lessons.ViewPlanning
		}
		lesson := lessons.Select(view, lessons.State{}, hasActiveSprint, false, lessons.TriggerState{}, lessons.ComparisonSummary{})
		return rslt.Ok(web.Response{
			Status: http.StatusOK,
			Body: LessonsResponse{
				CurrentLesson: LessonResponse{
					ID:      string(lesson.ID),
					Title:   lesson.Title,
					Content: lesson.Content,
					Tips:    lesson.Tips,
				},
				Progress: "0/8 concepts",
				Links: map[string]string{
					"self":       "/simulations/" + id + "/lessons",
					"simulation": "/simulations/" + id,
				},
			},
		})
	}
}

// deriveOfficeEvents compares old and new simulation states to derive office animation events.
// Returns updated OfficeProjection with events recorded.
// Calculation: (OfficeProjection, oldSim, newSim) → OfficeProjection
func deriveOfficeEvents(proj office.OfficeProjection, oldSim, newSim model.Simulation) office.OfficeProjection {
	tick := newSim.CurrentTick
	now := time.Now()

	// Clear previous tick's bubbles before checking for new frustration
	proj = proj.Record(office.BubblesExpired{}, tick, now)

	// Check for sprint end - all developers return to conference
	oldSprintActive := oldSim.CurrentSprintOption.IsOk()
	newSprintActive := newSim.CurrentSprintOption.IsOk()
	if oldSprintActive && !newSprintActive {
		for _, dev := range newSim.Developers { // justified:SM
			proj = proj.Record(office.DevEnteredConference{DevID: dev.ID}, tick, now)
		}
		return proj
	}

	// Check each developer for state changes
	for _, newDev := range newSim.Developers {
		oldDev, ok := findDeveloper(oldSim.Developers, newDev.ID).Get()
		if !ok {
			continue
		}

		// Developer became idle (completed ticket)
		if !oldDev.IsIdle() && newDev.IsIdle() {
			proj = proj.Record(office.DevCompletedTicket{
				DevID:    newDev.ID,
				TicketID: oldDev.CurrentTicket,
			}, tick, now)
			continue
		}

		// Developer still working - check for frustration
		if !newDev.IsIdle() {
			if ticket, ok := findActiveTicket(newSim.ActiveTickets, newDev.CurrentTicket).Get(); ok && ticket.ActualDays > ticket.EstimatedDays {
				// Check if already frustrated
				if anim, ok := proj.State().GetAnimationOption(newDev.ID).Get(); ok {
					if anim.State != office.StateFrustrated {
						proj = proj.Record(office.DevBecameFrustrated{
							DevID:    newDev.ID,
							TicketID: ticket.ID,
						}, tick, now)
					}
				}
			}
		}
	}

	return proj
}

// findDeveloper finds a developer by ID in a slice.
func findDeveloper(devs []model.Developer, id string) option.Option[model.Developer] {
	// hasID returns true if developer ID matches.
	hasID := func(d model.Developer) bool { return d.ID == id }
	return slice.From(devs).Find(hasID)
}

// findActiveTicket finds a ticket by ID in the active tickets slice.
func findActiveTicket(tickets []model.Ticket, id string) option.Option[model.Ticket] {
	// hasID returns true if ticket ID matches.
	hasID := func(t model.Ticket) bool { return t.ID == id }
	return slice.From(tickets).Find(hasID)
}

// handleGetOffice returns the office animation state for programmatic assertions.
func handleGetOffice(r SimRegistry) web.Handler {
	return func(req *http.Request) rslt.Result[web.Response] {
		id := req.PathValue("id")
		inst, ok := r.GetInstanceOption(id).Get()
		if !ok {
			return rslt.Err[web.Response](web.NotFound("simulation not found"))
		}
		sim := inst.Engine.Sim()
		state := inst.Office.State()
		names := slice.From(sim.Developers).ToString(model.Developer.GetName)
		devStates := make([]DeveloperAnimationState, 0, len(state.Animations))
		for i, anim := range state.Animations { // justified:IX
			name := ""
			if i < len(names) {
				name = names[i]
			}
			colorName := ""
			if anim.ColorIndex < len(office.DeveloperColorNames) {
				colorName = office.DeveloperColorNames[anim.ColorIndex]
			}
			ticketID := ""
			if dev, ok := findDeveloper(sim.Developers, anim.DevID).Get(); ok {
				ticketID = dev.CurrentTicket
			}
			devStates = append(devStates, DeveloperAnimationState{
				DevID:     anim.DevID,
				DevName:   name,
				State:     anim.State.String(),
				ColorName: colorName,
				TicketID:  ticketID,
			})
		}
		transitions := inst.Office.Transitions()
		recentCount := 10
		if len(transitions) < recentCount {
			recentCount = len(transitions)
		}
		recentTransitions := make([]StateTransitionResponse, recentCount)
		for i := 0; i < recentCount; i++ { // justified:IX
			t := transitions[len(transitions)-recentCount+i]
			recentTransitions[i] = StateTransitionResponse{
				DevID:     t.DevID,
				FromState: t.FromState,
				ToState:   t.ToState,
				Tick:      t.Tick,
				Timestamp: t.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
				Reason:    t.Reason,
			}
		}
		return rslt.Ok(web.Response{
			Status: http.StatusOK,
			Body: OfficeResponse{
				Developers:  devStates,
				Transitions: recentTransitions,
				CurrentTick: inst.Office.CurrentTick(),
				Links: map[string]string{
					"self":       "/simulations/" + id + "/office",
					"simulation": "/simulations/" + id,
				},
			},
		})
	}
}

// SpendInvestmentRequest is the request body for POST /simulations/{id}/investments.
// UC40 #15446.
type SpendInvestmentRequest struct {
	// Option is the operator-supplied kebab-case option name; must match
	// one of "hire", "cicd-slot", "review-tool", "verify-paydown".
	// ParseInvestmentOption is case-insensitive.
	Option string `json:"option"`
}

// HandleSpendInvestment dispatches a between-sprint investment per UC40.
// Returns 201 on success, 422 for ErrInsufficientBudget + ErrInvalidInvestment,
// 409 for ErrInvestmentWindowClosed (state conflict), 404 if simulation doesn't
// exist. UC40 #15446.
// handleSpendInvestment dispatches a between-sprint investment per UC40 #15446.
// Sentinel-mapped errors (ErrInsufficientBudget/ErrInvestmentWindowClosed/
// ErrInvalidInvestment) flow through domainErrorMapper at the Adapt boundary.
// ParseInvestmentOption errors return typed *web.Error directly (bypass mapper).
func handleSpendInvestment(r SimRegistry) web.Handler {
	return func(req *http.Request) rslt.Result[web.Response] {
		id := req.PathValue("id")
		if _, ok := r.GetInstanceOption(id).Get(); !ok {
			return rslt.Err[web.Response](web.NotFound("simulation not found"))
		}
		var body SpendInvestmentRequest
		if err := decodeJSON(req, &body); err != nil {
			return rslt.Err[web.Response](decodeError(err))
		}
		option, parseErr := model.ParseInvestmentOption(body.Option)
		if parseErr != nil {
			return rslt.Err[web.Response](&web.Error{Status: http.StatusUnprocessableEntity, Message: parseErr.Error(), Code: "INVALID_INVESTMENT"})
		}
		if err := r.SpendInvestment(id, option); err != nil {
			// Sentinels flow through domainErrorMapper at Adapt boundary
			// (ErrInsufficientBudget → 422, ErrInvestmentWindowClosed → 409,
			// ErrInvalidInvestment → 422). Unmapped errors fall to 500 default.
			return rslt.Err[web.Response](err)
		}
		inst, _ := r.GetInstanceOption(id).Get()
		return rslt.Ok(simulationResponse(inst, http.StatusCreated))
	}
}
