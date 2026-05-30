package api

import (
	"net/http"
	"time"

	"github.com/binaryphile/fluentfp/web"
)

// Route pairs a Go 1.22+ method+path pattern with a fluentfp/web.Handler closure.
// Built by routes(SimRegistry) for iteration in NewRouter — single registration
// loop replaces the per-handler mux.HandleFunc enumeration that existed before
// the #18915 fluentfp/web migration.
type Route struct {
	Pattern string
	Handler web.Handler
}

// routes returns the canonical routing table for the simulation API.
// 13 endpoints covering UC10 (shared simulation), UC11 (plan sprint),
// UC37/38/39 (configuration), UC40 (investments), UC23 (manager takeaways),
// UC27-36 (TUI feature parity surfaces). /health is registered separately
// in NewRouter — it stays stdlib (no domain-error mapping).
func routes(r SimRegistry) []Route {
	return []Route{
		{Pattern: "GET /", Handler: handleEntryPoint(r)},
		{Pattern: "GET /simulations", Handler: handleListSimulations(r)},
		{Pattern: "POST /simulations", Handler: handleCreateSimulation(r)},
		{Pattern: "GET /simulations/{id}", Handler: handleGetSimulation(r)},
		{Pattern: "PATCH /simulations/{id}", Handler: handleUpdateSimulation(r)},
		{Pattern: "POST /simulations/{id}/sprints", Handler: handleStartSprint(r)},
		{Pattern: "POST /simulations/{id}/tick", Handler: handleTick(r)},
		{Pattern: "POST /simulations/{id}/assignments", Handler: handleAssignTicket(r)},
		{Pattern: "POST /simulations/{id}/decompose", Handler: handleDecompose(r)},
		{Pattern: "POST /simulations/{id}/investments", Handler: handleSpendInvestment(r)}, // UC40
		{Pattern: "GET /simulations/{id}/lessons", Handler: handleGetLessons(r)},
		{Pattern: "GET /simulations/{id}/office", Handler: handleGetOffice(r)},
		{Pattern: "POST /comparisons", Handler: handleCompare(r)},
	}
}

// NewRouter creates HTTP router with all API endpoints.
// Uses Go 1.22+ ServeMux with path parameters.
// Wraps mutating endpoints with deduplication middleware (5 min TTL).
//
// Post-#18915: each route is bound via web.Adapt(handler, WithErrorMapper(domainErrorMapper))
// — the consolidated domain-sentinel-to-HTTP-status mapping lives in
// adapt.go (single source of truth).
func NewRouter(registry SimRegistry) http.Handler {
	mux := http.NewServeMux()

	// Health check for service discovery (stays stdlib; no domain mapping needed).
	mux.HandleFunc("GET /health", handleHealth)

	// Register the 13 simulation/comparison endpoints via the routes table.
	// Single mapper covers ALL domain sentinels per /grade R1 F1 (#18915 plan).
	for _, rt := range routes(registry) { // justified:CF
		mux.HandleFunc(rt.Pattern, web.Adapt(rt.Handler, web.WithErrorMapper(domainErrorMapper)))
	}

	// Middleware chain - request flows top to bottom:
	//
	//   Request → LimitBody → RequireJSON → Dedup → mux → Response
	//
	// Wrapping is inside-out: last wrap is first to execute.
	dedup := NewDedupMiddleware(5 * time.Minute)
	handler := dedup.Wrap(mux)            // 3. Dedup (innermost)
	handler = RequireJSON(handler)        // 2. RequireJSON
	handler = LimitBody(1 << 20)(handler) // 1. LimitBody (outermost, runs first)
	return handler
}
