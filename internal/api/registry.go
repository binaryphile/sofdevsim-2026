package api

import (
	"github.com/binaryphile/sofdevsim-2026/internal/registry"
)

// SimRegistry wraps registry.SimRegistry to add HTTP handler methods.
// Embedding allows all registry methods to be called directly on SimRegistry.
type SimRegistry struct {
	registry.SimRegistry
}

// NewSimRegistry creates an empty registry with an in-memory event store.
func NewSimRegistry() SimRegistry {
	return SimRegistry{
		SimRegistry: registry.NewSimRegistry(),
	}
}

// Re-export types for convenience
type (
	SimInstance       = registry.SimInstance
	SimulationSummary = registry.SimulationSummary
)
