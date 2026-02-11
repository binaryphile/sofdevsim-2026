package model_test

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// Test domain logic: fever status transitions based on buffer consumption
// Uses 50% progress as baseline to test buffer thresholds
func TestSprint_BufferPressureAffectsFeverStatus(t *testing.T) {
	tests := []struct {
		name           string
		bufferDays     float64
		bufferConsumed float64
		progress       float64
		wantStatus     model.FeverStatus
	}{
		{
			name:           "green when buffer behind progress",
			bufferDays:     10,
			bufferConsumed: 3.0, // 30% buffer at 50% progress = green
			progress:       0.50,
			wantStatus:     model.FeverGreen,
		},
		{
			name:           "yellow when buffer tracking progress",
			bufferDays:     10,
			bufferConsumed: 5.0, // 50% buffer at 50% progress = yellow
			progress:       0.50,
			wantStatus:     model.FeverYellow,
		},
		{
			name:           "red when buffer ahead of progress",
			bufferDays:     10,
			bufferConsumed: 7.0, // 70% buffer at 50% progress = red
			progress:       0.50,
			wantStatus:     model.FeverRed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sprint := model.NewSprint(1, 0, 10, 0.2)
			sprint.BufferDays = tt.bufferDays
			sprint.BufferConsumed = tt.bufferConsumed
			sprint = sprint.WithUpdatedFeverStatus(tt.progress)

			if sprint.FeverStatus != tt.wantStatus {
				t.Errorf("UpdateFeverStatus() = %v, want %v (consumed %.1f%%)",
					sprint.FeverStatus, tt.wantStatus, tt.bufferConsumed/tt.bufferDays*100)
			}
		})
	}
}

// Test that ConsumeBuffer updates fever status atomically
func TestSprint_BufferConsumptionUpdatesFeverStatus(t *testing.T) {
	sprint := model.NewSprint(1, 0, 10, 0.2) // 2 days buffer

	// Start green (at 50% progress, 0% buffer = green)
	sprint = sprint.WithUpdatedFeverStatus(0.5)
	if sprint.FeverStatus != model.FeverGreen {
		t.Fatalf("Expected initial status Green, got %v", sprint.FeverStatus)
	}

	// Consume to yellow (at 50% progress, 40% buffer = yellow)
	sprint = sprint.WithConsumedBuffer(0.8, 0.5) // 40% consumed
	if sprint.FeverStatus != model.FeverYellow {
		t.Errorf("After 40%% consumed at 50%% progress, expected Yellow, got %v", sprint.FeverStatus)
	}

	// Consume to red (at 50% progress, 70% buffer = red)
	sprint = sprint.WithConsumedBuffer(0.6, 0.5) // 70% total
	if sprint.FeverStatus != model.FeverRed {
		t.Errorf("After 70%% consumed at 50%% progress, expected Red, got %v", sprint.FeverStatus)
	}
}

// Test sprint progress calculation
func TestSprint_ProgressPct_CalculatesPercentage(t *testing.T) {
	sprint := model.NewSprint(1, 0, 10, 0.2)

	tests := []struct {
		currentDay int
		wantPct    float64
	}{
		{0, 0.0},
		{5, 0.5},
		{10, 1.0},
		{15, 1.0}, // clamped at 100%
	}

	for _, tt := range tests {
		got := sprint.ProgressPct(tt.currentDay)
		if got != tt.wantPct {
			t.Errorf("ProgressPct(%d) = %v, want %v", tt.currentDay, got, tt.wantPct)
		}
	}
}

// Test TameFlow diagonal zone boundaries
// Zone is determined by comparing progress to buffer consumption:
// - Green: bufferPct <= progress * 0.66
// - Red: bufferPct >= 0.33 + progress * 0.67
// - Yellow: between thresholds
func TestWithUpdatedFeverStatus_DiagonalThresholds(t *testing.T) {
	tests := []struct {
		name           string
		bufferConsumed float64
		bufferDays     float64
		progress       float64
		wantStatus     model.FeverStatus
	}{
		// Green: bufferPct <= progress * 0.66
		{"green_ahead", 1.0, 10.0, 0.50, model.FeverGreen},       // 10% buffer, 50% progress
		{"green_at_boundary", 3.3, 10.0, 0.50, model.FeverGreen}, // 33% = 50% * 0.66

		// Yellow: between thresholds
		{"yellow_on_track", 4.0, 10.0, 0.50, model.FeverYellow}, // 40% buffer, 50% progress

		// Red: bufferPct >= 0.33 + progress * 0.67
		{"red_early_danger", 4.0, 10.0, 0.10, model.FeverRed},  // 40% buffer, only 10% progress
		{"red_at_boundary", 6.7, 10.0, 0.50, model.FeverRed},   // 67% = 33% + 50%*0.67

		// Edge cases
		{"start_state", 0, 10.0, 0, model.FeverGreen},             // 0% consumed = ahead
		{"buffer_no_progress", 1.0, 10.0, 0, model.FeverYellow},   // 10% buffer at 0% = early warning
		{"done_buffer_used", 10.0, 10.0, 1.0, model.FeverYellow},  // 100% buffer, 100% progress = on track
		{"buffer_overrun", 12.0, 10.0, 0.80, model.FeverRed},      // 120% buffer consumed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sprint := model.Sprint{
				BufferDays:     tt.bufferDays,
				BufferConsumed: tt.bufferConsumed,
			}
			got := sprint.WithUpdatedFeverStatus(tt.progress)
			if got.FeverStatus != tt.wantStatus {
				t.Errorf("got %v, want %v (buffer %.0f%%, progress %.0f%%)",
					got.FeverStatus, tt.wantStatus,
					tt.bufferConsumed/tt.bufferDays*100, tt.progress*100)
			}
		})
	}
}

// Test WIP tracking for export
func TestSprint_AvgWIP_CalculatesAverage(t *testing.T) {
	tests := []struct {
		name     string
		wipSum   int
		wipTicks int
		want     float64
	}{
		{
			name:     "no ticks returns zero",
			wipSum:   0,
			wipTicks: 0,
			want:     0,
		},
		{
			name:     "single tick",
			wipSum:   3,
			wipTicks: 1,
			want:     3.0,
		},
		{
			name:     "average over multiple ticks",
			wipSum:   15, // 3+3+4+5 over 4 ticks
			wipTicks: 4,
			want:     3.75,
		},
		{
			name:     "zero WIP sum with ticks",
			wipSum:   0,
			wipTicks: 5,
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sprint := model.NewSprint(1, 0, 10, 0.2)
			sprint.WIPSum = tt.wipSum
			sprint.WIPTicks = tt.wipTicks

			got := sprint.AvgWIP()
			if got != tt.want {
				t.Errorf("AvgWIP() = %v, want %v", got, tt.want)
			}
		})
	}
}
