package zkproof

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// ProofResult is the JSON structure returned by the TypeScript prover.
// This matches the ProofResult interface in zk-event-proofs/src/prover.ts.
// When Success is false, PublicOutput will be zero value - check Success first.
type ProofResult struct {
	Success      bool              `json:"success"`
	Error        string            `json:"error,omitempty"`
	Proof        string            `json:"proof,omitempty"`
	PublicOutput ProofPublicOutput `json:"publicOutput"`
}

// ProofPublicOutput contains the public output from the proof.
type ProofPublicOutput struct {
	SequenceHash      string `json:"sequenceHash"`
	CrisisTimestamp   int    `json:"crisisTimestamp"`
	RecoveryTimestamp int    `json:"recoveryTimestamp"`
	ProofType         int    `json:"proofType"`
}

// InvokeProver calls the TypeScript prover as a subprocess.
// proverPath is the directory containing the zk-event-proofs project.
// Returns the proof result or an error if invocation fails.
func InvokeProver(proverPath, simID string, seq BufferCrisisSequence) (ProofResult, error) {
	// Export the sequence as JSON
	requestJSON, err := ExportProofRequest(simID, seq)
	if err != nil {
		return ProofResult{}, fmt.Errorf("export request: %w", err)
	}

	// Set up 5-minute timeout for circuit compilation + proving
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Run: npx tsx src/prover.ts
	cmd := exec.CommandContext(ctx, "npx", "tsx", "src/prover.ts")
	cmd.Dir = proverPath
	cmd.Stdin = bytes.NewReader(requestJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ProofResult{}, fmt.Errorf("prover timeout after 5 minutes")
		}
		// Include stderr for debugging
		return ProofResult{}, fmt.Errorf("prover failed: %w\nstderr: %s", err, stderr.String())
	}

	// Parse result
	var result ProofResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return ProofResult{}, fmt.Errorf("parse result: %w\nstdout: %s", err, stdout.String())
	}

	return result, nil
}
