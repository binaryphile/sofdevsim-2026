package api

import (
	"fmt"
	"math/rand"

	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// SimRegistry manages simulation instances.
// Value receiver: map/interface fields have reference semantics.
type SimRegistry struct {
	instances map[string]SimInstance
	store     events.Store // shared event store for all simulations
}

// NewSimRegistry creates an empty registry with an in-memory event store.
func NewSimRegistry() SimRegistry {
	return SimRegistry{
		instances: make(map[string]SimInstance),
		store:     events.NewMemoryStore(),
	}
}

// Store returns the shared event store for subscriptions.
func (r SimRegistry) Store() events.Store {
	return r.store
}

// IsZero returns true if the registry is uninitialized (zero value).
// Implements option.ZeroChecker for use with option.IfNotZero.
func (r SimRegistry) IsZero() bool {
	return r.instances == nil
}

// SimInstance owns a simulation.
// Note: Engine is stateful (holds *Simulation, seeded RNG).
// Engine mutates simulation in place - this is existing design.
type SimInstance struct {
	sim     *model.Simulation
	engine  *engine.Engine
	tracker metrics.Tracker
}

// CreateSimulation creates a new simulation with given seed and policy.
func (r SimRegistry) CreateSimulation(seed int64, policy model.SizingPolicy) string {
	id := fmt.Sprintf("sim-%d", seed)

	sim := model.NewSimulation(policy, seed)
	sim.ID = id // Set ID for event sourcing

	// Add default team (matching TUI pattern)
	sim.AddDeveloper(model.NewDeveloper("dev-1", "Alice", 1.0))
	sim.AddDeveloper(model.NewDeveloper("dev-2", "Bob", 0.8))
	sim.AddDeveloper(model.NewDeveloper("dev-3", "Carol", 1.2))

	// Generate backlog (matching TUI pattern)
	gen := engine.Scenarios["healthy"]
	rng := rand.New(rand.NewSource(seed))
	tickets := gen.Generate(rng, 12)
	for _, t := range tickets {
		sim.AddTicket(t)
	}

	eng := engine.NewEngineWithStore(sim, r.store)
	eng.EmitCreated() // Emit after setup complete

	r.instances[id] = SimInstance{
		sim:     sim,
		engine:  eng,
		tracker: metrics.NewTracker(),
	}

	return id
}

// RegisterSimulation registers an existing simulation with the shared event store.
// Returns the engine configured to emit to the shared store.
// Use this to share simulations between TUI and API.
func (r SimRegistry) RegisterSimulation(sim *model.Simulation, tracker metrics.Tracker) *engine.Engine {
	eng := engine.NewEngineWithStore(sim, r.store)
	eng.EmitCreated()

	r.instances[sim.ID] = SimInstance{
		sim:     sim,
		engine:  eng,
		tracker: tracker,
	}

	return eng
}

// getInstance returns simulation instance using comma-ok pattern.
// SimInstance contains pointers, so mutations via engine affect original.
func (r SimRegistry) getInstance(id string) (SimInstance, bool) {
	inst, ok := r.instances[id]
	return inst, ok
}

// ListSimulations returns all active simulation IDs and their states.
func (r SimRegistry) ListSimulations() []SimulationSummary {
	result := make([]SimulationSummary, 0, len(r.instances))
	for id, inst := range r.instances {
		result = append(result, SimulationSummary{
			ID:           id,
			SprintActive: inst.sim.CurrentSprintOption.IsOk(),
		})
	}
	return result
}

// SimulationSummary is a lightweight view of a simulation for listing.
type SimulationSummary struct {
	ID           string
	SprintActive bool
}
