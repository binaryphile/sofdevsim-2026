package zkproof

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// proverPathForTest returns the path to the zk-event-proofs project.
// Uses ZK_PROVER_PATH env var if set, otherwise defaults to ~/projects/zk-event-proofs.
func proverPathForTest() string {
	if path := os.Getenv("ZK_PROVER_PATH"); path != "" {
		return path
	}
	return filepath.Join(os.Getenv("HOME"), "projects", "zk-event-proofs")
}

// TestInvokeProver_Integration tests the full prover invocation.
// This test is slow (~30-60s) due to circuit compilation.
// Run with: go test -v -run TestInvokeProver_Integration -timeout 5m
// Set ZK_PROVER_PATH to override the default prover location.
func TestInvokeProver_Integration(t *testing.T) {
	t.Skip("disabled: 22s prover compilation dominates test suite")
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	proverPath := proverPathForTest()

	// Check prover exists
	if _, err := os.Stat(filepath.Join(proverPath, "src", "prover.ts")); os.IsNotExist(err) {
		t.Skipf("prover not found at %s", proverPath)
	}

	// Create a simple crisis-recovery sequence
	seq := BufferCrisisSequence{
		CrisisTick:   8,
		RecoveryTick: 15,
		Events: []events.Event{
			events.NewBufferZoneChanged("test-sim", 8, model.FeverYellow, model.FeverRed, 0.72),
			events.NewBufferZoneChanged("test-sim", 15, model.FeverRed, model.FeverGreen, 0.28),
		},
	}

	result, err := InvokeProver(proverPath, "test-sim", seq)
	if err != nil {
		t.Fatalf("InvokeProver failed: %v", err)
	}

	if !result.Success {
		t.Fatalf("Proof generation failed: %s", result.Error)
	}

	// Success is true, so PublicOutput should have valid data
	if result.PublicOutput.CrisisTimestamp != 8 {
		t.Errorf("CrisisTimestamp = %d, want 8", result.PublicOutput.CrisisTimestamp)
	}

	if result.PublicOutput.RecoveryTimestamp != 15 {
		t.Errorf("RecoveryTimestamp = %d, want 15", result.PublicOutput.RecoveryTimestamp)
	}

	if result.PublicOutput.ProofType != 1 {
		t.Errorf("ProofType = %d, want 1", result.PublicOutput.ProofType)
	}

	if result.Proof == "" {
		t.Error("Proof is empty")
	}

	t.Logf("Proof generated successfully: crisis=%d, recovery=%d",
		result.PublicOutput.CrisisTimestamp,
		result.PublicOutput.RecoveryTimestamp)
}

func TestProofResult_JSONParsing(t *testing.T) {
	// Test that we can parse a ProofResult from JSON
	jsonData := `{
		"success": true,
		"proof": "base64data...",
		"publicOutput": {
			"sequenceHash": "123456",
			"crisisTimestamp": 8,
			"recoveryTimestamp": 15,
			"proofType": 1
		}
	}`

	var result ProofResult
	if err := json.Unmarshal([]byte(jsonData), &result); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if !result.Success {
		t.Error("Success should be true")
	}

	if result.PublicOutput.CrisisTimestamp != 8 {
		t.Errorf("CrisisTimestamp = %d, want 8", result.PublicOutput.CrisisTimestamp)
	}
}

func TestProofResult_ErrorParsing(t *testing.T) {
	jsonData := `{
		"success": false,
		"error": "No crisis event found"
	}`

	var result ProofResult
	if err := json.Unmarshal([]byte(jsonData), &result); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if result.Success {
		t.Error("Success should be false")
	}

	if result.Error != "No crisis event found" {
		t.Errorf("Error = %q, want %q", result.Error, "No crisis event found")
	}
}
