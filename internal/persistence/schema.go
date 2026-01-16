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

	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// CurrentVersion is the schema version for save files.
// Increment when making breaking changes to the schema.
// See migrate.go for adding migration functions.
const CurrentVersion = 1

// SaveFile is the top-level container for persisted state.
type SaveFile struct {
	Version   int              // Schema version for migrations
	Timestamp time.Time        // When saved
	Name      string           // User-provided or auto-generated
	State     SimulationState  // Full simulation state
}

// SimulationState captures everything needed to restore a simulation.
type SimulationState struct {
	Simulation model.Simulation   // Core simulation state
	DORA       metrics.DORAMetrics // DORA calculator with history
	Fever      metrics.FeverChart  // Fever calculator with history
}

// SaveInfo provides metadata about a save file for listing.
type SaveInfo struct {
	Path      string    // Full path to save file
	Name      string    // Save name
	Timestamp time.Time // When saved
	Version   int       // Schema version
}
