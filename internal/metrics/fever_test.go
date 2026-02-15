package metrics_test

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// Test fever chart updates from sprint state
// Uses 50% progress as baseline to test buffer thresholds with diagonal zones
func TestFeverChart_SyncsWithSprintBuffer(t *testing.T) {
	tests := []struct {
		name           string
		bufferDays     float64
		bufferConsumed float64
		progress       float64
		wantPctUsed    float64
		wantStatus     model.FeverStatus
	}{
		{
			name:           "green zone",
			bufferDays:     10,
			bufferConsumed: 2,
			progress:       0.5,
			wantPctUsed:    20,
			wantStatus:     model.FeverGreen,
		},
		{
			name:           "yellow zone",
			bufferDays:     10,
			bufferConsumed: 5,
			progress:       0.5,
			wantPctUsed:    50,
			wantStatus:     model.FeverYellow,
		},
		{
			name:           "red zone",
			bufferDays:     10,
			bufferConsumed: 8,
			progress:       0.5,
			wantPctUsed:    80,
			wantStatus:     model.FeverRed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sprint := model.NewSprint(1, 0, 10, 0.2)
			sprint.BufferDays = tt.bufferDays
			sprint.BufferConsumed = tt.bufferConsumed
			sprint = sprint.WithUpdatedFeverStatus(tt.progress)

			fever := metrics.NewFeverChart()
			fever = fever.Updated(sprint)

			if fever.PercentUsed() != tt.wantPctUsed {
				t.Errorf("PercentUsed() = %.1f, want %.1f", fever.PercentUsed(), tt.wantPctUsed)
			}

			if fever.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", fever.Status, tt.wantStatus)
			}

			if fever.BufferRemaining != tt.bufferDays-tt.bufferConsumed {
				t.Errorf("BufferRemaining = %.1f, want %.1f", fever.BufferRemaining, tt.bufferDays-tt.bufferConsumed)
			}
		})
	}
}

// Test history accumulation for sparklines
func TestFeverChart_HistoryValues(t *testing.T) {
	sprint := model.NewSprint(1, 0, 10, 0.2)
	sprint.BufferDays = 10
	fever := metrics.NewFeverChart()

	// Simulate buffer consumption over time with increasing progress
	consumptions := []float64{1, 2, 3, 5, 7}
	progresses := []float64{0.1, 0.2, 0.3, 0.5, 0.7}
	for i, consumed := range consumptions { // justified:IX
		sprint.BufferConsumed = consumed
		sprint = sprint.WithUpdatedFeverStatus(progresses[i])
		fever = fever.Updated(sprint)
	}

	values := fever.HistoryValues()
	if len(values) != 5 {
		t.Fatalf("HistoryValues has %d entries, want 5", len(values))
	}

	// Values should match consumption percentages
	expected := []float64{10, 20, 30, 50, 70}
	for i, v := range values { // justified:IX
		if v != expected[i] {
			t.Errorf("HistoryValues[%d] = %.1f, want %.1f", i, v, expected[i])
		}
	}
}

// Test status helpers
// Uses 50% progress as baseline for consistent diagonal zone testing
func TestFeverChart_StatusHelpers(t *testing.T) {
	sprint := model.NewSprint(1, 0, 10, 0.2)
	sprint.BufferDays = 10
	fever := metrics.NewFeverChart()
	progress := 0.5

	// Green (20% buffer at 50% progress)
	sprint.BufferConsumed = 2
	sprint = sprint.WithUpdatedFeverStatus(progress)
	fever = fever.Updated(sprint)
	if !fever.IsGreen() || fever.IsYellow() || fever.IsRed() {
		t.Error("Expected only IsGreen() to be true at 20% buffer/50% progress")
	}

	// Yellow (50% buffer at 50% progress)
	sprint.BufferConsumed = 5
	sprint = sprint.WithUpdatedFeverStatus(progress)
	fever = fever.Updated(sprint)
	if fever.IsGreen() || !fever.IsYellow() || fever.IsRed() {
		t.Error("Expected only IsYellow() to be true at 50% buffer/50% progress")
	}

	// Red (80% buffer at 50% progress)
	sprint.BufferConsumed = 8
	sprint = sprint.WithUpdatedFeverStatus(progress)
	fever = fever.Updated(sprint)
	if fever.IsGreen() || fever.IsYellow() || !fever.IsRed() {
		t.Error("Expected only IsRed() to be true at 80% buffer/50% progress")
	}
}
