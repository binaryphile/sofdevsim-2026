package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/binaryphile/fluentfp/either"
	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/lessons"
	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
	"github.com/binaryphile/sofdevsim-2026/internal/registry"
)

// respondWithSimulation writes the HAL response for a simulation instance.
// Query: builds read model from instance state.
// Per CQRS Guide §Query Side: queries should be clearly separated from commands.
// Note: Contains pure calculations (ToState, LinksFor) then Action (writeJSON) -
// acceptable I/O boundary layer per FP Guide.
func respondWithSimulation(w http.ResponseWriter, inst registry.SimInstance, status int) {
	state := ToState(inst.Engine.Sim(), inst.Tracker)
	response := HALResponse{
		State: state,
		Links: LinksFor(state),
	}
	writeJSON(w, status, response)
}

// HealthResponse is returned by the /health endpoint for service identification.
type HealthResponse struct {
	Service string `json:"service"`
	Version string `json:"version"`
}

// handleHealth returns service identification for discovery.
// TUI uses this to verify it's connecting to a sofdevsim server.
func handleHealth(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(HealthResponse{
		Service: "sofdevsim",
		Version: "1.0",
	})
}

// writeJSON writes a HAL+JSON response with proper content type and status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/hal+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response with the given status and message.
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// EntryPointResponse is the HATEOAS discovery response.
type EntryPointResponse struct {
	Links map[string]string `json:"_links"`
}

// HandleEntryPoint returns HATEOAS discovery links for API navigation.
// This is the API root - clients start here and follow links.
func (r SimRegistry) HandleEntryPoint(w http.ResponseWriter, req *http.Request) {
	response := EntryPointResponse{
		Links: map[string]string{
			"self":        "/",
			"simulations": "/simulations",
			"comparisons": "/comparisons",
		},
	}
	writeJSON(w, http.StatusOK, response)
}

// SimulationListItem is a simulation entry in the list response.
type SimulationListItem struct {
	ID    string            `json:"id"`
	Links map[string]string `json:"_links"`
}

// SimulationListResponse is the response for GET /simulations.
type SimulationListResponse struct {
	Simulations []SimulationListItem `json:"simulations"`
	Links       map[string]string    `json:"_links"`
}

// HandleListSimulations returns all active simulations with their IDs and links.
// Per UC10: "API client lists active simulations to discover available IDs"
func (r SimRegistry) HandleListSimulations(w http.ResponseWriter, req *http.Request) {
	summaries := r.ListSimulations()

	items := make([]SimulationListItem, len(summaries))
	for i, s := range summaries {
		items[i] = SimulationListItem{
			ID: s.ID,
			Links: map[string]string{
				"self": "/simulations/" + s.ID,
			},
		}
	}

	response := SimulationListResponse{
		Simulations: items,
		Links: map[string]string{
			"self": "/simulations",
		},
	}
	writeJSON(w, http.StatusOK, response)
}

// CreateSimulationRequest is the request body for creating a simulation.
type CreateSimulationRequest struct {
	Seed   int64  `json:"seed"`
	Policy string `json:"policy,omitempty"` // "none", "dora-strict", "tameflow-cognitive"
}

// HandleCreateSimulation creates a new simulation with the given seed and policy.
// Returns the initial state with links to start a sprint.
func (r SimRegistry) HandleCreateSimulation(w http.ResponseWriter, req *http.Request) {
	var body CreateSimulationRequest
	if err := decodeJSON(req, &body); err != nil {
		respondDecodeError(w, err)
		return
	}

	// Validate seed (pure calculation)
	if err := isValidSeed(body.Seed); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Default policy
	policy := model.PolicyDORAStrict
	switch body.Policy {
	case "none":
		policy = model.PolicyNone
	case "tameflow-cognitive":
		policy = model.PolicyTameFlowCognitive
	case "dora-strict", "":
		policy = model.PolicyDORAStrict
	default:
		writeError(w, http.StatusBadRequest, "invalid policy")
		return
	}

	id, err := r.CreateSimulation(body.Seed, policy)
	if err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	inst, _ := r.GetInstance(id)
	respondWithSimulation(w, inst, http.StatusCreated)
}

// HandleGetSimulation returns the current state of a simulation.
// Includes context-appropriate links based on whether a sprint is active.
func (r SimRegistry) HandleGetSimulation(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	inst, ok := r.GetInstance(id)
	if !ok {
		writeError(w, http.StatusNotFound, "simulation not found")
		return
	}

	respondWithSimulation(w, inst, http.StatusOK)
}

// HandleStartSprint starts a new sprint for the simulation.
// Returns 409 Conflict if a sprint is already active.
func (r SimRegistry) HandleStartSprint(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		inst, ok := r.GetInstance(id)
		if !ok {
			writeError(w, http.StatusNotFound, "simulation not found")
			return
		}

		// Check if sprint already active
		sim := inst.Engine.Sim()
		if _, active := sim.CurrentSprintOption.Get(); active {
			writeError(w, http.StatusConflict, "sprint already active")
			return
		}

		var err error
		if inst.Engine, err = inst.Engine.StartSprint(); err != nil {
			continue // Retry with fresh state
		}
		r.SetInstance(id, inst)

		respondWithSimulation(w, inst, http.StatusOK)
		return
	}
	writeError(w, http.StatusConflict, "conflict")
}

// HandleTick advances the simulation by one tick.
// Returns updated state with context-appropriate links (tick disappears when sprint ends).
func (r SimRegistry) HandleTick(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		inst, ok := r.GetInstance(id)
		if !ok {
			writeError(w, http.StatusNotFound, "simulation not found")
			return
		}

		// Check if sprint is active
		sim := inst.Engine.Sim()
		if _, active := sim.CurrentSprintOption.Get(); !active {
			writeError(w, http.StatusConflict, "no active sprint")
			return
		}

		// Engine emits events and updates projection - capture new Engine
		var err error
		if inst.Engine, _, err = inst.Engine.Tick(); err != nil {
			continue // Retry with fresh state
		}

		// SprintEnded event clears sprint in projection automatically
		sim = inst.Engine.Sim()
		inst.Tracker = inst.Tracker.Updated(&sim)
		r.SetInstance(id, inst)

		respondWithSimulation(w, inst, http.StatusOK)
		return
	}
	writeError(w, http.StatusConflict, "conflict")
}

// AssignTicketRequest is the request body for assigning a ticket.
type AssignTicketRequest struct {
	TicketID    string `json:"ticketId"`
	DeveloperID string `json:"developerId"`
}

// HandleAssignTicket assigns a ticket to a developer.
// If developerId is omitted, auto-assigns to first idle developer.
// Returns 400 if ticket/developer not found or developer is busy.
func (r SimRegistry) HandleAssignTicket(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	inst, ok := r.GetInstance(id)
	if !ok {
		writeError(w, http.StatusNotFound, "simulation not found")
		return
	}

	var body AssignTicketRequest
	if err := decodeJSON(req, &body); err != nil {
		respondDecodeError(w, err)
		return
	}

	// Auto-assign if no developer specified
	devID := body.DeveloperID
	if devID == "" {
		sim := inst.Engine.Sim()
		idle := sim.IdleDevelopers()
		if len(idle) == 0 {
			writeError(w, http.StatusBadRequest, "no idle developers")
			return
		}
		devID = idle[0].ID
	}

	var err error
	inst.Engine, err = inst.Engine.AssignTicket(body.TicketID, devID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	r.SetInstance(id, inst)

	respondWithSimulation(w, inst, http.StatusOK)
}

// HandleCompare runs two simulations with different policies and compares them.
// Returns DORA metrics and per-metric winners.
func (r SimRegistry) HandleCompare(w http.ResponseWriter, req *http.Request) {
	var body CompareRequest
	if err := decodeJSON(req, &body); err != nil {
		respondDecodeError(w, err)
		return
	}

	// Defaults and validation
	seed := body.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	// Validate seed (pure calculation)
	if err := isValidSeed(seed); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	sprints := body.Sprints
	if sprints == 0 {
		sprints = 3 // Default per design doc
	}
	if sprints < 0 {
		writeError(w, http.StatusBadRequest, "invalid sprints count")
		return
	}

	// Run simulation A (DORA-strict)
	resultA := runComparison(model.PolicyDORAStrict, seed, sprints)

	// Run simulation B (TameFlow-cognitive)
	resultB := runComparison(model.PolicyTameFlowCognitive, seed, sprints)

	// Compare
	comparison := metrics.Compare(resultA, resultB, seed)

	// Build response
	response := buildCompareResponse(seed, sprints, comparison)
	writeJSON(w, http.StatusOK, response)
}

// addStandardTeam adds the fixed 3-developer team for comparison runs.
// Calculation: pure transformation (engine in → engine out).
func addStandardTeam(eng engine.Engine) engine.Engine {
	eng, _ = eng.AddDeveloper("dev-1", "Alice", 1.0)
	eng, _ = eng.AddDeveloper("dev-2", "Bob", 0.8)
	eng, _ = eng.AddDeveloper("dev-3", "Carol", 1.2)
	return eng
}

// addStandardBacklog adds the fixed 5-ticket backlog for comparison runs.
// Calculation: pure transformation (engine in → engine out).
func addStandardBacklog(eng engine.Engine) engine.Engine {
	eng, _ = eng.AddTicket(model.NewTicket("TKT-001", "Small clear", 2, model.HighUnderstanding))
	eng, _ = eng.AddTicket(model.NewTicket("TKT-002", "Medium clear", 4, model.HighUnderstanding))
	eng, _ = eng.AddTicket(model.NewTicket("TKT-003", "Small unclear", 2, model.LowUnderstanding))
	eng, _ = eng.AddTicket(model.NewTicket("TKT-004", "Large unclear", 8, model.LowUnderstanding))
	eng, _ = eng.AddTicket(model.NewTicket("TKT-005", "Medium mixed", 5, model.MediumUnderstanding))
	return eng
}

// runSprintsWithTracking runs N sprints and returns final metrics result.
// Calculation: pure transformation (engine in → metrics out).
func runSprintsWithTracking(eng engine.Engine, policy model.SizingPolicy, sprints int) metrics.SimulationResult {
	tracker := metrics.NewTracker()
	for i := 0; i < sprints; i++ {
		eng, _ = eng.StartSprint()
		eng = autoAssignForComparison(eng)
		eng, _, _ = eng.RunSprint()
		state := eng.Sim()
		tracker = tracker.Updated(&state)
	}
	state := eng.Sim()
	return tracker.GetResult(policy, &state)
}

// runComparison runs a single simulation with the given policy.
// Calculation: pure transformation (policy, seed, sprints → metrics).
func runComparison(policy model.SizingPolicy, seed int64, sprints int) metrics.SimulationResult {
	sim := model.NewSimulation(policy, seed)
	sim.ID = fmt.Sprintf("cmp-%d", seed)

	eng := engine.NewEngine(sim.Seed)
	eng, _ = eng.EmitCreated(sim.ID, sim.CurrentTick, events.SimConfig{
		TeamSize:     3,
		SprintLength: sim.SprintLength,
		Seed:         sim.Seed,
		Policy:       policy,
	})

	eng = addStandardTeam(eng)
	eng = addStandardBacklog(eng)
	return runSprintsWithTracking(eng, policy, sprints)
}

// autoAssignForComparison assigns backlog tickets to idle developers.
// Calculation: pure transformation (engine in → engine out).
// No event store, so errors are impossible (in-memory only).
func autoAssignForComparison(eng engine.Engine) engine.Engine {
	// Use index-based iteration so re-reads affect subsequent checks
	state := eng.Sim()
	idleDevs := state.IdleDevelopers()
	for i := 0; i < len(idleDevs) && len(state.Backlog) > 0; i++ {
		dev := idleDevs[i]
		eng, _ = eng.AssignTicket(state.Backlog[0].ID, dev.ID)
		state = eng.Sim() // Re-read after assignment - updates Backlog for next iteration
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

// HandleUpdateSimulation updates simulation settings (currently just policy).
// Returns 400 if invalid policy, 404 if simulation not found.
func (r SimRegistry) HandleUpdateSimulation(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	var body UpdateSimulationRequest
	if err := decodeJSON(req, &body); err != nil {
		respondDecodeError(w, err)
		return
	}

	// Parse and validate policy
	var policy model.SizingPolicy
	switch body.Policy {
	case "none":
		policy = model.PolicyNone
	case "dora-strict":
		policy = model.PolicyDORAStrict
	case "tameflow-cognitive":
		policy = model.PolicyTameFlowCognitive
	default:
		writeError(w, http.StatusBadRequest, "invalid policy")
		return
	}

	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		inst, ok := r.GetInstance(id)
		if !ok {
			writeError(w, http.StatusNotFound, "simulation not found")
			return
		}

		var err error
		if inst.Engine, err = inst.Engine.SetPolicy(policy); err != nil {
			continue // Retry with fresh state
		}
		r.SetInstance(id, inst)

		respondWithSimulation(w, inst, http.StatusOK)
		return
	}
	writeError(w, http.StatusConflict, "conflict")
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

// HandleDecompose attempts to decompose a ticket into smaller tasks.
// Returns decomposed=false if ticket not found or policy doesn't allow decomposition.
func (r SimRegistry) HandleDecompose(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	var body DecomposeRequest
	if err := decodeJSON(req, &body); err != nil {
		respondDecodeError(w, err)
		return
	}

	// toTicketStates converts model.Ticket slice to TicketState slice.
	toTicketStates := func(tickets []model.Ticket) []TicketState {
		states := make([]TicketState, len(tickets))
		for i, t := range tickets {
			states[i] = ToTicketState(t)
		}
		return states
	}

	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		inst, ok := r.GetInstance(id)
		if !ok {
			writeError(w, http.StatusNotFound, "simulation not found")
			return
		}

		var result either.Either[engine.NotDecomposable, []model.Ticket]
		var err error
		if inst.Engine, result, err = inst.Engine.TryDecompose(body.TicketID); err != nil {
			continue // Retry with fresh state
		}
		r.SetInstance(id, inst)

		children, decomposed := result.Get()
		state := ToState(inst.Engine.Sim(), inst.Tracker)
		response := DecomposeResponse{
			Decomposed: decomposed,
			Children:   toTicketStates(children),
			Simulation: state,
			Links:      LinksFor(state),
		}
		writeJSON(w, http.StatusOK, response)
		return
	}
	writeError(w, http.StatusConflict, "conflict")
}

// HandleGetLessons returns contextual lessons for a simulation.
// Reuses shared lesson selection logic - API is stateless so always starts fresh.
func (r SimRegistry) HandleGetLessons(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	inst, ok := r.GetInstance(id)
	if !ok {
		writeError(w, http.StatusNotFound, "simulation not found")
		return
	}

	// Determine view context from simulation state
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

	// API is stateless - always show orientation for fresh consumers
	lesson := lessons.Select(view, lessons.State{}, hasActiveSprint, false)

	writeJSON(w, http.StatusOK, LessonsResponse{
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
	})
}
