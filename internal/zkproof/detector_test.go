package zkproof

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

func TestDetectBufferCrisis_FindsGreenToRedToGreen(t *testing.T) {
	// Simulate: green -> red (crisis) -> green (recovery)
	evts := []events.Event{
		events.NewTicked("sim-1", 5),
		events.NewBufferZoneChanged("sim-1", 8, model.FeverGreen, model.FeverRed, 0.72),
		events.NewTicked("sim-1", 10),
		events.NewBufferZoneChanged("sim-1", 15, model.FeverRed, model.FeverGreen, 0.28),
		events.NewTicked("sim-1", 20),
	}

	sequences := DetectBufferCrisis(evts)

	if len(sequences) != 1 {
		t.Fatalf("expected 1 sequence, got %d", len(sequences))
	}
	if sequences[0].CrisisTick != 8 {
		t.Errorf("CrisisTick = %d, want 8", sequences[0].CrisisTick)
	}
	if sequences[0].RecoveryTick != 15 {
		t.Errorf("RecoveryTick = %d, want 15", sequences[0].RecoveryTick)
	}
	if len(sequences[0].Events) != 2 {
		t.Errorf("Events count = %d, want 2", len(sequences[0].Events))
	}
}

func TestDetectBufferCrisis_NoRecoveryYet(t *testing.T) {
	// Incomplete sequence - still in crisis
	evts := []events.Event{
		events.NewBufferZoneChanged("sim-1", 8, model.FeverGreen, model.FeverRed, 0.72),
	}

	sequences := DetectBufferCrisis(evts)

	// Incomplete sequences should not be returned
	if len(sequences) != 0 {
		t.Errorf("expected 0 sequences for incomplete crisis, got %d", len(sequences))
	}
}

func TestDetectBufferCrisis_MultipleCrises(t *testing.T) {
	// Two complete crisis->recovery cycles
	evts := []events.Event{
		events.NewBufferZoneChanged("sim-1", 5, model.FeverGreen, model.FeverRed, 0.70),
		events.NewBufferZoneChanged("sim-1", 10, model.FeverRed, model.FeverGreen, 0.30),
		events.NewBufferZoneChanged("sim-1", 20, model.FeverGreen, model.FeverRed, 0.75),
		events.NewBufferZoneChanged("sim-1", 25, model.FeverRed, model.FeverGreen, 0.25),
	}

	sequences := DetectBufferCrisis(evts)

	if len(sequences) != 2 {
		t.Fatalf("expected 2 sequences, got %d", len(sequences))
	}
	if sequences[0].CrisisTick != 5 || sequences[0].RecoveryTick != 10 {
		t.Errorf("first sequence: crisis=%d recovery=%d, want 5,10",
			sequences[0].CrisisTick, sequences[0].RecoveryTick)
	}
	if sequences[1].CrisisTick != 20 || sequences[1].RecoveryTick != 25 {
		t.Errorf("second sequence: crisis=%d recovery=%d, want 20,25",
			sequences[1].CrisisTick, sequences[1].RecoveryTick)
	}
}

func TestDetectBufferCrisis_Max32Events(t *testing.T) {
	// Sequence with >32 events should be truncated
	evts := make([]events.Event, 0, 35)
	evts = append(evts, events.NewBufferZoneChanged("sim-1", 1, model.FeverGreen, model.FeverRed, 0.70))
	for i := 2; i <= 33; i++ { // justified:IX
		// Add intermediate events (staying in red)
		evts = append(evts, events.NewBufferZoneChanged("sim-1", i, model.FeverRed, model.FeverRed, 0.70))
	}
	evts = append(evts, events.NewBufferZoneChanged("sim-1", 34, model.FeverRed, model.FeverGreen, 0.30))

	sequences := DetectBufferCrisis(evts)

	if len(sequences) != 1 {
		t.Fatalf("expected 1 sequence, got %d", len(sequences))
	}
	if len(sequences[0].Events) > 32 {
		t.Errorf("Events count = %d, want <= 32", len(sequences[0].Events))
	}
}
