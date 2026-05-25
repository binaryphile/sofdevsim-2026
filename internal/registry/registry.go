package registry

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

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

// ErrUnknownScenario is returned when CreateSimulation receives a scenarioName
// not registered in engine.Scenarios. Per /c absorption (Go dev guide §8 errors-
// as-values): callers MUST use errors.Is(err, ErrUnknownScenario) to differentiate
// the bad-input case from internal errors — never string-matching the message.
var ErrUnknownScenario = errors.New("unknown scenario")

// SimRegistry manages simulation instances.
// Pointer receiver required: contains sync.RWMutex (must not be copied).
type SimRegistry struct {
	mu          sync.RWMutex
	instances   map[string]SimInstance
	store events.Store // shared event store for all simulations
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

// CreateSimulation creates a new simulation with given seed, policy, backlog
// mix scenario, per-phase WIP cap configuration, and (UC39) release mode.
// Returns the simulation ID and nil error on success.
// Returns ErrAlreadyExists if a simulation with the same seed already exists.
// Returns ErrUnknownScenario if scenarioName is not a registered scenario.
// Returns one of model.ErrCap{Zero,Negative,BelowMentorMin,Conflict}
// (wrapped) if phaseWIPConfig fails validation.
// UC37: scenarioName selects the backlog generation profile (default "healthy"
// preserves pre-UC37 behaviour — all-Feature regression-safe default).
// UC38: phaseWIPConfig nil/empty = unlimited everywhere (regression-safe per
// Decision D); validation is single-pass here so CLI + REST share friendly
// errors (the CLI parser only does syntax-only k=v checking).
// UC39: releaseMode is a strongly-typed enum at this layer; ParseReleaseMode
// is invoked upstream (HTTP handler / CLI parser) — the registry just accepts
// the typed value. ReleaseModePush zero-value = regression-safe default.
func (r *SimRegistry) CreateSimulation(
	seed int64,
	policy model.SizingPolicy,
	scenarioName string,
	phaseWIPConfig map[model.WorkflowPhase]int,
	releaseMode model.ReleaseMode,
) (string, error) {
	if scenarioName == "" {
		scenarioName = "healthy"
	}
	gen, ok := engine.Scenarios[scenarioName]
	if !ok {
		names := make([]string, 0, len(engine.Scenarios))
		for n := range engine.Scenarios { // justified:SM (key extraction for diagnostic; sorted by caller)
			names = append(names, n)
		}
		sort.Strings(names)
		return "", fmt.Errorf("%w %q (registered: %v)", ErrUnknownScenario, scenarioName, names)
	}

	// UC38 single-pass validation: rope config defaults to disabled at
	// registry creation so ErrCapConflict only fires when caller's
	// PhaseWIPConfig conflicts with an explicit rope config (deferred to
	// future cycle if needed). NewSimulation's zero-value RopeConfig is
	// Enabled=false, so conflict-detection is dormant by design here.
	if err := model.ValidatePhaseWIPConfig(phaseWIPConfig, model.RopeConfig{}); err != nil {
		return "", err
	}

	id := fmt.Sprintf("sim-%d", seed)

	// Check existence under read lock first
	r.mu.RLock()
	_, exists := r.instances[id]
	r.mu.RUnlock()
	if exists {
		return "", fmt.Errorf("simulation %s: %w", id, ErrAlreadyExists)
	}

	sim := model.NewSimulation(id, policy, seed)
	sim.PhaseWIPConfig = phaseWIPConfig
	sim.ReleaseMode = releaseMode

	var err error
	eng := engine.NewEngineWithStore(sim.Seed, r.store)
	if eng, err = eng.EmitCreated(sim.ID, sim.CurrentTick, events.SimConfig{
		TeamSize:       len(sim.Developers),
		SprintLength:   sim.SprintLength,
		Seed:           sim.Seed,
		Policy:         policy,
		PhaseWIPConfig: phaseWIPConfig,
		ReleaseMode:    releaseMode,
	}); err != nil {
		return "", fmt.Errorf("emit created: %w", err)
	}

	// Add default team via engine (emits DeveloperAdded events)
	// Matches TUI team: 6 developers for 2×3 cubicle layout
	team := []struct{ id, name string; velocity float64 }{
		{"dev-1", "MsPac", 1.0}, {"dev-2", "Qbert", 0.8}, {"dev-3", "Samus", 1.2},
		{"dev-4", "Athena", 1.0}, {"dev-5", "Mappy", 0.9}, {"dev-6", "Pengo", 1.1},
	}
	for _, d := range team {
		if eng, err = eng.AddDeveloper(d.id, d.name, d.velocity); err != nil {
			return "", fmt.Errorf("add developer: %w", err)
		}
	}

	// Generate backlog via engine (emits TicketCreated events)
	rng := rand.New(rand.NewSource(seed))
	tickets := gen.Generate(rng, 12)
	for _, t := range tickets {
		if eng, err = eng.AddTicket(t); err != nil {
			return "", fmt.Errorf("add ticket: %w", err)
		}
	}

	// Initialize office projection with developer IDs (developers start at cubicles)
	devIDs := slice.From(eng.Sim().Developers).ToString(model.Developer.GetID)
	officeProj := office.NewOfficeProjection(devIDs)

	// Move all developers to conference for initial sprint planning
	now := time.Now()
	recordConferenceEntry := func(proj office.OfficeProjection, devID string) office.OfficeProjection {
		return proj.Record(office.DevEnteredConference{DevID: devID}, 0, now)
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
func (r *SimRegistry) GetInstanceOption(id string) option.Option[SimInstance] {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return option.Lookup(r.instances, id)
}

// SetInstance stores a simulation instance in the registry.
// Used internally to update tracker state after operations.
func (r *SimRegistry) SetInstance(id string, inst SimInstance) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.instances[id] = inst
}

// UpdateOffice updates the office projection for a simulation.
// Used by TUI to sync office state to registry for Claude vision.
func (r *SimRegistry) UpdateOffice(simID string, proj office.OfficeProjection) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if inst, ok := r.instances[simID]; ok {
		inst.Office = proj
		r.instances[simID] = inst
	}
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
