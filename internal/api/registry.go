package api

import (
	"fmt"
	"math/rand"

	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// SimRegistry is an aggregate root (manages collection).
// Uses pointer receiver because map is reference type.
type SimRegistry struct {
	instances map[string]SimInstance
}

// NewSimRegistry creates an empty registry.
func NewSimRegistry() *SimRegistry {
	return &SimRegistry{
		instances: make(map[string]SimInstance),
	}
}

// SimInstance owns a simulation.
// Note: Engine is stateful (holds *Simulation, seeded RNG).
// Engine mutates simulation in place - this is existing design.
type SimInstance struct {
	sim     *model.Simulation
	engine  *engine.Engine
	tracker *metrics.Tracker
}

// CreateSimulation creates a new simulation with given seed and policy.
func (r *SimRegistry) CreateSimulation(seed int64, policy model.SizingPolicy) string {
	sim := model.NewSimulation(policy, seed)

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

	id := fmt.Sprintf("sim-%d", seed)
	r.instances[id] = SimInstance{
		sim:     sim,
		engine:  engine.NewEngine(sim),
		tracker: metrics.NewTracker(),
	}

	return id
}

// getInstance returns simulation instance using comma-ok pattern.
// SimInstance contains pointers, so mutations via engine affect original.
func (r *SimRegistry) getInstance(id string) (SimInstance, bool) {
	inst, ok := r.instances[id]
	return inst, ok
}
