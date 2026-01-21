package api

import (
	"net/http"
	"time"
)

// NewRouter creates HTTP router with all API endpoints.
// Uses Go 1.22+ ServeMux with path parameters.
// Wraps mutating endpoints with deduplication middleware (5 min TTL).
func NewRouter(registry SimRegistry) http.Handler {
	mux := http.NewServeMux()

	// Health check for service discovery
	mux.HandleFunc("GET /health", handleHealth)

	// HATEOAS entry point
	mux.HandleFunc("GET /", registry.HandleEntryPoint)

	// Simulation endpoints
	mux.HandleFunc("GET /simulations", registry.HandleListSimulations)
	mux.HandleFunc("POST /simulations", registry.HandleCreateSimulation)
	mux.HandleFunc("GET /simulations/{id}", registry.HandleGetSimulation)
	mux.HandleFunc("POST /simulations/{id}/sprints", registry.HandleStartSprint)
	mux.HandleFunc("POST /simulations/{id}/tick", registry.HandleTick)
	mux.HandleFunc("POST /simulations/{id}/assignments", registry.HandleAssignTicket)
	mux.HandleFunc("GET /simulations/{id}/lessons", registry.HandleGetLessons)

	// Comparison endpoint
	mux.HandleFunc("POST /comparisons", registry.HandleCompare)

	// Wrap with deduplication middleware (5 min TTL for request ID caching)
	dedup := NewDedupMiddleware(5 * time.Minute)
	return dedup.Wrap(mux)
}
