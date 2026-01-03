package model_test

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// Test domain logic: fever status transitions based on buffer consumption
func TestSprint_UpdateFeverStatus(t *testing.T) {
	tests := []struct {
		name           string
		bufferDays     float64
		bufferConsumed float64
		wantStatus     model.FeverStatus
	}{
		{
			name:           "green when under 33% consumed",
			bufferDays:     10,
			bufferConsumed: 3.2, // 32%
			wantStatus:     model.FeverGreen,
		},
		{
			name:           "yellow when between 33% and 66%",
			bufferDays:     10,
			bufferConsumed: 5.0, // 50%
			wantStatus:     model.FeverYellow,
		},
		{
			name:           "red when over 66% consumed",
			bufferDays:     10,
			bufferConsumed: 7.0, // 70%
			wantStatus:     model.FeverRed,
		},
		{
			name:           "edge case: exactly 33% is yellow",
			bufferDays:     9,
			bufferConsumed: 3.0, // 33.3%
			wantStatus:     model.FeverYellow,
		},
		{
			name:           "edge case: exactly 66% is red",
			bufferDays:     9,
			bufferConsumed: 6.0, // 66.7%
			wantStatus:     model.FeverRed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sprint := model.NewSprint(1, 0, 10, 0.2)
			sprint.BufferDays = tt.bufferDays
			sprint.BufferConsumed = tt.bufferConsumed
			sprint.UpdateFeverStatus()

			if sprint.FeverStatus != tt.wantStatus {
				t.Errorf("UpdateFeverStatus() = %v, want %v (consumed %.1f%%)",
					sprint.FeverStatus, tt.wantStatus, tt.bufferConsumed/tt.bufferDays*100)
			}
		})
	}
}

// Test that ConsumeBuffer updates fever status atomically
func TestSprint_ConsumeBuffer(t *testing.T) {
	sprint := model.NewSprint(1, 0, 10, 0.2) // 2 days buffer

	// Start green
	if sprint.FeverStatus != model.FeverGreen {
		t.Fatalf("Expected initial status Green, got %v", sprint.FeverStatus)
	}

	// Consume to yellow
	sprint.ConsumeBuffer(0.8) // 40% consumed
	if sprint.FeverStatus != model.FeverYellow {
		t.Errorf("After 40%% consumed, expected Yellow, got %v", sprint.FeverStatus)
	}

	// Consume to red
	sprint.ConsumeBuffer(0.6) // 70% total
	if sprint.FeverStatus != model.FeverRed {
		t.Errorf("After 70%% consumed, expected Red, got %v", sprint.FeverStatus)
	}
}

// Test sprint progress calculation
func TestSprint_ProgressPct(t *testing.T) {
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
