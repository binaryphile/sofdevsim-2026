package persistence_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
	"github.com/binaryphile/sofdevsim-2026/internal/persistence"
)

// =============================================================================
// Domain Tests (Unit) - Test serialization logic
// =============================================================================

// TestSaveLoad_RoundTrip verifies that a saved simulation can be loaded back
// with all state intact.
func TestSaveLoad_RoundTrip(t *testing.T) {
	// Arrange: create a simulation with some state
	sim := model.NewSimulation(model.PolicyDORAStrict, 12345)
	sim.Developers = append(sim.Developers, model.NewDeveloper("dev-001", "Alice", 1.0))
	sim.Backlog = append(sim.Backlog, model.NewTicket("TKT-001", "Test ticket", 5.0, model.HighUnderstanding))
	sim.CurrentTick = 42

	tracker := metrics.NewTracker()
	tracker.DORA.TotalDeploys = 5
	tracker.Fever.BufferTotal = 10.0
	tracker.Fever.BufferConsumed = 3.0

	// Create temp directory for test
	tmpDir := t.TempDir()
	savePath := filepath.Join(tmpDir, "test-save.sds")

	// Act: save and load
	err := persistence.Save(savePath, "test-save", sim, tracker)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loadedSim, loadedTracker, err := persistence.Load(savePath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Assert: verify round-trip integrity
	if loadedSim.CurrentTick != 42 {
		t.Errorf("CurrentTick = %d, want 42", loadedSim.CurrentTick)
	}
	if loadedSim.Seed != 12345 {
		t.Errorf("Seed = %d, want 12345", loadedSim.Seed)
	}
	if len(loadedSim.Developers) != 1 {
		t.Errorf("Developers count = %d, want 1", len(loadedSim.Developers))
	}
	if len(loadedSim.Backlog) != 1 {
		t.Errorf("Backlog count = %d, want 1", len(loadedSim.Backlog))
	}
	if loadedTracker.DORA.TotalDeploys != 5 {
		t.Errorf("TotalDeploys = %d, want 5", loadedTracker.DORA.TotalDeploys)
	}
	if loadedTracker.Fever.BufferConsumed != 3.0 {
		t.Errorf("BufferConsumed = %f, want 3.0", loadedTracker.Fever.BufferConsumed)
	}
}

// TestSaveFile_IncludesVersion verifies that saved files contain a schema version.
func TestSaveFile_IncludesVersion(t *testing.T) {
	sim := model.NewSimulation(model.PolicyNone, 1)
	tracker := metrics.NewTracker()

	tmpDir := t.TempDir()
	savePath := filepath.Join(tmpDir, "version-test.sds")

	err := persistence.Save(savePath, "version-test", sim, tracker)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load and check version
	saveFile, err := persistence.LoadRaw(savePath)
	if err != nil {
		t.Fatalf("LoadRaw failed: %v", err)
	}

	if saveFile.Version != persistence.CurrentVersion {
		t.Errorf("Version = %d, want %d", saveFile.Version, persistence.CurrentVersion)
	}
}

// TestSaveFile_IncludesTimestamp verifies that saved files contain a timestamp.
func TestSaveFile_IncludesTimestamp(t *testing.T) {
	sim := model.NewSimulation(model.PolicyNone, 1)
	tracker := metrics.NewTracker()

	before := time.Now()

	tmpDir := t.TempDir()
	savePath := filepath.Join(tmpDir, "timestamp-test.sds")

	err := persistence.Save(savePath, "timestamp-test", sim, tracker)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	after := time.Now()

	saveFile, err := persistence.LoadRaw(savePath)
	if err != nil {
		t.Fatalf("LoadRaw failed: %v", err)
	}

	if saveFile.Timestamp.Before(before) || saveFile.Timestamp.After(after) {
		t.Errorf("Timestamp %v not between %v and %v", saveFile.Timestamp, before, after)
	}
}

// =============================================================================
// Controller Tests (Integration) - Test file I/O
// =============================================================================

// TestSave_CreatesFile verifies that Save creates a file on disk.
func TestSave_CreatesFile(t *testing.T) {
	sim := model.NewSimulation(model.PolicyNone, 1)
	tracker := metrics.NewTracker()

	tmpDir := t.TempDir()
	savePath := filepath.Join(tmpDir, "create-test.sds")

	err := persistence.Save(savePath, "create-test", sim, tracker)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	info, err := os.Stat(savePath)
	if os.IsNotExist(err) {
		t.Fatal("Save file was not created")
	}
	if info.Size() == 0 {
		t.Error("Save file is empty")
	}
}

// TestLoad_RestoresState verifies that Load restores simulation state correctly.
func TestLoad_RestoresState(t *testing.T) {
	// Create simulation with meaningful state
	sim := model.NewSimulation(model.PolicyTameFlowCognitive, 99999)
	sim.Developers = append(sim.Developers, model.NewDeveloper("dev-001", "Alice", 1.0))
	sim.Developers = append(sim.Developers, model.NewDeveloper("dev-002", "Bob", 0.8))

	ticket := model.NewTicket("TKT-001", "Important feature", 8.0, model.LowUnderstanding)
	sim.Backlog = append(sim.Backlog, ticket)

	sim.CurrentTick = 100
	sim.SprintNumber = 5

	tracker := metrics.NewTracker()
	tracker.DORA.TotalDeploys = 20
	tracker.DORA.TotalIncidents = 3

	tmpDir := t.TempDir()
	savePath := filepath.Join(tmpDir, "restore-test.sds")

	// Save
	err := persistence.Save(savePath, "restore-test", sim, tracker)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load into fresh variables
	loadedSim, loadedTracker, err := persistence.Load(savePath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify simulation state
	if loadedSim.SizingPolicy != model.PolicyTameFlowCognitive {
		t.Errorf("SizingPolicy = %v, want TameFlowCognitive", loadedSim.SizingPolicy)
	}
	if loadedSim.Seed != 99999 {
		t.Errorf("Seed = %d, want 99999", loadedSim.Seed)
	}
	if loadedSim.CurrentTick != 100 {
		t.Errorf("CurrentTick = %d, want 100", loadedSim.CurrentTick)
	}
	if loadedSim.SprintNumber != 5 {
		t.Errorf("SprintNumber = %d, want 5", loadedSim.SprintNumber)
	}
	if len(loadedSim.Developers) != 2 {
		t.Errorf("Developers = %d, want 2", len(loadedSim.Developers))
	}
	if len(loadedSim.Backlog) != 1 {
		t.Errorf("Backlog = %d, want 1", len(loadedSim.Backlog))
	}

	// Verify metrics state
	if loadedTracker.DORA.TotalDeploys != 20 {
		t.Errorf("TotalDeploys = %d, want 20", loadedTracker.DORA.TotalDeploys)
	}
	if loadedTracker.DORA.TotalIncidents != 3 {
		t.Errorf("TotalIncidents = %d, want 3", loadedTracker.DORA.TotalIncidents)
	}
}

// TestLoad_NonexistentFile verifies that Load returns an error for missing files.
func TestLoad_NonexistentFile(t *testing.T) {
	_, _, err := persistence.Load("/nonexistent/path/file.sds")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

// TestListSaves_ReturnsFiles verifies that ListSaves finds save files in a directory.
func TestListSaves_ReturnsFiles(t *testing.T) {
	sim := model.NewSimulation(model.PolicyNone, 1)
	tracker := metrics.NewTracker()

	tmpDir := t.TempDir()

	// Create multiple saves
	for _, name := range []string{"save1", "save2", "save3"} {
		savePath := filepath.Join(tmpDir, name+".sds")
		err := persistence.Save(savePath, name, sim, tracker)
		if err != nil {
			t.Fatalf("Save %s failed: %v", name, err)
		}
	}

	// List saves
	saves, err := persistence.ListSaves(tmpDir)
	if err != nil {
		t.Fatalf("ListSaves failed: %v", err)
	}

	if len(saves) != 3 {
		t.Errorf("Found %d saves, want 3", len(saves))
	}
}
