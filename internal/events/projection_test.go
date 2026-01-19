package events_test

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
	"github.com/google/go-cmp/cmp"
)

// TestProjection_Apply tests the Projection.Apply() method for each event type.
// Per Khorikov: Domain logic gets heavy unit testing with table-driven tests.
// Projection is pure (no side effects), making it ideal for unit testing.

func TestProjection_Apply_SimulationCreated(t *testing.T) {
	proj := events.NewProjection()

	evt := events.NewSimulationCreated("sim-1", 0, events.SimConfig{
		TeamSize:     3,
		SprintLength: 10,
		Seed:         42,
		Policy:       model.PolicyDORAStrict,
	})

	got := proj.Apply(evt)

	// Verify state was initialized
	state := got.State()
	if state.ID != "sim-1" {
		t.Errorf("ID = %q, want %q", state.ID, "sim-1")
	}
	if state.Seed != 42 {
		t.Errorf("Seed = %d, want %d", state.Seed, 42)
	}
	if state.SizingPolicy != model.PolicyDORAStrict {
		t.Errorf("SizingPolicy = %v, want %v", state.SizingPolicy, model.PolicyDORAStrict)
	}
	if got.Version() != 1 {
		t.Errorf("Version = %d, want 1", got.Version())
	}
}

func TestProjection_Apply_Ticked(t *testing.T) {
	// Setup: create simulation first
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))

	// Apply tick event
	evt := events.NewTicked("sim-1", 1)
	got := proj.Apply(evt)

	state := got.State()
	if state.CurrentTick != 1 {
		t.Errorf("CurrentTick = %d, want 1", state.CurrentTick)
	}
	if got.Version() != 2 {
		t.Errorf("Version = %d, want 2", got.Version())
	}
}

func TestProjection_Apply_DeveloperAdded(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))

	evt := events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0)
	got := proj.Apply(evt)

	state := got.State()
	if len(state.Developers) != 1 {
		t.Fatalf("len(Developers) = %d, want 1", len(state.Developers))
	}
	dev := state.Developers[0]
	if dev.ID != "dev-1" {
		t.Errorf("Developer.ID = %q, want %q", dev.ID, "dev-1")
	}
	if dev.Name != "Alice" {
		t.Errorf("Developer.Name = %q, want %q", dev.Name, "Alice")
	}
	if dev.Velocity != 1.0 {
		t.Errorf("Developer.Velocity = %f, want 1.0", dev.Velocity)
	}
}

func TestProjection_Apply_TicketCreated(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))

	evt := events.NewTicketCreated("sim-1", 0, "TKT-001", "Fix bug", 3.0, model.MediumUnderstanding)
	got := proj.Apply(evt)

	state := got.State()
	if len(state.Backlog) != 1 {
		t.Fatalf("len(Backlog) = %d, want 1", len(state.Backlog))
	}
	ticket := state.Backlog[0]
	if ticket.ID != "TKT-001" {
		t.Errorf("Ticket.ID = %q, want %q", ticket.ID, "TKT-001")
	}
	if ticket.Title != "Fix bug" {
		t.Errorf("Ticket.Title = %q, want %q", ticket.Title, "Fix bug")
	}
	if ticket.EstimatedDays != 3.0 {
		t.Errorf("Ticket.EstimatedDays = %f, want 3.0", ticket.EstimatedDays)
	}
}

func TestProjection_Apply_SprintStarted(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{
		SprintLength: 10,
		Seed:         42,
	}))

	// SprintStarted now includes BufferDays for fever chart
	evt := events.NewSprintStarted("sim-1", 0, 1, 2.0) // 2 buffer days
	got := proj.Apply(evt)

	state := got.State()
	sprint, ok := state.CurrentSprintOption.Get()
	if !ok {
		t.Fatal("CurrentSprintOption should be set")
	}
	if sprint.Number != 1 {
		t.Errorf("Sprint.Number = %d, want 1", sprint.Number)
	}
	if sprint.StartDay != 0 {
		t.Errorf("Sprint.StartDay = %d, want 0", sprint.StartDay)
	}
	if sprint.BufferDays != 2.0 {
		t.Errorf("Sprint.BufferDays = %f, want 2.0", sprint.BufferDays)
	}
}

func TestProjection_Apply_TicketAssigned(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))
	proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "TKT-001", "Fix bug", 3.0, model.MediumUnderstanding))

	evt := events.NewTicketAssigned("sim-1", 0, "TKT-001", "dev-1")
	got := proj.Apply(evt)

	state := got.State()

	// Ticket should move from Backlog to ActiveTickets
	if len(state.Backlog) != 0 {
		t.Errorf("len(Backlog) = %d, want 0", len(state.Backlog))
	}
	if len(state.ActiveTickets) != 1 {
		t.Fatalf("len(ActiveTickets) = %d, want 1", len(state.ActiveTickets))
	}

	// Developer should have ticket assigned
	if state.Developers[0].CurrentTicket != "TKT-001" {
		t.Errorf("Developer.CurrentTicket = %q, want %q", state.Developers[0].CurrentTicket, "TKT-001")
	}
}

func TestProjection_Apply_WorkProgressed(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))
	proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "TKT-001", "Fix bug", 3.0, model.MediumUnderstanding))
	proj = proj.Apply(events.NewTicketAssigned("sim-1", 0, "TKT-001", "dev-1"))

	// Get initial remaining effort
	initialEffort := proj.State().ActiveTickets[0].RemainingEffort

	// WorkProgressed now includes Phase for PhaseEffortSpent tracking
	evt := events.NewWorkProgressed("sim-1", 1, "TKT-001", model.PhaseResearch, 1.0)
	got := proj.Apply(evt)

	state := got.State()
	ticket := state.ActiveTickets[0]

	if ticket.RemainingEffort != initialEffort-1.0 {
		t.Errorf("RemainingEffort = %f, want %f", ticket.RemainingEffort, initialEffort-1.0)
	}
	if ticket.ActualDays != 1.0 {
		t.Errorf("ActualDays = %f, want 1.0", ticket.ActualDays)
	}
	// Verify PhaseEffortSpent is tracked
	if ticket.PhaseEffortSpent[model.PhaseResearch] != 1.0 {
		t.Errorf("PhaseEffortSpent[Research] = %f, want 1.0", ticket.PhaseEffortSpent[model.PhaseResearch])
	}
}

func TestProjection_Apply_WorkProgressed_MultiplePhases(t *testing.T) {
	// Verify PhaseEffortSpent accumulates correctly across multiple phases
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))
	proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "TKT-001", "Fix bug", 3.0, model.MediumUnderstanding))
	proj = proj.Apply(events.NewTicketAssigned("sim-1", 0, "TKT-001", "dev-1"))

	// Work in Research phase
	proj = proj.Apply(events.NewWorkProgressed("sim-1", 1, "TKT-001", model.PhaseResearch, 0.5))
	proj = proj.Apply(events.NewWorkProgressed("sim-1", 2, "TKT-001", model.PhaseResearch, 0.3))

	// Transition to Implement phase
	proj = proj.Apply(events.NewTicketPhaseChanged("sim-1", 3, "TKT-001", model.PhaseResearch, model.PhaseImplement))

	// Work in Implement phase
	proj = proj.Apply(events.NewWorkProgressed("sim-1", 4, "TKT-001", model.PhaseImplement, 1.0))
	proj = proj.Apply(events.NewWorkProgressed("sim-1", 5, "TKT-001", model.PhaseImplement, 0.7))

	state := proj.State()
	ticket := state.ActiveTickets[0]

	// Verify Research effort accumulated
	if ticket.PhaseEffortSpent[model.PhaseResearch] != 0.8 {
		t.Errorf("PhaseEffortSpent[Research] = %f, want 0.8", ticket.PhaseEffortSpent[model.PhaseResearch])
	}
	// Verify Implement effort accumulated
	if ticket.PhaseEffortSpent[model.PhaseImplement] != 1.7 {
		t.Errorf("PhaseEffortSpent[Implement] = %f, want 1.7", ticket.PhaseEffortSpent[model.PhaseImplement])
	}
	// Verify total ActualDays
	if ticket.ActualDays != 2.5 {
		t.Errorf("ActualDays = %f, want 2.5", ticket.ActualDays)
	}
}

func TestProjection_Apply_TicketCompleted(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))
	proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "TKT-001", "Fix bug", 3.0, model.MediumUnderstanding))
	proj = proj.Apply(events.NewTicketAssigned("sim-1", 0, "TKT-001", "dev-1"))

	evt := events.NewTicketCompleted("sim-1", 3, "TKT-001", "dev-1")
	got := proj.Apply(evt)

	state := got.State()

	// Ticket should move from ActiveTickets to CompletedTickets
	if len(state.ActiveTickets) != 0 {
		t.Errorf("len(ActiveTickets) = %d, want 0", len(state.ActiveTickets))
	}
	if len(state.CompletedTickets) != 1 {
		t.Fatalf("len(CompletedTickets) = %d, want 1", len(state.CompletedTickets))
	}

	// Developer should be idle
	if state.Developers[0].CurrentTicket != "" {
		t.Errorf("Developer.CurrentTicket = %q, want empty", state.Developers[0].CurrentTicket)
	}
}

func TestProjection_Apply_BufferConsumed(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{
		SprintLength: 10,
		Seed:         42,
	}))
	proj = proj.Apply(events.NewSprintStarted("sim-1", 0, 1, 2.0)) // 2 buffer days

	// Consume some buffer
	evt := events.NewBufferConsumed("sim-1", 1, 0.5) // 0.5 days consumed
	got := proj.Apply(evt)

	state := got.State()
	sprint, ok := state.CurrentSprintOption.Get()
	if !ok {
		t.Fatal("CurrentSprintOption should be set")
	}
	if sprint.BufferConsumed != 0.5 {
		t.Errorf("Sprint.BufferConsumed = %f, want 0.5", sprint.BufferConsumed)
	}

	// Consume more
	got = got.Apply(events.NewBufferConsumed("sim-1", 2, 0.3))
	sprint, _ = got.State().CurrentSprintOption.Get()
	if sprint.BufferConsumed != 0.8 {
		t.Errorf("Sprint.BufferConsumed = %f, want 0.8", sprint.BufferConsumed)
	}
}

func TestProjection_Apply_BufferConsumed_FeverTransitions(t *testing.T) {
	// Verify fever status transitions as buffer is consumed
	// Thresholds: Green (<33%), Yellow (33-65%), Red (>=66%)
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{
		SprintLength: 10,
		Seed:         42,
	}))
	proj = proj.Apply(events.NewSprintStarted("sim-1", 0, 1, 3.0)) // 3 buffer days

	// Initially Green (0% consumed)
	sprint, _ := proj.State().CurrentSprintOption.Get()
	if sprint.FeverStatus != model.FeverGreen {
		t.Errorf("Initial FeverStatus = %v, want FeverGreen", sprint.FeverStatus)
	}

	// Consume 1.0 of 3.0 = 33% -> Yellow (threshold is <0.33 for Green)
	proj = proj.Apply(events.NewBufferConsumed("sim-1", 1, 1.0))
	sprint, _ = proj.State().CurrentSprintOption.Get()
	if sprint.FeverStatus != model.FeverYellow {
		t.Errorf("After 33%% consumed: FeverStatus = %v, want FeverYellow", sprint.FeverStatus)
	}

	// Consume another 0.9 = 63% total -> still Yellow (threshold is <0.66)
	proj = proj.Apply(events.NewBufferConsumed("sim-1", 2, 0.9))
	sprint, _ = proj.State().CurrentSprintOption.Get()
	if sprint.FeverStatus != model.FeverYellow {
		t.Errorf("After 63%% consumed: FeverStatus = %v, want FeverYellow", sprint.FeverStatus)
	}

	// Consume another 0.2 = 70% total -> Red (>=66%)
	proj = proj.Apply(events.NewBufferConsumed("sim-1", 3, 0.2))
	sprint, _ = proj.State().CurrentSprintOption.Get()
	if sprint.FeverStatus != model.FeverRed {
		t.Errorf("After 70%% consumed: FeverStatus = %v, want FeverRed", sprint.FeverStatus)
	}
}

func TestProjection_Apply_SprintEnded(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewSprintStarted("sim-1", 0, 1, 2.0))

	evt := events.NewSprintEnded("sim-1", 10, 1)
	got := proj.Apply(evt)

	state := got.State()
	_, ok := state.CurrentSprintOption.Get()
	if ok {
		t.Error("CurrentSprintOption should be cleared after SprintEnded")
	}
}

func TestProjection_ValueSemantics(t *testing.T) {
	// Verify Apply returns new Projection, doesn't mutate original
	proj1 := events.NewProjection()
	proj2 := proj1.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))

	// Original should be unchanged
	if proj1.Version() != 0 {
		t.Errorf("Original proj1.Version() = %d, want 0 (should be unchanged)", proj1.Version())
	}
	if proj1.State().ID != "" {
		t.Errorf("Original proj1.State().ID = %q, want empty (should be unchanged)", proj1.State().ID)
	}

	// New projection should have changes
	if proj2.Version() != 1 {
		t.Errorf("New proj2.Version() = %d, want 1", proj2.Version())
	}
	if proj2.State().ID != "sim-1" {
		t.Errorf("New proj2.State().ID = %q, want %q", proj2.State().ID, "sim-1")
	}
}

// BenchmarkProjection_Apply_SingleEvent benchmarks applying a single event.
// Target: < 1μs/op
func BenchmarkProjection_Apply_SingleEvent(b *testing.B) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	evt := events.NewTicked("sim-1", 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proj.Apply(evt)
	}
}

// BenchmarkProjection_ReplayFull benchmarks replaying 1000 events.
// Target: < 1ms total
func BenchmarkProjection_ReplayFull(b *testing.B) {
	// Generate 1000 events: create, add devs, add tickets, ticks
	evts := generateTestEvents(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proj := events.NewProjection()
		for _, e := range evts {
			proj = proj.Apply(e)
		}
	}
}

// generateTestEvents creates n events for benchmarking.
func generateTestEvents(n int) []events.Event {
	result := make([]events.Event, 0, n)

	// Start with SimulationCreated
	result = append(result, events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))

	// Add 3 developers
	result = append(result, events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))
	result = append(result, events.NewDeveloperAdded("sim-1", 0, "dev-2", "Bob", 0.8))
	result = append(result, events.NewDeveloperAdded("sim-1", 0, "dev-3", "Carol", 1.2))

	// Fill rest with Ticked events (most common event type)
	for i := len(result); i < n; i++ {
		result = append(result, events.NewTicked("sim-1", i))
	}

	return result
}

// cmp is used for complex struct comparisons
var _ = cmp.Diff
