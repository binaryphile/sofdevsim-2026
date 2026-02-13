package zkproof

import (
	"encoding/json"

	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// ProofRequest is the JSON structure expected by the TypeScript prover.
// This matches the ProofRequest interface in zk-event-proofs/src/types.ts.
type ProofRequest struct {
	Version      int                 `json:"version"`
	SimulationID string              `json:"simulationId"`
	SequenceType string              `json:"sequenceType"`
	Events       []ProofRequestEvent `json:"events"`
}

// ProofRequestEvent is a single event in the proof request.
// This matches ProofRequestEvent in zk-event-proofs/src/types.ts.
type ProofRequestEvent struct {
	Type string                 `json:"type"`
	ID   string                 `json:"id"`
	Tick int                    `json:"tick"`
	Data map[string]interface{} `json:"data"`
}

// toProofRequestEvent converts a domain event to a ProofRequestEvent.
func toProofRequestEvent(e events.Event) ProofRequestEvent {
	pre := ProofRequestEvent{
		Type: e.EventType(),
		ID:   e.EventID(),
		Tick: e.OccurrenceTime(),
		Data: make(map[string]interface{}),
	}

	// Extract event-specific data based on type
	switch evt := e.(type) {
	case events.BufferZoneChanged:
		pre.Data["oldZone"] = feverStatusString(evt.OldZone)
		pre.Data["newZone"] = feverStatusString(evt.NewZone)
		pre.Data["penetration"] = evt.Penetration
	}

	return pre
}

// feverStatusString converts FeverStatus to the string format expected by TypeScript.
func feverStatusString(fs model.FeverStatus) string {
	switch fs {
	case model.FeverGreen:
		return "Green"
	case model.FeverYellow:
		return "Yellow"
	case model.FeverRed:
		return "Red"
	default:
		return "Unknown"
	}
}

// ExportProofRequest creates a JSON-serialized ProofRequest from a BufferCrisisSequence.
func ExportProofRequest(simID string, seq BufferCrisisSequence) ([]byte, error) {
	evts := slice.MapTo[ProofRequestEvent](seq.Events).Map(toProofRequestEvent)

	request := ProofRequest{
		Version:      1,
		SimulationID: simID,
		SequenceType: "buffer-crisis",
		Events:       evts,
	}

	return json.MarshalIndent(request, "", "  ")
}
