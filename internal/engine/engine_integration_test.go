package engine_test

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// Integration test: full simulation run completes without panic
// Per Khorikov: integration test for happy path, unit tests for edge cases
func TestEngine_FullSimulationRun(t *testing.T) {
	policies := []model.SizingPolicy{
		model.PolicyNone,
		model.PolicyDORAStrict,
		model.PolicyTameFlowCognitive,
		model.PolicyHybrid,
	}

	for _, policy := range policies {
		t.Run(policy.String(), func(t *testing.T) {
			sim := model.NewSimulation("test-full-run", policy, 12345)

			eng := engine.NewEngine(sim.Seed)
			eng, _ = eng.EmitCreated(sim.ID, sim.CurrentTick, events.SimConfig{
				TeamSize:     2,
				SprintLength: sim.SprintLength,
				Seed:         sim.Seed,
				Policy:       policy,
			})

			// Add developers
			eng, _ = eng.AddDeveloper("dev-1", "Alice", 1.0)
			eng, _ = eng.AddDeveloper("dev-2", "Bob", 0.8)

			// Add tickets
			eng, _ = eng.AddTicket(model.NewTicket("TKT-001", "Small task", 2, model.HighUnderstanding))
			eng, _ = eng.AddTicket(model.NewTicket("TKT-002", "Medium task", 5, model.MediumUnderstanding))
			eng, _ = eng.AddTicket(model.NewTicket("TKT-003", "Large task", 8, model.LowUnderstanding))

			// Assign tickets
			var err error
			eng, err = eng.AssignTicket("TKT-001", "dev-1")
			if err != nil {
				t.Fatalf("Failed to assign TKT-001: %v", err)
			}
			eng, err = eng.AssignTicket("TKT-002", "dev-2")
			if err != nil {
				t.Fatalf("Failed to assign TKT-002: %v", err)
			}

			// Run a sprint
			var evts []model.Event
			eng, evts, _ = eng.RunSprint()

			// Should have produced some events
			if len(evts) == 0 {
				t.Error("Expected some events from sprint, got none")
			}

			// Simulation state should have progressed (read from projection, not sim)
			state := eng.Sim()
			if state.CurrentTick == 0 {
				t.Error("CurrentTick should have advanced")
			}

			// At least one ticket should have made progress
			hasProgress := false
			for _, ticket := range state.ActiveTickets {
				if ticket.Phase > model.PhaseBacklog {
					hasProgress = true
					break
				}
			}
			if len(state.CompletedTickets) > 0 {
				hasProgress = true
			}

			if !hasProgress {
				t.Error("Expected at least one ticket to have progressed")
			}
		})
	}
}

// Integration test: decomposition with Either return type
// Tests all three cases: not found, policy forbids, and success
func TestEngine_TryDecompose_Either(t *testing.T) {
	tests := []struct {
		name       string
		policy     model.SizingPolicy
		ticketID   string
		ticketSize float64
		wantLeft   bool
		wantReason string
	}{
		{
			name:       "ticket not found returns Left",
			policy:     model.PolicyDORAStrict,
			ticketID:   "NONEXISTENT",
			ticketSize: 10,
			wantLeft:   true,
			wantReason: "ticket not found",
		},
		{
			name:       "policy forbids decomposition returns Left",
			policy:     model.PolicyNone, // No decomposition policy
			ticketID:   "TKT-001",
			ticketSize: 10,
			wantLeft:   true,
			wantReason: "policy forbids decomposition",
		},
		{
			name:       "successful decomposition returns Right",
			policy:     model.PolicyDORAStrict, // Decomposes large tickets
			ticketID:   "TKT-001",
			ticketSize: 10, // Large enough to trigger decomposition
			wantLeft:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sim := model.NewSimulation("test-decompose", tt.policy, 12345)

			eng := engine.NewEngine(sim.Seed)
			eng, _ = eng.EmitCreated(sim.ID, sim.CurrentTick, events.SimConfig{
				TeamSize:     0,
				SprintLength: sim.SprintLength,
				Seed:         sim.Seed,
				Policy:       tt.policy,
			})

			// Only add ticket if we're testing with a real ticket
			if tt.ticketID == "TKT-001" {
				eng, _ = eng.AddTicket(model.NewTicket("TKT-001", "Large feature", tt.ticketSize, model.MediumUnderstanding))
			}

			eng, result, _ := eng.TryDecompose(tt.ticketID)

			if tt.wantLeft {
				notDecomp, ok := result.GetLeft()
				if !ok {
					t.Errorf("Expected Left (NotDecomposable), got Right")
					return
				}
				if notDecomp.Reason != tt.wantReason {
					t.Errorf("Reason = %q, want %q", notDecomp.Reason, tt.wantReason)
				}
			} else {
				children, ok := result.Get()
				if !ok {
					t.Errorf("Expected Right (children), got Left")
					return
				}
				if len(children) < 2 {
					t.Errorf("Expected 2+ children, got %d", len(children))
				}

				// Verify children are in backlog
				state := eng.Sim()
				childIDs := make(map[string]bool)
				for _, child := range children {
					childIDs[child.ID] = true
				}
				childrenInBacklog := 0
				for _, ticket := range state.Backlog {
					if childIDs[ticket.ID] {
						childrenInBacklog++
					}
				}
				if childrenInBacklog != len(children) {
					t.Errorf("Expected %d children in backlog, found %d", len(children), childrenInBacklog)
				}
			}
		})
	}
}

// Integration test: WIP tracking during sprint
// Verifies SprintWIPUpdated events are emitted with correct WIP values
func TestEngine_WIPTracking(t *testing.T) {
	sim := model.NewSimulation("wip-test", model.PolicyNone, 12345)

	// Use event store to verify WIP tracking via events
	store := events.NewMemoryStore()
	eng := engine.NewEngineWithStore(sim.Seed, store)
	eng, _ = eng.EmitCreated(sim.ID, sim.CurrentTick, events.SimConfig{
		TeamSize:     2,
		SprintLength: sim.SprintLength,
		Seed:         sim.Seed,
		Policy:       model.PolicyNone,
	})

	eng, _ = eng.AddDeveloper("dev-1", "Alice", 1.0)
	eng, _ = eng.AddDeveloper("dev-2", "Bob", 1.0)

	// Add tickets that will create WIP
	eng, _ = eng.AddTicket(model.NewTicket("TKT-001", "Task 1", 3, model.HighUnderstanding))
	eng, _ = eng.AddTicket(model.NewTicket("TKT-002", "Task 2", 5, model.HighUnderstanding))

	// Assign both tickets - creates WIP of 2
	eng, _ = eng.AssignTicket("TKT-001", "dev-1")
	eng, _ = eng.AssignTicket("TKT-002", "dev-2")

	// Run sprint
	eng, _, _ = eng.RunSprint()

	// Verify WIP tracking via SprintWIPUpdated events
	evts := store.Replay("wip-test")
	wipEvents := 0
	maxWIPSeen := 0
	for _, evt := range evts {
		if wipEvt, ok := evt.(events.SprintWIPUpdated); ok {
			wipEvents++
			if wipEvt.CurrentWIP > maxWIPSeen {
				maxWIPSeen = wipEvt.CurrentWIP
			}
		}
	}

	// Should have emitted WIP events (one per tick during sprint)
	if wipEvents == 0 {
		t.Error("Expected SprintWIPUpdated events during sprint")
	}

	// Should have seen WIP of at least 1 (tickets were assigned)
	if maxWIPSeen < 1 {
		t.Errorf("Expected MaxWIP >= 1, got %d", maxWIPSeen)
	}
}

// Integration test: reproducibility with same seed
func TestEngine_Reproducibility(t *testing.T) {
	seed := int64(42)

	runSimulation := func() model.Simulation {
		sim := model.NewSimulation("repro-test", model.PolicyDORAStrict, seed)

		eng := engine.NewEngine(sim.Seed)
		eng, _ = eng.EmitCreated(sim.ID, sim.CurrentTick, events.SimConfig{
			TeamSize:     1,
			SprintLength: sim.SprintLength,
			Seed:         sim.Seed,
			Policy:       model.PolicyDORAStrict,
		})
		eng, _ = eng.AddDeveloper("dev-1", "Alice", 1.0)
		eng, _ = eng.AddTicket(model.NewTicket("TKT-001", "Task", 5, model.MediumUnderstanding))
		eng, _ = eng.AssignTicket("TKT-001", "dev-1")
		eng, _, _ = eng.RunSprint()

		return eng.Sim() // Return state from engine
	}

	sim1 := runSimulation()
	sim2 := runSimulation()

	// Same seed should produce same results
	if sim1.CurrentTick != sim2.CurrentTick {
		t.Errorf("CurrentTick: %d != %d", sim1.CurrentTick, sim2.CurrentTick)
	}

	if len(sim1.CompletedTickets) != len(sim2.CompletedTickets) {
		t.Errorf("CompletedTickets: %d != %d", len(sim1.CompletedTickets), len(sim2.CompletedTickets))
	}
}

// Integration test: sprint ends exactly on boundary tick
// Per Khorikov: edge case test for off-by-one boundary conditions
func TestEngine_SprintEndsExactlyOnBoundary(t *testing.T) {
	sim := model.NewSimulation("boundary-test", model.PolicyNone, 12345)

	eng := engine.NewEngine(sim.Seed)
	eng, _ = eng.EmitCreated(sim.ID, sim.CurrentTick, events.SimConfig{
		TeamSize:     1,
		SprintLength: 10, // 10 ticks
		Seed:         sim.Seed,
		Policy:       model.PolicyNone,
	})
	eng, _ = eng.AddDeveloper("dev-1", "Alice", 1.0)
	eng, _ = eng.StartSprint()

	// Verify sprint started
	state := eng.Sim()
	sprint, hasActiveSprint := state.CurrentSprintOption.Get()
	if !hasActiveSprint {
		t.Fatal("Sprint should be active after StartSprint")
	}
	if sprint.EndDay != 10 {
		t.Errorf("Sprint.EndDay = %d, want 10", sprint.EndDay)
	}

	// Tick exactly to sprint end (10 ticks)
	for i := 0; i < 10; i++ {
		eng, _, _ = eng.Tick()
	}

	state = eng.Sim()

	// Sprint should have ended exactly at tick 10
	_, hasActiveSprint = state.CurrentSprintOption.Get()
	if hasActiveSprint {
		t.Error("Sprint should have ended at tick 10")
	}

	// CurrentTick should be exactly 10
	if state.CurrentTick != 10 {
		t.Errorf("CurrentTick = %d, want 10", state.CurrentTick)
	}

	// One more tick should not cause panic or start new sprint
	eng, _, _ = eng.Tick()
	state = eng.Sim()
	if state.CurrentTick != 11 {
		t.Errorf("CurrentTick after extra tick = %d, want 11", state.CurrentTick)
	}
}

// TestEngine_Tick_ReturnsNewEngine verifies immutable Engine semantics.
// Per FP Guide §7: operations return new values, original unchanged.
// This test expects the new signature: Tick() (Engine, []model.Event)
func TestEngine_Tick_ReturnsNewEngine(t *testing.T) {
	sim := model.NewSimulation("test-immutable", model.PolicyNone, 42)
	eng := engine.NewEngine(sim.Seed)
	eng, _ = eng.EmitCreated(sim.ID, 0, events.SimConfig{
		TeamSize:     1,
		SprintLength: 10,
		Seed:         sim.Seed,
		Policy:       model.PolicyNone,
	})
	eng, _ = eng.AddDeveloper("dev-1", "Alice", 1.0)
	eng, _ = eng.AddTicket(model.NewTicket("TKT-001", "Task", 3, model.HighUnderstanding))
	eng, _ = eng.AssignTicket("TKT-001", "dev-1")
	eng, _ = eng.StartSprint()

	// Capture original state
	originalTick := eng.Sim().CurrentTick

	// Call Tick - expects new signature returning (Engine, []model.Event, error)
	newEng, _, _ := eng.Tick()

	// Verify original engine unchanged (immutability)
	if eng.Sim().CurrentTick != originalTick {
		t.Errorf("original engine was mutated: tick=%d, want=%d",
			eng.Sim().CurrentTick, originalTick)
	}

	// Verify new engine has advanced tick
	if newEng.Sim().CurrentTick != originalTick+1 {
		t.Errorf("new engine tick=%d, want=%d",
			newEng.Sim().CurrentTick, originalTick+1)
	}
}
