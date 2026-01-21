package tui

import (
	"fmt"
	"strings"
	"testing"
)

// TestCaptureAllCheckpoints runs the simulation with seed 42 and captures
// exact values for tutorial checkpoints. Skipped by default.
// Run manually with: go test -v -run TestCaptureAllCheckpoints ./internal/tui/
func TestCaptureAllCheckpoints(t *testing.T) {
	t.Skip("Documentation utility - run manually when updating tutorial checkpoints")
	app := NewAppWithSeed(42)
	eng, _ := app.mode.GetLeft()

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("CHECKPOINT 1: Initial State")
	fmt.Println(strings.Repeat("=", 60))
	printState(app, "Initial")

	// Checkpoint 2: Assign 3 tickets
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("CHECKPOINT 2: After Assignment")
	fmt.Println(strings.Repeat("=", 60))

	// Assign first 3 tickets to developers
	sim := eng.Engine.Sim()
	for i, dev := range sim.Developers {
		if i < len(sim.Backlog) {
			ticket := sim.Backlog[0] // Always take first (it gets removed)
			eng.Engine.AssignTicket(ticket.ID, dev.ID)
			sim = eng.Engine.Sim() // Refresh after mutation
		}
	}
	printState(app, "After Assignment")

	// Checkpoint 3: Start sprint and run to Day 5
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("CHECKPOINT 3: Mid-Sprint (Day 5)")
	fmt.Println(strings.Repeat("=", 60))

	eng.Engine.StartSprint()
	sim = eng.Engine.Sim()
	for sim.CurrentTick < 5 {
		eng.Engine.Tick()
		sim = eng.Engine.Sim()
		eng.Tracker = eng.Tracker.Updated(&sim)
	}
	printStateWithEngine(eng, "Day 5")
	printFeverWithEngine(eng)
	printActiveWorkWithEngine(eng)

	// Checkpoint 5: Run to sprint end (Day 10)
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("CHECKPOINT 5: Sprint Complete (Day 10)")
	fmt.Println(strings.Repeat("=", 60))

	sim = eng.Engine.Sim()
	for sim.CurrentTick < 10 {
		eng.Engine.Tick()
		sim = eng.Engine.Sim()
		eng.Tracker = eng.Tracker.Updated(&sim)
	}
	printStateWithEngine(eng, "Day 10")
	printCompletedWithEngine(eng)
	printMetricsWithEngine(eng)
}

func printState(app *App, label string) {
	eng, _ := app.mode.GetLeft()
	printStateWithEngine(eng, label)
}

func printStateWithEngine(eng EngineMode, label string) {
	sim := eng.Engine.Sim()
	fmt.Printf("\n[%s]\n", label)
	fmt.Printf("  Day: %d\n", sim.CurrentTick)
	fmt.Printf("  Backlog: %d tickets\n", len(sim.Backlog))
	fmt.Printf("  Completed: %d tickets\n", len(sim.CompletedTickets))
	fmt.Printf("  Policy: %s\n", sim.SizingPolicy)

	fmt.Println("  Team:")
	for _, dev := range sim.Developers {
		status := "idle"
		ticket := ""
		if !dev.IsIdle() {
			status = "busy"
			ticket = fmt.Sprintf(" → %s", dev.CurrentTicket)
		}
		fmt.Printf("    %s (v:%.1f) [%s]%s\n", dev.Name, dev.Velocity, status, ticket)
	}
}

func printFeverWithEngine(eng EngineMode) {
	sim := eng.Engine.Sim()
	sprint, ok := sim.CurrentSprintOption.Get()
	if !ok {
		return
	}
	pctUsed := 0.0
	if sprint.BufferDays > 0 {
		pctUsed = (sprint.BufferConsumed / sprint.BufferDays) * 100
	}
	fmt.Printf("\n  Fever Chart:\n")
	fmt.Printf("    Buffer: %.1f / %.1f days (%.0f%% used)\n",
		sprint.BufferConsumed, sprint.BufferDays, pctUsed)
	fmt.Printf("    Status: %s\n", sprint.FeverStatus)
}

func printActiveWorkWithEngine(eng EngineMode) {
	sim := eng.Engine.Sim()
	fmt.Println("\n  Active Work:")
	for _, dev := range sim.Developers {
		if !dev.IsIdle() {
			// Find the ticket
			for _, t := range sim.ActiveTickets {
				if t.ID == dev.CurrentTicket {
					progress := 0.0
					if t.EstimatedDays > 0 {
						progress = ((t.EstimatedDays - t.RemainingEffort) / t.EstimatedDays) * 100
					}
					fmt.Printf("    %s → %s [%.0f%%] Phase: %s\n",
						dev.Name, t.ID, progress, t.Phase)
					break
				}
			}
		} else {
			fmt.Printf("    %s [idle]\n", dev.Name)
		}
	}
}

func printCompletedWithEngine(eng EngineMode) {
	sim := eng.Engine.Sim()
	fmt.Println("\n  Completed Tickets:")
	for _, t := range sim.CompletedTickets {
		ratio := 0.0
		if t.EstimatedDays > 0 {
			ratio = t.ActualDays / t.EstimatedDays
		}
		fmt.Printf("    %s: Est %.1fd, Actual %.1fd, Ratio %.2f, %s\n",
			t.ID, t.EstimatedDays, t.ActualDays, ratio, t.UnderstandingLevel)
	}
}

func printMetricsWithEngine(eng EngineMode) {
	sim := eng.Engine.Sim()
	result := eng.Tracker.GetResult(sim.SizingPolicy, &sim)
	m := result.FinalMetrics
	fmt.Println("\n  DORA Metrics:")
	fmt.Printf("    Lead Time: %.2f days\n", m.LeadTimeAvgDays())
	fmt.Printf("    Deploy Freq: %.2f/day\n", m.DeployFrequency)
	fmt.Printf("    MTTR: %.2f days\n", m.MTTRAvgDays())
	fmt.Printf("    Change Fail Rate: %.1f%%\n", m.ChangeFailRatePct())
}
