package events_test

import (
	"testing"
	"time"

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

	// Verify SprintNumber on simulation is updated (bug fix)
	if state.SprintNumber != 1 {
		t.Errorf("Simulation.SprintNumber = %d, want 1", state.SprintNumber)
	}

	// Start sprint 2
	proj = got.Apply(events.NewSprintEnded("sim-1", 10, 1))
	proj = proj.Apply(events.NewSprintStarted("sim-1", 10, 2, 2.5))
	state = proj.State()
	if state.SprintNumber != 2 {
		t.Errorf("Simulation.SprintNumber after sprint 2 = %d, want 2", state.SprintNumber)
	}
}

func TestProjection_Apply_TicketAssigned(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))
	proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "TKT-001", "Fix bug", 3.0, model.MediumUnderstanding))

	// Assign at tick 5
	startedAt := time.Now()
	evt := events.NewTicketAssigned("sim-1", 5, "TKT-001", "dev-1", startedAt)
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

	// Verify StartedTick is set
	ticket := state.ActiveTickets[0]
	if ticket.StartedTick != 5 {
		t.Errorf("Ticket.StartedTick = %d, want 5", ticket.StartedTick)
	}

	// Verify StartedAt is set (approximately - allow 1 second tolerance)
	if ticket.StartedAt.Before(startedAt.Add(-time.Second)) || ticket.StartedAt.After(startedAt.Add(time.Second)) {
		t.Errorf("Ticket.StartedAt = %v, want approximately %v", ticket.StartedAt, startedAt)
	}
}

func TestProjection_Apply_WorkProgressed(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))
	proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "TKT-001", "Fix bug", 3.0, model.MediumUnderstanding))
	proj = proj.Apply(events.NewTicketAssigned("sim-1", 0, "TKT-001", "dev-1", time.Time{}))

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
	proj = proj.Apply(events.NewTicketAssigned("sim-1", 0, "TKT-001", "dev-1", time.Time{}))

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
	proj = proj.Apply(events.NewTicketAssigned("sim-1", 0, "TKT-001", "dev-1", time.Time{}))

	evt := events.NewTicketCompleted("sim-1", 3, "TKT-001", "dev-1", 3.0)
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

func TestProjection_Apply_IncidentStarted(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))

	evt := events.NewIncidentStarted("sim-1", 10, "INC-001", "DEV-001", "", model.SeverityLow)
	got := proj.Apply(evt)

	state := got.State()
	if len(state.OpenIncidents) != 1 {
		t.Fatalf("len(OpenIncidents) = %d, want 1", len(state.OpenIncidents))
	}
	if state.OpenIncidents[0].ID != "INC-001" {
		t.Errorf("Incident.ID = %q, want %q", state.OpenIncidents[0].ID, "INC-001")
	}
}

func TestProjection_Apply_IncidentResolved(t *testing.T) {
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewIncidentStarted("sim-1", 10, "INC-001", "DEV-001", "", model.SeverityLow))

	evt := events.NewIncidentResolved("sim-1", 15, "INC-001", "DEV-001")
	got := proj.Apply(evt)

	state := got.State()
	if len(state.OpenIncidents) != 0 {
		t.Errorf("len(OpenIncidents) = %d, want 0", len(state.OpenIncidents))
	}
	if len(state.ResolvedIncidents) != 1 {
		t.Fatalf("len(ResolvedIncidents) = %d, want 1", len(state.ResolvedIncidents))
	}
	if state.ResolvedIncidents[0].ResolvedAt == nil {
		t.Error("ResolvedAt should not be nil")
	}
}

func TestProjection_Apply_IncidentResolved_MultipleIncidents(t *testing.T) {
	// Verify resolving one incident doesn't affect others
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewIncidentStarted("sim-1", 10, "INC-001", "DEV-001", "", model.SeverityLow))
	proj = proj.Apply(events.NewIncidentStarted("sim-1", 12, "INC-002", "DEV-002", "", model.SeverityMedium))

	// Resolve first incident
	got := proj.Apply(events.NewIncidentResolved("sim-1", 15, "INC-001", "DEV-001"))

	state := got.State()
	// Second incident should still be open
	if len(state.OpenIncidents) != 1 {
		t.Fatalf("len(OpenIncidents) = %d, want 1", len(state.OpenIncidents))
	}
	if state.OpenIncidents[0].ID != "INC-002" {
		t.Errorf("OpenIncidents[0].ID = %q, want %q", state.OpenIncidents[0].ID, "INC-002")
	}
	// First incident should be resolved
	if len(state.ResolvedIncidents) != 1 {
		t.Fatalf("len(ResolvedIncidents) = %d, want 1", len(state.ResolvedIncidents))
	}
	if state.ResolvedIncidents[0].ID != "INC-001" {
		t.Errorf("ResolvedIncidents[0].ID = %q, want %q", state.ResolvedIncidents[0].ID, "INC-001")
	}
}

func TestProjection_Apply_IncidentResolved_NonExistent(t *testing.T) {
	// Verify resolving non-existent incident is a no-op (event sourcing idempotency)
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewIncidentStarted("sim-1", 10, "INC-001", "DEV-001", "", model.SeverityLow))

	// Try to resolve incident that doesn't exist
	got := proj.Apply(events.NewIncidentResolved("sim-1", 15, "INC-999", "DEV-001"))

	state := got.State()
	// Original incident should still be open
	if len(state.OpenIncidents) != 1 {
		t.Errorf("len(OpenIncidents) = %d, want 1", len(state.OpenIncidents))
	}
	// Nothing should be resolved
	if len(state.ResolvedIncidents) != 0 {
		t.Errorf("len(ResolvedIncidents) = %d, want 0", len(state.ResolvedIncidents))
	}
	// Version should still increment (event was processed)
	if got.Version() != 3 {
		t.Errorf("Version() = %d, want 3", got.Version())
	}
}

// cmp is used for complex struct comparisons
var _ = cmp.Diff

func TestProjection_Apply_SprintWIPUpdated(t *testing.T) {
	// Setup: create simulation with sprint and some active tickets
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{
		SprintLength: 10,
		Seed:         42,
	}))
	proj = proj.Apply(events.NewSprintStarted("sim-1", 0, 1, 2.0))

	// Apply WIP update with 3 active tickets
	evt := events.NewSprintWIPUpdated("sim-1", 1, 3)
	got := proj.Apply(evt)

	state := got.State()
	sprint, ok := state.CurrentSprintOption.Get()
	if !ok {
		t.Fatal("CurrentSprintOption should be set")
	}

	// Verify WIP tracking updated
	if sprint.MaxWIP != 3 {
		t.Errorf("Sprint.MaxWIP = %d, want 3", sprint.MaxWIP)
	}
	if sprint.WIPSum != 3 {
		t.Errorf("Sprint.WIPSum = %d, want 3", sprint.WIPSum)
	}
	if sprint.WIPTicks != 1 {
		t.Errorf("Sprint.WIPTicks = %d, want 1", sprint.WIPTicks)
	}

	// Apply another WIP update with 5 active tickets (higher)
	got = got.Apply(events.NewSprintWIPUpdated("sim-1", 2, 5))
	sprint, _ = got.State().CurrentSprintOption.Get()

	if sprint.MaxWIP != 5 {
		t.Errorf("Sprint.MaxWIP = %d, want 5 (higher of 3 and 5)", sprint.MaxWIP)
	}
	if sprint.WIPSum != 8 {
		t.Errorf("Sprint.WIPSum = %d, want 8 (3+5)", sprint.WIPSum)
	}
	if sprint.WIPTicks != 2 {
		t.Errorf("Sprint.WIPTicks = %d, want 2", sprint.WIPTicks)
	}

	// Apply WIP update with 2 active tickets (lower - MaxWIP shouldn't change)
	got = got.Apply(events.NewSprintWIPUpdated("sim-1", 3, 2))
	sprint, _ = got.State().CurrentSprintOption.Get()

	if sprint.MaxWIP != 5 {
		t.Errorf("Sprint.MaxWIP = %d, want 5 (max unchanged)", sprint.MaxWIP)
	}
	if sprint.WIPSum != 10 {
		t.Errorf("Sprint.WIPSum = %d, want 10 (3+5+2)", sprint.WIPSum)
	}
	if sprint.WIPTicks != 3 {
		t.Errorf("Sprint.WIPTicks = %d, want 3", sprint.WIPTicks)
	}
}

func TestProjection_Apply_TicketDecomposed(t *testing.T) {
	// Setup: create simulation with a parent ticket in backlog
	proj := events.NewProjection()
	proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
	proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "TKT-PARENT", "Big feature", 10.0, model.LowUnderstanding))

	// Verify parent is in backlog
	if len(proj.State().Backlog) != 1 {
		t.Fatalf("Setup: len(Backlog) = %d, want 1", len(proj.State().Backlog))
	}

	// Decompose parent into 2 children
	children := []events.ChildTicket{
		{ID: "TKT-CHILD-1", Title: "Part 1", EstimatedDays: 4.0, Understanding: model.MediumUnderstanding},
		{ID: "TKT-CHILD-2", Title: "Part 2", EstimatedDays: 4.0, Understanding: model.MediumUnderstanding},
	}
	evt := events.NewTicketDecomposed("sim-1", 0, "TKT-PARENT", children)
	got := proj.Apply(evt)

	state := got.State()

	// Parent should be removed from backlog
	for _, ticket := range state.Backlog {
		if ticket.ID == "TKT-PARENT" {
			t.Errorf("Parent ticket should be removed from backlog, but found it")
		}
	}

	// Children should be in backlog
	if len(state.Backlog) != 2 {
		t.Fatalf("len(Backlog) = %d, want 2", len(state.Backlog))
	}

	// Verify child tickets exist with correct properties
	childIDs := map[string]bool{}
	for _, ticket := range state.Backlog {
		childIDs[ticket.ID] = true
		if ticket.Phase != model.PhaseBacklog {
			t.Errorf("Child %s Phase = %v, want PhaseBacklog", ticket.ID, ticket.Phase)
		}
	}
	if !childIDs["TKT-CHILD-1"] {
		t.Error("TKT-CHILD-1 not found in backlog")
	}
	if !childIDs["TKT-CHILD-2"] {
		t.Error("TKT-CHILD-2 not found in backlog")
	}
}

func TestProjection_Apply_BugDiscovered(t *testing.T) {
	tests := []struct {
		name         string
		reworkEffort float64
	}{
		{"adds half day rework", 0.5},
		{"adds full day rework", 1.0},
		{"adds small rework", 0.25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup: simulation with an active ticket
			proj := events.NewProjection()
			proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
			proj = proj.Apply(events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))
			proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "TKT-001", "Fix bug", 5.0, model.MediumUnderstanding))
			proj = proj.Apply(events.NewTicketAssigned("sim-1", 1, "TKT-001", "dev-1", time.Time{}))

			// Get RemainingEffort after assignment (calculated from phase effort)
			effortBeforeBug := proj.State().ActiveTickets[0].RemainingEffort

			// Apply BugDiscovered event
			evt := events.NewBugDiscovered("sim-1", 2, "TKT-001", tt.reworkEffort)
			got := proj.Apply(evt)

			state := got.State()
			if len(state.ActiveTickets) != 1 {
				t.Fatalf("len(ActiveTickets) = %d, want 1", len(state.ActiveTickets))
			}

			ticket := state.ActiveTickets[0]
			expectedEffort := effortBeforeBug + tt.reworkEffort
			if ticket.RemainingEffort != expectedEffort {
				t.Errorf("RemainingEffort = %.2f, want %.2f (%.2f + %.2f)",
					ticket.RemainingEffort, expectedEffort, effortBeforeBug, tt.reworkEffort)
			}
		})
	}
}

func TestProjection_Apply_ScopeCreepOccurred(t *testing.T) {
	tests := []struct {
		name          string
		effortAdded   float64
		estimateAdded float64
	}{
		{"adds effort and estimate", 1.0, 1.0},
		{"adds small creep", 0.5, 0.5},
		{"adds large creep", 2.0, 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup: simulation with an active ticket
			proj := events.NewProjection()
			proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
			proj = proj.Apply(events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))
			proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "TKT-001", "Feature", 5.0, model.MediumUnderstanding))
			proj = proj.Apply(events.NewTicketAssigned("sim-1", 1, "TKT-001", "dev-1", time.Time{}))

			// Get values after assignment
			ticketBefore := proj.State().ActiveTickets[0]
			effortBefore := ticketBefore.RemainingEffort
			estimateBefore := ticketBefore.EstimatedDays

			// Apply ScopeCreepOccurred event
			evt := events.NewScopeCreepOccurred("sim-1", 2, "TKT-001", tt.effortAdded, tt.estimateAdded)
			got := proj.Apply(evt)

			state := got.State()
			if len(state.ActiveTickets) != 1 {
				t.Fatalf("len(ActiveTickets) = %d, want 1", len(state.ActiveTickets))
			}

			ticket := state.ActiveTickets[0]

			// Verify RemainingEffort increased
			expectedEffort := effortBefore + tt.effortAdded
			if ticket.RemainingEffort != expectedEffort {
				t.Errorf("RemainingEffort = %.2f, want %.2f", ticket.RemainingEffort, expectedEffort)
			}

			// Verify EstimatedDays increased
			expectedEstimate := estimateBefore + tt.estimateAdded
			if ticket.EstimatedDays != expectedEstimate {
				t.Errorf("EstimatedDays = %.2f, want %.2f", ticket.EstimatedDays, expectedEstimate)
			}
		})
	}
}

func TestProjection_Apply_IncidentStarted_Enhanced(t *testing.T) {
	// Test the enhanced IncidentStarted: sets incident's TicketID/Severity
	// AND marks the completed ticket as having caused an incident.
	tests := []struct {
		name     string
		severity model.Severity
	}{
		{"low severity", model.SeverityLow},
		{"medium severity", model.SeverityMedium},
		{"high severity", model.SeverityHigh},
		{"critical severity", model.SeverityCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup: simulation with a completed ticket
			proj := events.NewProjection()
			proj = proj.Apply(events.NewSimulationCreated("sim-1", 0, events.SimConfig{Seed: 42}))
			proj = proj.Apply(events.NewDeveloperAdded("sim-1", 0, "dev-1", "Alice", 1.0))
			proj = proj.Apply(events.NewTicketCreated("sim-1", 0, "TKT-001", "Feature", 3.0, model.MediumUnderstanding))
			proj = proj.Apply(events.NewTicketAssigned("sim-1", 1, "TKT-001", "dev-1", time.Time{}))
			proj = proj.Apply(events.NewTicketCompleted("sim-1", 5, "TKT-001", "dev-1", 3.0))

			// Verify ticket is completed before incident
			if len(proj.State().CompletedTickets) != 1 {
				t.Fatalf("Setup: len(CompletedTickets) = %d, want 1", len(proj.State().CompletedTickets))
			}

			// Apply IncidentStarted with TicketID and Severity
			evt := events.NewIncidentStarted("sim-1", 10, "INC-001", "dev-1", "TKT-001", tt.severity)
			got := proj.Apply(evt)

			state := got.State()

			// Verify incident created with correct fields
			if len(state.OpenIncidents) != 1 {
				t.Fatalf("len(OpenIncidents) = %d, want 1", len(state.OpenIncidents))
			}
			incident := state.OpenIncidents[0]
			if incident.ID != "INC-001" {
				t.Errorf("Incident.ID = %q, want %q", incident.ID, "INC-001")
			}
			if incident.TicketID != "TKT-001" {
				t.Errorf("Incident.TicketID = %q, want %q", incident.TicketID, "TKT-001")
			}
			if incident.Severity != tt.severity {
				t.Errorf("Incident.Severity = %v, want %v", incident.Severity, tt.severity)
			}

			// Verify completed ticket was marked as causing incident
			if len(state.CompletedTickets) != 1 {
				t.Fatalf("len(CompletedTickets) = %d, want 1", len(state.CompletedTickets))
			}
			ticket := state.CompletedTickets[0]
			if !ticket.CausedIncident {
				t.Errorf("Ticket.CausedIncident = false, want true")
			}
			if ticket.IncidentID != "INC-001" {
				t.Errorf("Ticket.IncidentID = %q, want %q", ticket.IncidentID, "INC-001")
			}
		})
	}
}
