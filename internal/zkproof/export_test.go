package zkproof

import (
	"encoding/json"
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

func TestExportProofRequest_ValidJSON(t *testing.T) {
	seq := BufferCrisisSequence{
		CrisisTick:   8,
		RecoveryTick: 15,
		Events: []events.Event{
			events.NewBufferZoneChanged("sim-1", 8, model.FeverYellow, model.FeverRed, 0.72),
			events.NewBufferZoneChanged("sim-1", 15, model.FeverRed, model.FeverGreen, 0.28),
		},
	}

	data, err := ExportProofRequest("sim-1", seq)
	if err != nil {
		t.Fatalf("ExportProofRequest failed: %v", err)
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	// Check required fields
	if result["version"] != float64(1) {
		t.Errorf("version = %v, want 1", result["version"])
	}
	if result["simulationId"] != "sim-1" {
		t.Errorf("simulationId = %v, want sim-1", result["simulationId"])
	}
	if result["sequenceType"] != "buffer-crisis" {
		t.Errorf("sequenceType = %v, want buffer-crisis", result["sequenceType"])
	}
}

func TestExportProofRequest_EventMapping(t *testing.T) {
	seq := BufferCrisisSequence{
		CrisisTick:   8,
		RecoveryTick: 15,
		Events: []events.Event{
			events.NewBufferZoneChanged("sim-1", 8, model.FeverYellow, model.FeverRed, 0.72),
		},
	}

	data, err := ExportProofRequest("sim-1", seq)
	if err != nil {
		t.Fatalf("ExportProofRequest failed: %v", err)
	}

	// Parse and check event structure
	var request ProofRequest
	if err := json.Unmarshal(data, &request); err != nil {
		t.Fatalf("Failed to parse ProofRequest: %v", err)
	}

	if len(request.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(request.Events))
	}

	evt := request.Events[0]
	if evt.Type != "BufferZoneChanged" {
		t.Errorf("event type = %s, want BufferZoneChanged", evt.Type)
	}
	if evt.Tick != 8 {
		t.Errorf("event tick = %d, want 8", evt.Tick)
	}
	if evt.Data["oldZone"] != "Yellow" {
		t.Errorf("event data.oldZone = %v, want Yellow", evt.Data["oldZone"])
	}
	if evt.Data["newZone"] != "Red" {
		t.Errorf("event data.newZone = %v, want Red", evt.Data["newZone"])
	}
	if evt.Data["penetration"] != 0.72 {
		t.Errorf("event data.penetration = %v, want 0.72", evt.Data["penetration"])
	}
}
