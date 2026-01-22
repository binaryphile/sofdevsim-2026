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
	mux.HandleFunc("PATCH /simulations/{id}", registry.HandleUpdateSimulation)
	mux.HandleFunc("POST /simulations/{id}/sprints", registry.HandleStartSprint)
	mux.HandleFunc("POST /simulations/{id}/tick", registry.HandleTick)
	mux.HandleFunc("POST /simulations/{id}/assignments", registry.HandleAssignTicket)
	mux.HandleFunc("POST /simulations/{id}/decompose", registry.HandleDecompose)
	mux.HandleFunc("GET /simulations/{id}/lessons", registry.HandleGetLessons)

	// Comparison endpoint
	mux.HandleFunc("POST /comparisons", registry.HandleCompare)

	// Middleware chain - request flows top to bottom:
	//
	//   Request → LimitBody → RequireJSON → Dedup → mux → Response
	//
	// Wrapping is inside-out: last wrap is first to execute.
	dedup := NewDedupMiddleware(5 * time.Minute)
	handler := dedup.Wrap(mux)           // 3. Dedup (innermost)
	handler = RequireJSON(handler)       // 2. RequireJSON
	handler = LimitBody(1 << 20)(handler) // 1. LimitBody (outermost, runs first)
	return handler
}
