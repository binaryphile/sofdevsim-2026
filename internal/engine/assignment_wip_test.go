package engine_test

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// UC38 (#15443): cap-check gate in assignFromQueues. Tests below drive
// through the public Engine surface (EmitCreated + AddDeveloper + Tick)
// per Khorikov classical posture — assertions are output-based on
// observable state (PhaseWIPCount, PhaseQueues), not on internal calls.

// makeCappedEngine constructs an engine with N devs available for Implement
// work and an initial sim carrying the given PhaseWIPConfig.
func makeCappedEngine(t *testing.T, seed int64, cfg map[model.WorkflowPhase]int, devCount int) engine.Engine {
	t.Helper()
	eng := engine.NewEngine(seed)
	var err error
	eng, err = eng.EmitCreated("wip-test", 0, events.SimConfig{
		TeamSize:       devCount,
		SprintLength:   10,
		Seed:           seed,
		Policy:         model.PolicyNone,
		PhaseWIPConfig: cfg,
	})
	if err != nil {
		t.Fatalf("EmitCreated: %v", err)
	}
	for i := 0; i < devCount; i++ {
		devID := "dev-" + string(rune('a'+i))
		devName := "Dev" + string(rune('A'+i))
		eng, err = eng.AddDeveloper(devID, devName, 1.0)
		if err != nil {
			t.Fatalf("AddDeveloper: %v", err)
		}
	}
	return eng
}

// Implement cap = 2 with 3 backlog tickets and 3 devs: at most 2 should
// land in Implement concurrently; the 3rd waits in PhaseQueues[Implement]
// (head-of-line blocking) even though a dev is idle.
func TestAssignFromQueues_PerPhaseCap_BlocksWhenAtCap(t *testing.T) {
	cfg := map[model.WorkflowPhase]int{
		model.PhaseImplement: 2,
	}
	eng := makeCappedEngine(t, 42, cfg, 3)

	// Three tickets, all directly assigned to Implement (skip backlog churn).
	for i, id := range []string{"T1", "T2", "T3"} {
		var err error
		eng, err = eng.AddTicket(model.NewTicket(id, "ticket-"+id, 3, model.HighUnderstanding))
		if err != nil {
			t.Fatalf("AddTicket %d: %v", i, err)
		}
	}

	var err error
	eng, err = eng.StartSprint()
	if err != nil {
		t.Fatalf("StartSprint: %v", err)
	}

	// Manually assign each to a dev for Research (UC37 lesson — RunSprint
	// doesn't auto-pull from CommittedTickets). The cap is on Implement,
	// not Research, so all 3 should pick up devs initially.
	for i, id := range []string{"T1", "T2", "T3"} {
		dev := "dev-" + string(rune('a'+i))
		eng, _ = eng.AssignTicket(id, dev)
	}

	// Tick until tickets reach Implement (or many ticks elapse).
	for tick := 0; tick < 100; tick++ {
		eng, _, _ = eng.Tick()
	}

	state := eng.Sim()
	implementAssigned := state.PhaseWIPCount(model.PhaseImplement)

	if implementAssigned > 2 {
		t.Errorf("PhaseWIPCount(Implement) = %d; cap=2 must not be exceeded", implementAssigned)
	}
}

// Direct unit test of the gate's underlying counter — covers the
// table-driven scenarios called out in the plan: cap-blocked /
// cap-allowed / unlimited cases against the same Simulation snapshot.
func TestPhaseWIPCount_GateInputs(t *testing.T) {
	tests := []struct {
		name        string
		assigned    int
		queued      int
		cap         int
		wantCount   int
		wantBlocked bool // count >= cap
	}{
		{
			name:        "empty phase + cap=2: not blocked",
			assigned:    0,
			queued:      0,
			cap:         2,
			wantCount:   0,
			wantBlocked: false,
		},
		{
			name:        "1 assigned + cap=2: not blocked",
			assigned:    1,
			queued:      0,
			cap:         2,
			wantCount:   1,
			wantBlocked: false,
		},
		{
			name:        "2 assigned + cap=2: blocked (gate must break)",
			assigned:    2,
			queued:      0,
			cap:         2,
			wantCount:   2,
			wantBlocked: true,
		},
		{
			name:        "2 assigned + 3 queued + cap=2: blocked (queued don't count toward cap)",
			assigned:    2,
			queued:      3,
			cap:         2,
			wantCount:   2,
			wantBlocked: true,
		},
		{
			name:        "0 assigned + 3 queued + cap=2: NOT blocked (queue is the head-of-line surface)",
			assigned:    0,
			queued:      3,
			cap:         2,
			wantCount:   0,
			wantBlocked: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := model.NewSimulation("test", model.PolicyNone, 0)
			tickets := make([]model.Ticket, 0, tc.assigned+tc.queued)
			for i := 0; i < tc.assigned; i++ {
				tickets = append(tickets, model.Ticket{
					ID:         "A" + string(rune('0'+i)),
					Phase:      model.PhaseImplement,
					AssignedTo: "dev-" + string(rune('a'+i)),
				})
			}
			queue := make([]string, 0, tc.queued)
			for i := 0; i < tc.queued; i++ {
				id := "Q" + string(rune('0'+i))
				tickets = append(tickets, model.Ticket{
					ID:         id,
					Phase:      model.PhaseImplement,
					AssignedTo: "",
				})
				queue = append(queue, id)
			}
			s.ActiveTickets = tickets
			s.PhaseQueues = map[model.WorkflowPhase][]string{
				model.PhaseImplement: queue,
			}
			s.PhaseWIPConfig = map[model.WorkflowPhase]int{
				model.PhaseImplement: tc.cap,
			}

			got := s.PhaseWIPCount(model.PhaseImplement)
			if got != tc.wantCount {
				t.Errorf("PhaseWIPCount = %d; want %d", got, tc.wantCount)
			}
			blocked := got >= s.PhaseWIPCap(model.PhaseImplement)
			if blocked != tc.wantBlocked {
				t.Errorf("blocked = %v; want %v", blocked, tc.wantBlocked)
			}
		})
	}
}

// CICDSlots fallback: PhaseWIPCap(PhaseCICD) returns CICDSlots when
// PhaseWIPConfig has no explicit CICD entry. Drives the parent epic's
// "declared but never enforced" closure (#15441 Phase 1 finding).
func TestPhaseWIPCap_CICDSlotsFallback_GateAware(t *testing.T) {
	s := model.NewSimulation("test", model.PolicyNone, 0)
	s.CICDSlots = 3
	// No PhaseWIPConfig entry for CICD.
	if got := s.PhaseWIPCap(model.PhaseCICD); got != 3 {
		t.Errorf("PhaseWIPCap(PhaseCICD) = %d; want 3 (CICDSlots fallback)", got)
	}
	// Now override via PhaseWIPConfig.
	s.PhaseWIPConfig = map[model.WorkflowPhase]int{model.PhaseCICD: 1}
	if got := s.PhaseWIPCap(model.PhaseCICD); got != 1 {
		t.Errorf("PhaseWIPCap(PhaseCICD) = %d; want 1 (explicit override)", got)
	}
}
