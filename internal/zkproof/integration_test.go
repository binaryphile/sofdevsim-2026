//go:build integration

package zkproof

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// getProverPath returns the path to the zk-event-proofs project.
// Uses ZK_PROVER_PATH env var if set, otherwise defaults to ~/projects/zk-event-proofs.
func getProverPath() string {
	if path := os.Getenv("ZK_PROVER_PATH"); path != "" {
		return path
	}
	return filepath.Join(os.Getenv("HOME"), "projects", "zk-event-proofs")
}

// TestEndToEnd_BufferCrisisProof tests the complete pipeline:
// Go events → DetectBufferCrisis → InvokeProver → verify timestamps.
//
// Run with: go test ./internal/zkproof/... -tags=integration -timeout=10m -v
// Set ZK_PROVER_PATH to override the default prover location.
func TestEndToEnd_BufferCrisisProof(t *testing.T) {
	proverPath := getProverPath()

	// Check prover exists
	if _, err := os.Stat(filepath.Join(proverPath, "src", "prover.ts")); os.IsNotExist(err) {
		t.Fatalf("prover not found at %s", proverPath)
	}

	// 1. Create event history simulating a crisis-recovery scenario
	evts := []events.Event{
		// Normal operation
		events.NewBufferZoneChanged("sim-e2e", 3, model.FeverGreen, model.FeverYellow, 0.45),
		// Crisis begins at tick 8
		events.NewBufferZoneChanged("sim-e2e", 8, model.FeverYellow, model.FeverRed, 0.72),
		// Still in crisis
		events.NewBufferZoneChanged("sim-e2e", 10, model.FeverRed, model.FeverRed, 0.68),
		// Recovery at tick 15
		events.NewBufferZoneChanged("sim-e2e", 15, model.FeverRed, model.FeverGreen, 0.28),
	}

	// 2. Detect crisis sequences
	sequences := DetectBufferCrisis(evts)
	if len(sequences) != 1 {
		t.Fatalf("expected 1 sequence, got %d", len(sequences))
	}

	seq := sequences[0]
	if seq.CrisisTick != 8 {
		t.Errorf("CrisisTick = %d, want 8", seq.CrisisTick)
	}
	if seq.RecoveryTick != 15 {
		t.Errorf("RecoveryTick = %d, want 15", seq.RecoveryTick)
	}

	// 3. Generate ZK proof
	t.Log("Invoking prover (this will take ~30-60 seconds)...")
	result, err := InvokeProver(proverPath, "sim-e2e", seq)
	if err != nil {
		t.Fatalf("InvokeProver failed: %v", err)
	}

	// 4. Verify result
	if !result.Success {
		t.Fatalf("Proof generation failed: %s", result.Error)
	}

	// 5. Verify timestamps match detection (Success=true means PublicOutput has data)
	if result.PublicOutput.CrisisTimestamp != seq.CrisisTick {
		t.Errorf("Proof crisis timestamp = %d, detection = %d",
			result.PublicOutput.CrisisTimestamp, seq.CrisisTick)
	}

	if result.PublicOutput.RecoveryTimestamp != seq.RecoveryTick {
		t.Errorf("Proof recovery timestamp = %d, detection = %d",
			result.PublicOutput.RecoveryTimestamp, seq.RecoveryTick)
	}

	if result.PublicOutput.ProofType != 1 {
		t.Errorf("ProofType = %d, want 1 (buffer_crisis)", result.PublicOutput.ProofType)
	}

	if result.Proof == "" {
		t.Error("Proof data is empty")
	}

	t.Logf("E2E test passed: crisis=%d, recovery=%d, proof_size=%d bytes",
		result.PublicOutput.CrisisTimestamp,
		result.PublicOutput.RecoveryTimestamp,
		len(result.Proof))
}

// TestEndToEnd_NoCrisis verifies that sequences without crisis are rejected.
func TestEndToEnd_NoCrisis(t *testing.T) {
	proverPath := getProverPath()

	// Check prover exists
	if _, err := os.Stat(filepath.Join(proverPath, "src", "prover.ts")); os.IsNotExist(err) {
		t.Fatalf("prover not found at %s", proverPath)
	}

	// Create sequence with no crisis (no RED zone)
	seq := BufferCrisisSequence{
		CrisisTick:   0,
		RecoveryTick: 0,
		Events: []events.Event{
			events.NewBufferZoneChanged("sim-nocrisis", 5, model.FeverGreen, model.FeverYellow, 0.45),
			events.NewBufferZoneChanged("sim-nocrisis", 10, model.FeverYellow, model.FeverGreen, 0.30),
		},
	}

	t.Log("Invoking prover with no-crisis sequence (expect failure)...")
	result, err := InvokeProver(proverPath, "sim-nocrisis", seq)
	if err != nil {
		t.Fatalf("InvokeProver failed: %v", err)
	}

	// Should fail because no crisis event
	if result.Success {
		t.Error("Expected proof to fail for sequence without crisis")
	}

	if result.Error == "" {
		t.Error("Expected error message for failed proof")
	}

	t.Logf("Correctly rejected no-crisis sequence: %s", result.Error)
}
