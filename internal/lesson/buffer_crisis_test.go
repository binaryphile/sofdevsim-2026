package lesson

import (
	"fmt"
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

func TestBufferCrisisLesson_FindWorkingSeed(t *testing.T) {
	// Find a seed that produces a crisis-recovery cycle by monitoring state
	for seed := int64(1); seed <= 500; seed++ {
		store := events.NewMemoryStore()
		eng := engine.NewEngineWithStore(seed, store)

		simID := "test-seed"
		config := events.SimConfig{
			TeamSize:     1,
			SprintLength: 14,
			Seed:         seed,
			Policy:       model.PolicyDORAStrict,
		}

		var err error
		eng, err = eng.EmitCreated(simID, 0, config)
		if err != nil {
			continue
		}

		// Multiple developers to increase active tickets
		eng, _ = eng.AddDeveloper("dev-1", "Alex", 1.0)
		eng, _ = eng.AddDeveloper("dev-2", "Blake", 1.0)
		eng, _ = eng.AddDeveloper("dev-3", "Casey", 1.0)

		// Long tickets that won't complete in one sprint → guaranteed buffer consumption
		// Recovery happens when a new sprint starts (fresh buffer) after completing some work
		tickets := []model.Ticket{
			model.NewTicket("TKT-001", "Task 1", 20, model.LowUnderstanding),
			model.NewTicket("TKT-002", "Task 2", 20, model.LowUnderstanding),
			model.NewTicket("TKT-003", "Task 3", 20, model.LowUnderstanding),
		}
		for _, ticket := range tickets {
			eng, _ = eng.AddTicket(ticket)
		}

		// Track zone by monitoring state
		lastZone := model.FeverGreen
		var crisisTick, recoveryTick int
		sprintFound := false

		// Run multiple sprints to allow recovery
		for sprintNum := 0; sprintNum < 3; sprintNum++ {
			eng, _ = eng.StartSprint()

			// Assign all backlog tickets to idle developers
			for {
				// Re-fetch state each iteration since eng changes
				sim := eng.Sim()
				if len(sim.Backlog) == 0 {
					break
				}
				ticketID := sim.Backlog[0].ID

				// Find an idle developer
				assigned := false
				for _, dev := range sim.Developers {
					if dev.IsIdle() {
						eng, _ = eng.AssignTicket(ticketID, dev.ID)
						assigned = true
						break
					}
				}
				if !assigned {
					break // No idle developers
				}
			}

			// Run through the sprint
			sprint, _ := eng.Sim().CurrentSprintOption.Get()
			for eng.Sim().CurrentTick < sprint.EndDay {
				eng, _, _ = eng.Tick()

				if sp, ok := eng.Sim().CurrentSprintOption.Get(); ok {
					sprintFound = true
					// Log buffer status for seed 1
					if seed == 1 && eng.Sim().CurrentTick%5 == 0 {
						t.Logf("  Sprint %d Tick %d: buffer=%.2f/%.2f (%.0f%%), status=%s, active=%d, completed=%d",
							sprintNum+1, eng.Sim().CurrentTick, sp.BufferConsumed, sp.BufferDays,
							sp.BufferPctUsed()*100, sp.FeverStatus,
							len(eng.Sim().ActiveTickets), len(eng.Sim().CompletedTickets))
					}
					if sp.FeverStatus != lastZone {
						if sp.FeverStatus == model.FeverRed && crisisTick == 0 {
							crisisTick = eng.Sim().CurrentTick
						}
						if sp.FeverStatus == model.FeverGreen && crisisTick > 0 && recoveryTick == 0 {
							recoveryTick = eng.Sim().CurrentTick
						}
						lastZone = sp.FeverStatus
					}
				}

				// Stop if we found recovery
				if recoveryTick > 0 {
					break
				}
			}

			if recoveryTick > 0 {
				break
			}
		}

		if seed == 1 {
			if !sprintFound {
				t.Log("Seed 1: Sprint never found!")
			} else {
				t.Logf("Seed 1: sprint ended, last zone=%s, crisis=%d, recovery=%d, BufferPct=%.2f",
					lastZone, crisisTick, recoveryTick, eng.Sim().BufferPct)
			}
		}

		if crisisTick > 0 && recoveryTick > 0 {
			t.Logf("✓ Seed %d produces crisis at tick %d, recovery at tick %d",
				seed, crisisTick, recoveryTick)
		} else if crisisTick > 0 {
			// Log seeds that get to crisis but don't recover
			if seed <= 10 { // Only log first few for debugging
				t.Logf("Seed %d: crisis at %d but no recovery", seed, crisisTick)
			}
		} else if seed <= 10 { // Only log first few for debugging
			// Log what happened
			if sprint, ok := eng.Sim().CurrentSprintOption.Get(); ok {
				t.Logf("Seed %d: no crisis, max zone=%s, buffer consumed=%.2f/%.2f",
					seed, lastZone, sprint.BufferConsumed, sprint.BufferDays)
			}
		}
	}
}

func TestBufferCrisisLesson_ProducesCrisisRecovery(t *testing.T) {
	// Use the same seed and configuration as the lesson
	store := events.NewMemoryStore()
	eng := engine.NewEngineWithStore(lessonSeed, store)

	simID := "test-lesson"
	config := events.SimConfig{
		TeamSize:     3,
		SprintLength: 14,
		Seed:         lessonSeed,
		Policy:       model.PolicyDORAStrict,
	}

	var err error
	eng, err = eng.EmitCreated(simID, 0, config)
	if err != nil {
		t.Fatalf("EmitCreated: %v", err)
	}

	// Multiple developers (same as lesson)
	eng, _ = eng.AddDeveloper("dev-1", "Alex", 1.0)
	eng, _ = eng.AddDeveloper("dev-2", "Blake", 1.0)
	eng, _ = eng.AddDeveloper("dev-3", "Casey", 1.0)

	// Long tickets that won't complete in one sprint (same as lesson)
	tickets := []model.Ticket{
		model.NewTicket("TKT-001", "Major refactoring", 20, model.LowUnderstanding),
		model.NewTicket("TKT-002", "New feature", 20, model.LowUnderstanding),
		model.NewTicket("TKT-003", "Technical debt", 20, model.LowUnderstanding),
	}
	for _, ticket := range tickets {
		eng, _ = eng.AddTicket(ticket)
	}

	// Track zone by monitoring state (same as lesson does)
	lastZone := model.FeverGreen
	var crisisTick, recoveryTick int
	var zoneChanges []string

	// Run multiple sprints (same as lesson)
	for sprintNum := 0; sprintNum < 3 && recoveryTick == 0; sprintNum++ {
		eng, _ = eng.StartSprint()

		// Assign all backlog tickets to idle developers
		for {
			sim := eng.Sim()
			if len(sim.Backlog) == 0 {
				break
			}
			ticketID := sim.Backlog[0].ID

			assigned := false
			for _, dev := range sim.Developers {
				if dev.IsIdle() {
					eng, _ = eng.AssignTicket(ticketID, dev.ID)
					assigned = true
					break
				}
			}
			if !assigned {
				break
			}
		}

		// Run through the sprint
		sprint, _ := eng.Sim().CurrentSprintOption.Get()
		for eng.Sim().CurrentTick < sprint.EndDay && recoveryTick == 0 {
			eng, _, err = eng.Tick()
			if err != nil {
				t.Fatalf("Tick: %v", err)
			}

			if sp, ok := eng.Sim().CurrentSprintOption.Get(); ok {
				if sp.FeverStatus != lastZone {
					zoneChanges = append(zoneChanges, fmt.Sprintf("Tick %d: %s -> %s",
						eng.Sim().CurrentTick, lastZone, sp.FeverStatus))

					if sp.FeverStatus == model.FeverRed && crisisTick == 0 {
						crisisTick = eng.Sim().CurrentTick
					}
					if sp.FeverStatus == model.FeverGreen && crisisTick > 0 && recoveryTick == 0 {
						recoveryTick = eng.Sim().CurrentTick
					}
					lastZone = sp.FeverStatus
				}
			}
		}
	}

	t.Log("Zone changes observed:")
	for _, zc := range zoneChanges {
		t.Log("  " + zc)
	}

	if crisisTick == 0 {
		t.Fatal("Expected a crisis (FeverRed), but none occurred")
	}

	if recoveryTick == 0 {
		t.Fatal("Expected a recovery (FeverGreen after crisis), but none occurred")
	}

	t.Logf("Crisis at tick %d, recovery at tick %d", crisisTick, recoveryTick)

	if recoveryTick <= crisisTick {
		t.Error("Recovery tick should be after crisis tick")
	}
}
