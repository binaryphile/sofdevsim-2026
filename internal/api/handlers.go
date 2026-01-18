package api

import (
	"encoding/json"
	"net/http"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

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
func (r *SimRegistry) HandleEntryPoint(w http.ResponseWriter, req *http.Request) {
	response := EntryPointResponse{
		Links: map[string]string{
			"self":        "/",
			"simulations": "/simulations",
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
func (r *SimRegistry) HandleListSimulations(w http.ResponseWriter, req *http.Request) {
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
func (r *SimRegistry) HandleCreateSimulation(w http.ResponseWriter, req *http.Request) {
	var body CreateSimulationRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
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

	id := r.CreateSimulation(body.Seed, policy)

	inst, _ := r.getInstance(id)
	state := ToState(*inst.sim)
	response := HALResponse{
		State: state,
		Links: LinksFor(state),
	}
	writeJSON(w, http.StatusCreated, response)
}

// HandleGetSimulation returns the current state of a simulation.
// Includes context-appropriate links based on whether a sprint is active.
func (r *SimRegistry) HandleGetSimulation(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	inst, ok := r.getInstance(id)
	if !ok {
		writeError(w, http.StatusNotFound, "simulation not found")
		return
	}

	state := ToState(*inst.sim)
	response := HALResponse{
		State: state,
		Links: LinksFor(state),
	}
	writeJSON(w, http.StatusOK, response)
}

// HandleStartSprint starts a new sprint for the simulation.
// Returns 409 Conflict if a sprint is already active.
func (r *SimRegistry) HandleStartSprint(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	inst, ok := r.getInstance(id)
	if !ok {
		writeError(w, http.StatusNotFound, "simulation not found")
		return
	}

	// Check if sprint already active
	if _, active := inst.sim.CurrentSprintOption.Get(); active {
		writeError(w, http.StatusConflict, "sprint already active")
		return
	}

	inst.engine.StartSprint()

	state := ToState(*inst.sim)
	response := HALResponse{
		State: state,
		Links: LinksFor(state),
	}
	writeJSON(w, http.StatusOK, response)
}

// HandleTick advances the simulation by one tick.
// Returns updated state with context-appropriate links (tick disappears when sprint ends).
func (r *SimRegistry) HandleTick(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	inst, ok := r.getInstance(id)
	if !ok {
		writeError(w, http.StatusNotFound, "simulation not found")
		return
	}

	// Check if sprint is active
	if _, active := inst.sim.CurrentSprintOption.Get(); !active {
		writeError(w, http.StatusConflict, "no active sprint")
		return
	}

	// Engine mutates *Simulation in place
	inst.engine.Tick()

	// Clear sprint if it has ended (domain logic - could move to Simulation.EndSprintIfComplete)
	clearSprintIfEnded(inst.sim)

	inst.tracker.Update(inst.sim)

	state := ToState(*inst.sim)
	response := HALResponse{
		State: state,
		Links: LinksFor(state),
	}
	writeJSON(w, http.StatusOK, response)
}

// clearSprintIfEnded clears the current sprint if CurrentTick has reached EndDay.
func clearSprintIfEnded(sim *model.Simulation) {
	sprint, ok := sim.CurrentSprintOption.Get()
	if !ok {
		return
	}
	if sim.CurrentTick >= sprint.EndDay {
		sim.CurrentSprintOption = model.NoSprint
	}
}

// AssignTicketRequest is the request body for assigning a ticket.
type AssignTicketRequest struct {
	TicketID    string `json:"ticketId"`
	DeveloperID string `json:"developerId"`
}

// HandleAssignTicket assigns a ticket to a developer.
// If developerId is omitted, auto-assigns to first idle developer.
// Returns 400 if ticket/developer not found or developer is busy.
func (r *SimRegistry) HandleAssignTicket(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	inst, ok := r.getInstance(id)
	if !ok {
		writeError(w, http.StatusNotFound, "simulation not found")
		return
	}

	var body AssignTicketRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Auto-assign if no developer specified
	devID := body.DeveloperID
	if devID == "" {
		idle := inst.sim.IdleDevelopers()
		if len(idle) == 0 {
			writeError(w, http.StatusBadRequest, "no idle developers")
			return
		}
		devID = idle[0].ID
	}

	if err := inst.engine.AssignTicket(body.TicketID, devID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	state := ToState(*inst.sim)
	response := HALResponse{
		State: state,
		Links: LinksFor(state),
	}
	writeJSON(w, http.StatusOK, response)
}
