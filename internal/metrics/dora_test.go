package metrics_test

import (
	"testing"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// Test DORA metrics calculation from simulation state
func TestDORAMetrics_Update(t *testing.T) {
	sim := model.NewSimulation(model.PolicyNone, 12345)
	sim.CurrentTick = 10

	// Add completed tickets with known lead times (using ticks and time.Duration)
	now := time.Now()
	sim.CompletedTickets = []model.Ticket{
		{ID: "TKT-001", StartedAt: now.Add(-48 * time.Hour), CompletedAt: now, StartedTick: 1, CompletedTick: 8},  // 2 days lead time
		{ID: "TKT-002", StartedAt: now.Add(-72 * time.Hour), CompletedAt: now.Add(-24 * time.Hour), StartedTick: 2, CompletedTick: 9}, // 2 days
		{ID: "TKT-003", StartedAt: now.Add(-120 * time.Hour), CompletedAt: now.Add(-24 * time.Hour), StartedTick: 1, CompletedTick: 10}, // 4 days
	}

	// Add resolved incidents
	resolvedAt := now
	sim.ResolvedIncidents = []model.Incident{
		{ID: "INC-001", CreatedAt: now.Add(-24 * time.Hour), ResolvedAt: &resolvedAt}, // 1 day
		{ID: "INC-002", CreatedAt: now.Add(-12 * time.Hour), ResolvedAt: &resolvedAt}, // 0.5 days
	}

	dora := metrics.NewDORAMetrics()
	dora.Update(sim)

	// Lead time should be average of 2, 2, 4 = ~2.67 days
	if dora.LeadTimeAvgDays() < 2.5 || dora.LeadTimeAvgDays() > 2.8 {
		t.Errorf("LeadTimeAvgDays() = %.2f, want ~2.67", dora.LeadTimeAvgDays())
	}

	// Deploy frequency: 3 deploys in ticks 3-10 (all within last 7 ticks) = 3/7 = 0.43/day
	if dora.DeployFrequency < 0.3 || dora.DeployFrequency > 0.6 {
		t.Errorf("DeployFrequency = %.2f, want ~0.43", dora.DeployFrequency)
	}

	// MTTR: average of 1 and 0.5 = 0.75 days
	if dora.MTTRAvgDays() < 0.6 || dora.MTTRAvgDays() > 0.9 {
		t.Errorf("MTTRAvgDays() = %.2f, want ~0.75", dora.MTTRAvgDays())
	}

	// Change fail rate: 2 incidents / 3 deploys = 66.7%
	if dora.ChangeFailRatePct() < 60 || dora.ChangeFailRatePct() > 70 {
		t.Errorf("ChangeFailRatePct() = %.1f%%, want ~66.7%%", dora.ChangeFailRatePct())
	}
}

// Test that history is appended for sparklines
func TestDORAMetrics_History(t *testing.T) {
	sim := model.NewSimulation(model.PolicyNone, 12345)
	dora := metrics.NewDORAMetrics()

	// Update multiple times
	for tick := 1; tick <= 5; tick++ {
		sim.CurrentTick = tick
		dora.Update(sim)
	}

	if len(dora.History) != 5 {
		t.Errorf("History has %d snapshots, want 5", len(dora.History))
	}

	// Each snapshot should have increasing day
	for i, snap := range dora.History {
		if snap.Day != i+1 {
			t.Errorf("History[%d].Day = %d, want %d", i, snap.Day, i+1)
		}
	}
}
