package tui

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// TestFeverEmoji verifies FeverStatus→emoji mapping.
// Per design.md §"Testing via Event Replay": TUI is a projection, test pure transformations.
func TestFeverEmoji(t *testing.T) {
	tests := []struct {
		status model.FeverStatus
		want   string
	}{
		{model.FeverGreen, "🟢"},
		{model.FeverYellow, "🟡"},
		{model.FeverRed, "🔴"},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			got := FeverEmoji(tt.status)
			if got != tt.want {
				t.Errorf("FeverEmoji(%v) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

// TestFeverEmojiFromString verifies string→emoji mapping for client mode.
func TestFeverEmojiFromString(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"Green", "🟢"},
		{"Yellow", "🟡"},
		{"Red", "🔴"},
		{"Unknown", "🔴"}, // Default to red for unknown
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := feverEmojiFromString(tt.status)
			if got != tt.want {
				t.Errorf("feverEmojiFromString(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}
