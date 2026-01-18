package api

import (
	"net/http"
)

// NewRouter creates HTTP router with all API endpoints.
// Uses Go 1.22+ ServeMux with path parameters.
func NewRouter(registry *SimRegistry) *http.ServeMux {
	mux := http.NewServeMux()

	// HATEOAS entry point
	mux.HandleFunc("GET /", registry.HandleEntryPoint)

	// Simulation endpoints
	mux.HandleFunc("GET /simulations", registry.HandleListSimulations)
	mux.HandleFunc("POST /simulations", registry.HandleCreateSimulation)
	mux.HandleFunc("GET /simulations/{id}", registry.HandleGetSimulation)
	mux.HandleFunc("POST /simulations/{id}/sprints", registry.HandleStartSprint)
	mux.HandleFunc("POST /simulations/{id}/tick", registry.HandleTick)
	mux.HandleFunc("POST /simulations/{id}/assignments", registry.HandleAssignTicket)

	// Comparison endpoint
	mux.HandleFunc("POST /comparisons", registry.HandleCompare)

	return mux
}
