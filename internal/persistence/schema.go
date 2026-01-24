// Package persistence provides save/load functionality for simulation state.
//
// Save files use Go's encoding/gob format for efficient binary serialization.
// Each save file includes a schema version to support forward-compatible migrations.
//
// Basic usage:
//
//	// Save current state
//	err := persistence.Save("saves/my-experiment.sds", "my-experiment", sim, tracker)
//
//	// Load saved state
//	sim, tracker, err := persistence.Load("saves/my-experiment.sds")
//
//	// List available saves
//	saves, err := persistence.ListSaves("saves/")
package persistence

import (
	"time"

	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// CurrentVersion is the schema version for save files.
// Increment when making breaking changes to the schema.
// See migrate.go for adding migration functions.
const CurrentVersion = 2

// SaveFile is the top-level container for persisted state.
type SaveFile struct {
	Version   int             // Schema version for migrations
	Timestamp time.Time       // When saved
	Name      string          // User-provided or auto-generated
	State     SimulationState // Full simulation state
}

// SimulationState captures everything needed to restore a simulation.
// Note: Uses PersistableSimulation for gob compatibility (option.Basic can't be serialized).
type SimulationState struct {
	Simulation PersistableSimulation // Core simulation state (gob-safe)
	DORA       metrics.DORAMetrics   // DORA calculator with history
	Fever      metrics.FeverChart    // Fever calculator with history
}

// PersistableSimulation mirrors model.Simulation but uses *Sprint instead of option.Basic[Sprint]
// for gob serialization compatibility.
type PersistableSimulation struct {
	CurrentTick   int
	CurrentSprint *model.Sprint // Uses pointer for gob (option.Basic has unexported fields)
	SprintNumber  int

	Developers []model.Developer

	Backlog          []model.Ticket
	ActiveTickets    []model.Ticket
	CompletedTickets []model.Ticket

	OpenIncidents     []model.Incident
	ResolvedIncidents []model.Incident

	SizingPolicy model.SizingPolicy
	SprintLength int
	BufferPct    float64

	Seed int64
}

// ToPersistable converts a model.Simulation to a gob-safe PersistableSimulation.
func ToPersistable(sim model.Simulation) PersistableSimulation {
	var sprintPtr *model.Sprint
	if sprint, ok := sim.CurrentSprintOption.Get(); ok {
		sprintPtr = &sprint
	}

	return PersistableSimulation{
		CurrentTick:       sim.CurrentTick,
		CurrentSprint:     sprintPtr,
		SprintNumber:      sim.SprintNumber,
		Developers:        sim.Developers,
		Backlog:           sim.Backlog,
		ActiveTickets:     sim.ActiveTickets,
		CompletedTickets:  sim.CompletedTickets,
		OpenIncidents:     sim.OpenIncidents,
		ResolvedIncidents: sim.ResolvedIncidents,
		SizingPolicy:      sim.SizingPolicy,
		SprintLength:      sim.SprintLength,
		BufferPct:         sim.BufferPct,
		Seed:              sim.Seed,
	}
}

// FromPersistable converts a PersistableSimulation back to a model.Simulation.
func FromPersistable(ps PersistableSimulation) model.Simulation {
	var sprintOption option.Basic[model.Sprint]
	if ps.CurrentSprint != nil {
		sprintOption = option.Of(*ps.CurrentSprint)
	}

	return model.Simulation{
		CurrentTick:          ps.CurrentTick,
		CurrentSprintOption:  sprintOption,
		SprintNumber:         ps.SprintNumber,
		Developers:           ps.Developers,
		Backlog:              ps.Backlog,
		ActiveTickets:        ps.ActiveTickets,
		CompletedTickets:     ps.CompletedTickets,
		OpenIncidents:        ps.OpenIncidents,
		ResolvedIncidents:    ps.ResolvedIncidents,
		SizingPolicy:         ps.SizingPolicy,
		SprintLength:         ps.SprintLength,
		BufferPct:            ps.BufferPct,
		Seed:                 ps.Seed,
	}
}

// SaveInfo provides metadata about a save file for listing.
type SaveInfo struct {
	Path      string    // Full path to save file
	Name      string    // Save name
	Timestamp time.Time // When saved
	Version   int       // Schema version
}
