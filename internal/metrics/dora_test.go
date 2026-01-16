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

	// Add completed tickets with known lead times (using ticks - 1 tick = 1 day)
	sim.CompletedTickets = []model.Ticket{
		{ID: "TKT-001", StartedTick: 1, CompletedTick: 3},  // 2 days lead time
		{ID: "TKT-002", StartedTick: 2, CompletedTick: 4},  // 2 days lead time
		{ID: "TKT-003", StartedTick: 1, CompletedTick: 5},  // 4 days lead time
	}

	// Add resolved incidents (MTTR still uses wall-clock time for now)
	now := time.Now()
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

// Test that lead time uses tick-based calculation, not wall-clock time
// This reproduces TKT-003: when simulation runs fast, time.Now() values are
// nearly identical but tick values differ
func TestDORAMetrics_LeadTime_UsesTicksNotWallClock(t *testing.T) {
	sim := model.NewSimulation(model.PolicyNone, 12345)
	sim.CurrentTick = 20

	// Simulate fast execution: StartedAt and CompletedAt are the same instant
	// but StartedTick and CompletedTick differ by 10 days
	now := time.Now()
	sim.CompletedTickets = []model.Ticket{
		{
			ID:            "TKT-001",
			StartedAt:     now, // Same wall-clock time!
			CompletedAt:   now, // Same wall-clock time!
			StartedTick:   5,   // But tick difference = 10 days
			CompletedTick: 15,
		},
	}

	dora := metrics.NewDORAMetrics()
	dora.Update(sim)

	// Lead time should be 10 days (based on ticks), not 0 (based on wall clock)
	if dora.LeadTimeAvgDays() < 9.5 || dora.LeadTimeAvgDays() > 10.5 {
		t.Errorf("LeadTimeAvgDays() = %.2f, want 10.0 (tick-based)", dora.LeadTimeAvgDays())
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
