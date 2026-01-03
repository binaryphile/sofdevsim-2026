package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// createDir creates a directory, handling collisions with sequence numbers
func createDir(path string) error {
	// Try the original path first
	if err := os.Mkdir(path, 0755); err == nil {
		return nil
	}

	// If it exists, try with sequence numbers
	for i := 1; i <= 100; i++ {
		seqPath := fmt.Sprintf("%s-%d", path, i)
		if err := os.Mkdir(seqPath, 0755); err == nil {
			return nil
		}
	}

	return fmt.Errorf("could not create directory after 100 attempts")
}

// writeMetadata writes metadata.csv
func (e *Exporter) writeMetadata(outputDir string) error {
	path := filepath.Join(outputDir, "metadata.csv")
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write(MetadataHeader); err != nil {
		return err
	}

	// Serialize phase effort distribution as JSON
	distJSON, _ := json.Marshal(PhaseEffortDistribution)

	row := []string{
		fmt.Sprintf("%d", e.sim.Seed),
		e.sim.SizingPolicy.String(),
		fmt.Sprintf("%d", e.sim.SprintNumber),
		time.Now().Format(time.RFC3339),
		string(distJSON),
	}

	return writer.Write(row)
}

// writeTickets writes tickets.csv and returns the count
func (e *Exporter) writeTickets(outputDir string) (int, error) {
	path := filepath.Join(outputDir, "tickets.csv")
	file, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write(TicketsHeader); err != nil {
		return 0, err
	}

	// Write completed tickets
	count := 0
	for _, ticket := range e.sim.CompletedTickets {
		row := formatTicketRow(ticket, e.sim.SizingPolicy, e.sim.SprintNumber)
		if err := writer.Write(row); err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

// writeSprints writes sprints.csv and returns the count
func (e *Exporter) writeSprints(outputDir string) (int, error) {
	path := filepath.Join(outputDir, "sprints.csv")
	file, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write(SprintsHeader); err != nil {
		return 0, err
	}

	// Write current sprint if it exists
	count := 0
	if e.sim.CurrentSprint != nil {
		startDay := e.sim.CurrentSprint.StartDay
		ticketsStarted := len(e.sim.CurrentSprint.Tickets)
		// completedInSprint returns true if ticket was completed after sprint start.
		completedInSprint := func(t model.Ticket) bool {
			return t.CompletedTick >= startDay
		}
		ticketsCompleted := slice.From(e.sim.CompletedTickets).
			KeepIf(completedInSprint).
			Len()
		incidents := len(e.sim.ResolvedIncidents) + len(e.sim.OpenIncidents)

		row := formatSprintRow(*e.sim.CurrentSprint, ticketsStarted, ticketsCompleted, incidents)
		if err := writer.Write(row); err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

// writeIncidents writes incidents.csv and returns the count
func (e *Exporter) writeIncidents(outputDir string) (int, error) {
	path := filepath.Join(outputDir, "incidents.csv")
	file, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write(IncidentsHeader); err != nil {
		return 0, err
	}

	count := 0
	sprintNum := e.sim.SprintNumber

	// Write resolved incidents
	for _, incident := range e.sim.ResolvedIncidents {
		row := formatIncidentRow(incident, sprintNum)
		if err := writer.Write(row); err != nil {
			return count, err
		}
		count++
	}

	// Write open incidents
	for _, incident := range e.sim.OpenIncidents {
		row := formatIncidentRow(incident, sprintNum)
		if err := writer.Write(row); err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

// writeMetrics writes metrics.csv
func (e *Exporter) writeMetrics(outputDir string) error {
	path := filepath.Join(outputDir, "metrics.csv")
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write(MetricsHeader); err != nil {
		return err
	}

	totalIncidents := len(e.sim.ResolvedIncidents) + len(e.sim.OpenIncidents)

	// Use tracker data if available, otherwise calculate from sim
	var leadTimeAvg, deployFreq, mttrAvg, changeFailRate float64
	if e.tracker != nil && e.tracker.DORA != nil {
		leadTimeAvg = e.tracker.DORA.LeadTimeAvgDays()
		deployFreq = e.tracker.DORA.DeployFrequency
		mttrAvg = e.tracker.DORA.MTTRAvgDays()
		changeFailRate = e.tracker.DORA.ChangeFailRatePct()
	} else {
		// Fallback: calculate from simulation state
		var leadTimeSum float64
		for _, t := range e.sim.CompletedTickets {
			leadTimeSum += float64(t.CompletedTick - t.StartedTick)
		}
		if len(e.sim.CompletedTickets) > 0 {
			leadTimeAvg = leadTimeSum / float64(len(e.sim.CompletedTickets))
			changeFailRate = float64(totalIncidents) / float64(len(e.sim.CompletedTickets)) * 100
		}
	}

	row := []string{
		e.sim.SizingPolicy.String(),
		fmt.Sprintf("%.2f", leadTimeAvg),
		"0.00", // stddev - not tracked
		fmt.Sprintf("%.2f", deployFreq),
		fmt.Sprintf("%.2f", mttrAvg),
		fmt.Sprintf("%.2f", changeFailRate),
		fmt.Sprintf("%d", len(e.sim.CompletedTickets)),
		fmt.Sprintf("%d", totalIncidents),
	}

	return writer.Write(row)
}

// writeComparison writes comparison.csv
func (e *Exporter) writeComparison(outputDir string) error {
	if e.comparison == nil {
		return nil
	}

	path := filepath.Join(outputDir, "comparison.csv")
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write(ComparisonHeader); err != nil {
		return err
	}

	c := e.comparison
	metricsA := c.ResultsA.FinalMetrics
	metricsB := c.ResultsB.FinalMetrics

	// Helper to format comparison row
	writeRow := func(metric string, valA, valB float64, winner model.SizingPolicy) error {
		diff := valB - valA
		diffPct := 0.0
		if valA != 0 {
			diffPct = (diff / valA) * 100
		}
		row := []string{
			fmt.Sprintf("%d", c.Seed),
			"3", // sprints run (standard comparison)
			metric,
			fmt.Sprintf("%.2f", valA),
			fmt.Sprintf("%.2f", valB),
			winner.String(),
			fmt.Sprintf("%.2f", diff),
			fmt.Sprintf("%.2f", diffPct),
		}
		return writer.Write(row)
	}

	// Write row for each metric
	if err := writeRow("lead_time", metricsA.LeadTimeAvgDays(), metricsB.LeadTimeAvgDays(), c.LeadTimeWinner); err != nil {
		return err
	}
	if err := writeRow("deploy_frequency", metricsA.DeployFrequency, metricsB.DeployFrequency, c.DeployFreqWinner); err != nil {
		return err
	}
	if err := writeRow("mttr", metricsA.MTTRAvgDays(), metricsB.MTTRAvgDays(), c.MTTRWinner); err != nil {
		return err
	}
	if err := writeRow("change_fail_rate", metricsA.ChangeFailRatePct(), metricsB.ChangeFailRatePct(), c.CFRWinner); err != nil {
		return err
	}

	return nil
}
