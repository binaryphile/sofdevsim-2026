package export

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// Exporter handles CSV export of simulation data.
//
// Value receiver: only reads fields, no mutation.
type Exporter struct {
	sim        *model.Simulation
	tracker    metrics.Tracker
	comparison *metrics.ComparisonResult
}

// ExportResult contains the results of an export operation
type ExportResult struct {
	Path            string
	TicketCount     int
	SprintCount     int
	IncidentCount   int
	MetricsCount    int
	ComparisonCount int // 0 or 4 (one row per metric)
}

// Summary returns a human-readable summary of the export
func (r ExportResult) Summary() string {
	parts := []string{
		fmt.Sprintf("Exported to %s/", filepath.Base(r.Path)),
		fmt.Sprintf("tickets: %d", r.TicketCount),
		fmt.Sprintf("sprints: %d", r.SprintCount),
		fmt.Sprintf("incidents: %d", r.IncidentCount),
	}
	if r.ComparisonCount > 0 {
		parts = append(parts, "comparison: included")
	}
	return strings.Join(parts, " | ")
}

// New creates an exporter for the given simulation
func New(sim *model.Simulation, tracker metrics.Tracker, comparison *metrics.ComparisonResult) Exporter {
	return Exporter{
		sim:        sim,
		tracker:    tracker,
		comparison: comparison,
	}
}

// Export writes CSV files to a timestamped directory in the current working directory
func (e Exporter) Export() (ExportResult, error) {
	timestamp := time.Now().Format("20060102-150405")
	dirName := fmt.Sprintf("sofdevsim-export-%s", timestamp)
	return e.ExportTo(dirName)
}

// ExportTo writes CSV files to a subdirectory within the given base path
func (e Exporter) ExportTo(basePath string) (ExportResult, error) {
	timestamp := time.Now().Format("20060102-150405")
	dirName := fmt.Sprintf("sofdevsim-export-%s", timestamp)
	outputDir := filepath.Join(basePath, dirName)

	result := ExportResult{
		Path: outputDir,
	}

	// Create output directory
	if err := createDir(outputDir); err != nil {
		return result, fmt.Errorf("failed to create export directory: %w", err)
	}

	// Write all CSV files
	if err := e.writeMetadata(outputDir); err != nil {
		return result, fmt.Errorf("failed to write metadata.csv: %w", err)
	}

	ticketCount, err := e.writeTickets(outputDir)
	if err != nil {
		return result, fmt.Errorf("failed to write tickets.csv: %w", err)
	}
	result.TicketCount = ticketCount

	sprintCount, err := e.writeSprints(outputDir)
	if err != nil {
		return result, fmt.Errorf("failed to write sprints.csv: %w", err)
	}
	result.SprintCount = sprintCount

	incidentCount, err := e.writeIncidents(outputDir)
	if err != nil {
		return result, fmt.Errorf("failed to write incidents.csv: %w", err)
	}
	result.IncidentCount = incidentCount

	if err := e.writeMetrics(outputDir); err != nil {
		return result, fmt.Errorf("failed to write metrics.csv: %w", err)
	}
	result.MetricsCount = 1

	// Only write comparison if we have one
	if e.comparison != nil {
		if err := e.writeComparison(outputDir); err != nil {
			return result, fmt.Errorf("failed to write comparison.csv: %w", err)
		}
		result.ComparisonCount = 4
	}

	return result, nil
}
