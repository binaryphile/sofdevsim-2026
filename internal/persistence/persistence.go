package persistence

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

func init() {
	// Register types for Gob encoding.
	// Enums (int-based) don't strictly need registration, but it's good practice.
	gob.Register(model.SizingPolicy(0))
	gob.Register(model.WorkflowPhase(0))
	gob.Register(model.UnderstandingLevel(0))
	gob.Register(model.FeverStatus(0))
	gob.Register(model.Severity(0))
	gob.Register(model.EventType(0))

	// Register struct types
	gob.Register(PersistableSimulation{}) // Gob-safe version of model.Simulation
	gob.Register(model.Developer{})
	gob.Register(model.Ticket{})
	gob.Register(model.Sprint{})
	gob.Register(model.Incident{})
	gob.Register(model.Event{})
	gob.Register(metrics.DORAMetrics{})
	gob.Register(metrics.DORASnapshot{})
	gob.Register(metrics.FeverChart{})
	gob.Register(metrics.FeverSnapshot{})
}

// Save persists the simulation state to a file.
//
// The save file includes the full simulation state, DORA metrics with history,
// and fever chart with history. The file is versioned for future migration support.
//
// Example:
//
//	path := persistence.GenerateSavePath("saves", "my-experiment")
//	err := persistence.Save(path, "my-experiment", sim, tracker)
func Save(path, name string, sim *model.Simulation, tracker metrics.Tracker) error {
	saveFile := SaveFile{
		Version:   CurrentVersion,
		Timestamp: time.Now(),
		Name:      name,
		State: SimulationState{
			Simulation: ToPersistable(sim),
			DORA:       tracker.DORA,
			Fever:      tracker.Fever,
		},
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Create file
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	// Encode with Gob
	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(saveFile); err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	return nil
}

// Load restores simulation state from a file.
//
// If the save file has an older schema version, migrations are applied automatically.
// Returns an error if the file version is newer than the current supported version.
//
// Example:
//
//	sim, tracker, err := persistence.Load("saves/my-experiment.sds")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// sim and tracker are fully restored
func Load(path string) (*model.Simulation, metrics.Tracker, error) {
	saveFile, err := LoadRaw(path)
	if err != nil {
		return nil, metrics.Tracker{}, err
	}

	// TODO: Handle version migrations here when needed
	if saveFile.Version > CurrentVersion {
		return nil, metrics.Tracker{}, fmt.Errorf("save file version %d is newer than supported version %d", saveFile.Version, CurrentVersion)
	}

	// Reconstruct tracker from saved metrics
	tracker := metrics.Tracker{
		DORA:  saveFile.State.DORA,
		Fever: saveFile.State.Fever,
	}

	sim := FromPersistable(saveFile.State.Simulation)

	return sim, tracker, nil
}

// LoadRaw loads a save file without processing, for inspection.
func LoadRaw(path string) (*SaveFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	var saveFile SaveFile
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&saveFile); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	return &saveFile, nil
}

// ListSaves returns information about all save files in a directory.
func ListSaves(dir string) ([]SaveInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	var saves []SaveInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".sds") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		saveFile, err := LoadRaw(path)
		if err != nil {
			// Skip corrupted files
			continue
		}

		saves = append(saves, SaveInfo{
			Path:      path,
			Name:      saveFile.Name,
			Timestamp: saveFile.Timestamp,
			Version:   saveFile.Version,
		})
	}

	return saves, nil
}

// DefaultSavesDir returns the default directory for save files.
func DefaultSavesDir() string {
	return "saves"
}

// GenerateSavePath creates a path for a new save file.
func GenerateSavePath(dir, name string) string {
	// Sanitize name: replace spaces with dashes, remove special chars
	safeName := strings.ReplaceAll(name, " ", "-")
	safeName = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, safeName)

	if safeName == "" {
		safeName = "auto-" + time.Now().Format("2006-01-02-150405")
	}

	return filepath.Join(dir, safeName+".sds")
}
