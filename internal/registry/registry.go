package registry

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"

	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
	"github.com/binaryphile/sofdevsim-2026/internal/office"
)

// ErrAlreadyExists is returned when attempting to create a duplicate simulation.
var ErrAlreadyExists = errors.New("already exists")

// SimRegistry manages simulation instances.
// Pointer receiver required: contains sync.RWMutex (must not be copied).
type SimRegistry struct {
	mu        sync.RWMutex
	instances map[string]SimInstance
	store     events.Store // shared event store for all simulations
}

// NewSimRegistry creates an empty registry with an in-memory event store.
func NewSimRegistry() *SimRegistry {
	return &SimRegistry{
		instances: make(map[string]SimInstance),
		store:     events.NewMemoryStore(),
	}
}

// Store returns the shared event store for subscriptions.
// No lock needed: store field is immutable after construction.
func (r *SimRegistry) Store() events.Store {
	return r.store
}

// IsZero returns true if the registry is uninitialized (zero value).
// Supports zero-value-as-absent pattern (see CLAUDE.md "Pseudo-Option Conventions").
func (r *SimRegistry) IsZero() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.instances == nil
}

// SimInstance owns a simulation.
// Engine uses value semantics (immutable pattern per FP Guide §7).
// After any Engine operation, store new Engine via SetInstance.
// State comes from engine.Sim() (projection); Sim field is for backward compat only.
type SimInstance struct {
	Sim     model.Simulation       // Value type for consistency; state from engine.Sim()
	Engine  engine.Engine          // Value type: immutable operations return new Engine
	Tracker metrics.Tracker        // DORA metrics tracker
	Office  office.OfficeProjection // Animation state for Claude vision
}

// CreateSimulation creates a new simulation with given seed and policy.
// Returns the simulation ID and nil error on success.
// Returns ErrAlreadyExists if a simulation with the same seed already exists.
func (r *SimRegistry) CreateSimulation(seed int64, policy model.SizingPolicy) (string, error) {
	id := fmt.Sprintf("sim-%d", seed)

	// Check existence under read lock first
	r.mu.RLock()
	_, exists := r.instances[id]
	r.mu.RUnlock()
	if exists {
		return "", fmt.Errorf("simulation %s: %w", id, ErrAlreadyExists)
	}

	sim := model.NewSimulation(id, policy, seed)

	var err error
	eng := engine.NewEngineWithStore(sim.Seed, r.store)
	if eng, err = eng.EmitCreated(sim.ID, sim.CurrentTick, events.SimConfig{
		TeamSize:     len(sim.Developers),
		SprintLength: sim.SprintLength,
		Seed:         sim.Seed,
		Policy:       policy,
	}); err != nil {
		return "", fmt.Errorf("emit created: %w", err)
	}

	// Add default team via engine (emits DeveloperAdded events)
	if eng, err = eng.AddDeveloper("dev-1", "Alice", 1.0); err != nil {
		return "", fmt.Errorf("add developer: %w", err)
	}
	if eng, err = eng.AddDeveloper("dev-2", "Bob", 0.8); err != nil {
		return "", fmt.Errorf("add developer: %w", err)
	}
	if eng, err = eng.AddDeveloper("dev-3", "Carol", 1.2); err != nil {
		return "", fmt.Errorf("add developer: %w", err)
	}

	// Generate backlog via engine (emits TicketCreated events)
	gen := engine.Scenarios["healthy"]
	rng := rand.New(rand.NewSource(seed))
	tickets := gen.Generate(rng, 12)
	for _, t := range tickets {
		if eng, err = eng.AddTicket(t); err != nil {
			return "", fmt.Errorf("add ticket: %w", err)
		}
	}

	// Initialize office projection with developer IDs
	devIDs := slice.From(eng.Sim().Developers).ToString(model.Developer.GetID)
	officeProj := office.NewOfficeProjection(devIDs)

	// All developers start in conference at tick 0
	recordConferenceEntry := func(proj office.OfficeProjection, devID string) office.OfficeProjection {
		return proj.Record(office.DevEnteredConference{DevID: devID}, 0)
	}
	officeProj = slice.Fold(devIDs, officeProj, recordConferenceEntry)

	// Write under lock
	r.mu.Lock()
	// Double-check after acquiring write lock (another goroutine may have created it)
	if _, exists := r.instances[id]; exists {
		r.mu.Unlock()
		return "", fmt.Errorf("simulation %s: %w", id, ErrAlreadyExists)
	}
	r.instances[id] = SimInstance{
		Sim:     sim,
		Engine:  eng,
		Tracker: metrics.NewTracker(),
		Office:  officeProj,
	}
	r.mu.Unlock()

	return id, nil
}

// GetInstanceOption returns simulation instance as an option.
// State should be read via engine.Sim() (projection), not inst.Sim.
func (r *SimRegistry) GetInstanceOption(id string) option.Basic[SimInstance] {
	r.mu.RLock()
	defer r.mu.RUnlock()
	inst, ok := r.instances[id]
	return option.New(inst, ok)
}

// SetInstance stores a simulation instance in the registry.
// Used internally to update tracker state after operations.
func (r *SimRegistry) SetInstance(id string, inst SimInstance) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.instances[id] = inst
}

// ListSimulations returns all active simulation IDs and their states.
func (r *SimRegistry) ListSimulations() []SimulationSummary {
	r.mu.RLock()
	defer r.mu.RUnlock()
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
