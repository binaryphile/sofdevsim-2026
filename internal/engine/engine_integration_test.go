package engine_test

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/engine"
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
			sim := model.NewSimulation(policy, 12345)

			// Add developers
			sim.AddDeveloper(model.NewDeveloper("dev-1", "Alice", 1.0))
			sim.AddDeveloper(model.NewDeveloper("dev-2", "Bob", 0.8))

			// Add tickets
			sim.AddTicket(model.NewTicket("TKT-001", "Small task", 2, model.HighUnderstanding))
			sim.AddTicket(model.NewTicket("TKT-002", "Medium task", 5, model.MediumUnderstanding))
			sim.AddTicket(model.NewTicket("TKT-003", "Large task", 8, model.LowUnderstanding))

			eng := engine.NewEngine(sim)

			// Assign tickets
			if err := eng.AssignTicket("TKT-001", "dev-1"); err != nil {
				t.Fatalf("Failed to assign TKT-001: %v", err)
			}
			if err := eng.AssignTicket("TKT-002", "dev-2"); err != nil {
				t.Fatalf("Failed to assign TKT-002: %v", err)
			}

			// Run a sprint
			events := eng.RunSprint()

			// Should have produced some events
			if len(events) == 0 {
				t.Error("Expected some events from sprint, got none")
			}

			// Simulation state should have progressed
			if sim.CurrentTick == 0 {
				t.Error("CurrentTick should have advanced")
			}

			// At least one ticket should have made progress
			hasProgress := false
			for _, ticket := range sim.ActiveTickets {
				if ticket.Phase > model.PhaseBacklog {
					hasProgress = true
					break
				}
			}
			if len(sim.CompletedTickets) > 0 {
				hasProgress = true
			}

			if !hasProgress {
				t.Error("Expected at least one ticket to have progressed")
			}
		})
	}
}

// Integration test: decomposition works end-to-end
func TestEngine_DecompositionIntegration(t *testing.T) {
	sim := model.NewSimulation(model.PolicyDORAStrict, 12345)

	// Large ticket that should be decomposed under DORA policy
	sim.AddTicket(model.NewTicket("TKT-001", "Large feature", 10, model.MediumUnderstanding))

	eng := engine.NewEngine(sim)
	children, decomposed := eng.TryDecompose("TKT-001")

	if !decomposed {
		t.Error("Expected ticket to be decomposed under DORA policy")
	}

	if len(children) < 2 {
		t.Errorf("Expected 2+ children, got %d", len(children))
	}

	// Original should be removed from backlog
	for _, ticket := range sim.Backlog {
		if ticket.ID == "TKT-001" {
			t.Error("Original ticket should be removed from backlog after decomposition")
		}
	}

	// Children should be in backlog
	childrenInBacklog := 0
	for _, ticket := range sim.Backlog {
		if ticket.ParentID == "TKT-001" {
			childrenInBacklog++
		}
	}

	if childrenInBacklog != len(children) {
		t.Errorf("Expected %d children in backlog, found %d", len(children), childrenInBacklog)
	}
}

// Integration test: reproducibility with same seed
func TestEngine_Reproducibility(t *testing.T) {
	seed := int64(42)

	runSimulation := func() *model.Simulation {
		sim := model.NewSimulation(model.PolicyDORAStrict, seed)
		sim.AddDeveloper(model.NewDeveloper("dev-1", "Alice", 1.0))
		sim.AddTicket(model.NewTicket("TKT-001", "Task", 5, model.MediumUnderstanding))

		eng := engine.NewEngine(sim)
		eng.AssignTicket("TKT-001", "dev-1")
		eng.RunSprint()

		return sim
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
