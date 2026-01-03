package export_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/export"
	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// =============================================================================
// DOMAIN/ALGORITHM TESTS - Unit test these heavily
// =============================================================================

// Test theoretical bounds calculation - pure domain algorithm
func TestGetVarianceBounds(t *testing.T) {
	tests := []struct {
		level   model.UnderstandingLevel
		wantMin float64
		wantMax float64
	}{
		{model.HighUnderstanding, 0.95, 1.05},
		{model.MediumUnderstanding, 0.80, 1.20},
		{model.LowUnderstanding, 0.50, 1.50},
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			min, max := export.GetVarianceBounds(tt.level)
			if min != tt.wantMin {
				t.Errorf("GetVarianceBounds(%v) min = %v, want %v", tt.level, min, tt.wantMin)
			}
			if max != tt.wantMax {
				t.Errorf("GetVarianceBounds(%v) max = %v, want %v", tt.level, max, tt.wantMax)
			}
		})
	}
}

// Test within expected bounds check - pure domain algorithm with edge cases
func TestIsWithinExpected(t *testing.T) {
	tests := []struct {
		name      string
		actual    float64
		estimated float64
		level     model.UnderstandingLevel
		want      bool
	}{
		{
			name:      "high understanding within bounds",
			actual:    5.0,
			estimated: 5.0,
			level:     model.HighUnderstanding,
			want:      true,
		},
		{
			name:      "high understanding outside bounds",
			actual:    6.0,
			estimated: 5.0, // ratio = 1.2, bounds = 0.95-1.05
			level:     model.HighUnderstanding,
			want:      false,
		},
		{
			name:      "low understanding within wide bounds",
			actual:    7.0,
			estimated: 5.0, // ratio = 1.4, bounds = 0.50-1.50
			level:     model.LowUnderstanding,
			want:      true,
		},
		{
			name:      "low understanding outside wide bounds",
			actual:    8.0,
			estimated: 5.0, // ratio = 1.6, bounds = 0.50-1.50
			level:     model.LowUnderstanding,
			want:      false,
		},
		{
			name:      "zero estimated with zero actual",
			actual:    0,
			estimated: 0,
			level:     model.HighUnderstanding,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := export.IsWithinExpected(tt.actual, tt.estimated, tt.level)
			if got != tt.want {
				t.Errorf("IsWithinExpected(%v, %v, %v) = %v, want %v",
					tt.actual, tt.estimated, tt.level, got, tt.want)
			}
		})
	}
}

// =============================================================================
// CONTROLLER TESTS - One happy path + edge cases that can't be unit tested
// =============================================================================

// Integration test: happy path - export creates all expected files with valid structure
func TestExporter_Export_HappyPath(t *testing.T) {
	sim := model.NewSimulation(model.PolicyDORAStrict, 12345)
	sim.AddDeveloper(model.NewDeveloper("dev-1", "Alice", 1.0))
	sim.StartSprint()

	// Add completed ticket
	ticket := model.NewTicket("TKT-001", "Test task", 2, model.HighUnderstanding)
	ticket.StartedTick = 1
	ticket.CompletedTick = 5
	sim.CompletedTickets = append(sim.CompletedTickets, ticket)

	tmpDir := t.TempDir()
	exporter := export.New(sim, nil, nil)
	result, err := exporter.ExportTo(tmpDir)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Observable outcome: all 5 base files created
	expectedFiles := []string{
		"metadata.csv",
		"tickets.csv",
		"sprints.csv",
		"incidents.csv",
		"metrics.csv",
	}

	for _, file := range expectedFiles {
		path := filepath.Join(result.Path, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s not created", file)
		}
	}

	// Observable outcome: comparison.csv NOT created when not provided
	compPath := filepath.Join(result.Path, "comparison.csv")
	if _, err := os.Stat(compPath); !os.IsNotExist(err) {
		t.Error("comparison.csv should not exist when no comparison provided")
	}

	// Observable outcome: each CSV has header + at least structure
	for _, file := range expectedFiles {
		content, err := os.ReadFile(filepath.Join(result.Path, file))
		if err != nil {
			t.Errorf("Failed to read %s: %v", file, err)
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		if len(lines) < 1 {
			t.Errorf("%s should have at least a header row", file)
		}
	}

	// Observable outcome: result counts are populated
	if result.TicketCount != 1 {
		t.Errorf("Expected TicketCount=1, got %d", result.TicketCount)
	}
}

// Integration test: edge case - comparison.csv created when comparison provided
func TestExporter_Export_WithComparison(t *testing.T) {
	sim := model.NewSimulation(model.PolicyDORAStrict, 12345)

	// Need completed ticket for export
	ticket := model.NewTicket("TKT-001", "Test", 2, model.HighUnderstanding)
	sim.CompletedTickets = append(sim.CompletedTickets, ticket)

	// Create comparison result
	comparison := &metrics.ComparisonResult{
		PolicyA: model.PolicyDORAStrict,
		PolicyB: model.PolicyTameFlowCognitive,
		Seed:    12345,
		ResultsA: metrics.SimulationResult{
			Policy:       model.PolicyDORAStrict,
			FinalMetrics: metrics.NewDORAMetrics(),
		},
		ResultsB: metrics.SimulationResult{
			Policy:       model.PolicyTameFlowCognitive,
			FinalMetrics: metrics.NewDORAMetrics(),
		},
		LeadTimeWinner:   model.PolicyTameFlowCognitive,
		DeployFreqWinner: model.PolicyTameFlowCognitive,
		MTTRWinner:       model.PolicyDORAStrict,
		CFRWinner:        model.PolicyTameFlowCognitive,
	}

	tmpDir := t.TempDir()
	exporter := export.New(sim, nil, comparison)
	result, err := exporter.ExportTo(tmpDir)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Observable outcome: comparison.csv exists
	compPath := filepath.Join(result.Path, "comparison.csv")
	if _, err := os.Stat(compPath); os.IsNotExist(err) {
		t.Error("comparison.csv should exist when comparison provided")
	}

	// Observable outcome: comparison.csv has header + 4 metric rows
	content, err := os.ReadFile(compPath)
	if err != nil {
		t.Fatalf("Failed to read comparison.csv: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 5 {
		t.Errorf("comparison.csv should have 5 lines (header + 4 metrics), got %d", len(lines))
	}

	// Observable outcome: result reflects comparison
	if result.ComparisonCount != 4 {
		t.Errorf("Expected ComparisonCount=4, got %d", result.ComparisonCount)
	}
}
