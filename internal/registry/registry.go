package registry

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
	Sim     *model.Simulation
	Engine  *engine.Engine
	Tracker metrics.Tracker
}

// CreateSimulation creates a new simulation with given seed and policy.
func (r SimRegistry) CreateSimulation(seed int64, policy model.SizingPolicy) string {
	id := fmt.Sprintf("sim-%d", seed)

	sim := model.NewSimulation(policy, seed)
	sim.ID = id // Set ID for event sourcing

	eng := engine.NewEngineWithStore(sim.Seed, r.store)
	eng.EmitCreated(sim.ID, sim.CurrentTick, events.SimConfig{
		TeamSize:     len(sim.Developers),
		SprintLength: sim.SprintLength,
		Seed:         sim.Seed,
		Policy:       policy,
	}) // Emit SimulationCreated first

	// Add default team via engine (emits DeveloperAdded events)
	eng.AddDeveloper("dev-1", "Alice", 1.0)
	eng.AddDeveloper("dev-2", "Bob", 0.8)
	eng.AddDeveloper("dev-3", "Carol", 1.2)

	// Generate backlog via engine (emits TicketCreated events)
	gen := engine.Scenarios["healthy"]
	rng := rand.New(rand.NewSource(seed))
	tickets := gen.Generate(rng, 12)
	for _, t := range tickets {
		eng.AddTicket(t)
	}

	r.instances[id] = SimInstance{
		Sim:     sim,
		Engine:  eng,
		Tracker: metrics.NewTracker(),
	}

	return id
}

// RegisterSimulation registers an existing simulation with the shared event store.
// Returns the engine configured to emit to the shared store.
// Use this to share simulations between TUI and API.
func (r SimRegistry) RegisterSimulation(sim *model.Simulation, tracker metrics.Tracker) *engine.Engine {
	eng := engine.NewEngineWithStore(sim.Seed, r.store)
	eng.EmitLoadedState(*sim) // Syncs all sim state (developers, tickets) to projection

	r.instances[sim.ID] = SimInstance{
		Sim:     sim,
		Engine:  eng,
		Tracker: tracker,
	}

	return eng
}

// GetInstance returns simulation instance using comma-ok pattern.
// SimInstance contains pointers, so mutations via engine affect original.
func (r SimRegistry) GetInstance(id string) (SimInstance, bool) {
	inst, ok := r.instances[id]
	return inst, ok
}

// SetInstance stores a simulation instance in the registry.
// Used internally to update tracker state after operations.
func (r SimRegistry) SetInstance(id string, inst SimInstance) {
	r.instances[id] = inst
}

// ListSimulations returns all active simulation IDs and their states.
func (r SimRegistry) ListSimulations() []SimulationSummary {
	result := make([]SimulationSummary, 0, len(r.instances))
	for id, inst := range r.instances {
		result = append(result, SimulationSummary{
			ID:           id,
			SprintActive: inst.Engine.Sim().CurrentSprintOption.IsOk(),
		})
	}
	return result
}

// SimulationSummary is a lightweight view of a simulation for listing.
type SimulationSummary struct {
	ID           string
	SprintActive bool
}
