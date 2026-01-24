package engine_test

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// setupBenchmarkSimulation creates a simulation with specified developers and tickets.
func setupBenchmarkSimulation(numDevs, numTickets int) model.Simulation {
	sim := model.NewSimulation("bench", model.PolicyDORAStrict, 12345)

	for i := 0; i < numDevs; i++ {
		sim.Developers = append(sim.Developers, model.NewDeveloper(
			idFor("dev", i),
			nameFor(i),
			1.0,
		))
	}

	for i := 0; i < numTickets; i++ {
		understanding := model.UnderstandingLevel(i % 3) // Cycle through H/M/L
		sim.Backlog = append(sim.Backlog, model.NewTicket(
			idFor("TKT", i),
			"Benchmark ticket",
			float64(3+(i%5)), // 3-7 day estimates
			understanding,
		))
	}

	return sim
}

// idFor generates an ID like "dev-001" or "TKT-042".
func idFor(prefix string, n int) string {
	return prefix + "-" + padInt(n)
}

// padInt returns a zero-padded 3-digit string.
func padInt(n int) string {
	if n < 10 {
		return "00" + itoa(n)
	}
	if n < 100 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

// itoa converts int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// nameFor returns a developer name.
func nameFor(n int) string {
	names := []string{"Alice", "Bob", "Carol", "Dave", "Eve", "Frank", "Grace", "Henry"}
	return names[n%len(names)]
}

// BenchmarkTick measures the performance of a single simulation tick.
// This is the primary hot path - called once per simulated day.
func BenchmarkTick(b *testing.B) {
	sim := setupBenchmarkSimulation(10, 50)
	eng := engine.NewEngine(sim.Seed)
	eng.EmitLoadedState(sim)

	// Assign some tickets to make tick do real work
	state := eng.Sim()
	for i := 0; i < 10 && i < len(state.Backlog); i++ {
		eng.AssignTicket(state.Backlog[0].ID, state.Developers[i%len(state.Developers)].ID)
		state = eng.Sim()
	}

	// Start a sprint
	eng.StartSprint()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eng.Tick()
	}
}

// BenchmarkTick_LargeSimulation measures tick with more developers and tickets.
func BenchmarkTick_LargeSimulation(b *testing.B) {
	sim := setupBenchmarkSimulation(30, 200)
	eng := engine.NewEngine(sim.Seed)
	eng.EmitLoadedState(sim)

	// Assign tickets
	state := eng.Sim()
	for i := 0; i < 30 && i < len(state.Backlog); i++ {
		eng.AssignTicket(state.Backlog[0].ID, state.Developers[i%len(state.Developers)].ID)
		state = eng.Sim()
	}

	eng.StartSprint()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eng.Tick()
	}
}

// BenchmarkFindActiveTicketIndex measures the linear search bottleneck.
// This is called once per working developer per tick.
func BenchmarkFindActiveTicketIndex(b *testing.B) {
	sim := setupBenchmarkSimulation(0, 100)

	// Move all tickets to active
	for i := 0; i < 100; i++ {
		if len(sim.Backlog) > 0 {
			ticket := sim.Backlog[0]
			sim.ActiveTickets = append(sim.ActiveTickets, ticket)
			sim.Backlog = sim.Backlog[1:]
		}
	}

	// Search for middle ticket (worst average case)
	targetID := sim.ActiveTickets[50].ID

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sim.FindActiveTicketIndex(targetID)
	}
}

// BenchmarkFindActiveTicketIndex_Small measures search with fewer tickets.
func BenchmarkFindActiveTicketIndex_Small(b *testing.B) {
	sim := setupBenchmarkSimulation(0, 10)

	for i := 0; i < 10; i++ {
		if len(sim.Backlog) > 0 {
			ticket := sim.Backlog[0]
			sim.ActiveTickets = append(sim.ActiveTickets, ticket)
			sim.Backlog = sim.Backlog[1:]
		}
	}

	targetID := sim.ActiveTickets[5].ID

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sim.FindActiveTicketIndex(targetID)
	}
}

// BenchmarkVarianceCalculate measures the variance model calculation.
// Called once per developer per tick.
func BenchmarkVarianceCalculate(b *testing.B) {
	vm := engine.NewVarianceModel(12345)
	ticket := model.NewTicket("TKT-001", "Test", 5, model.MediumUnderstanding)
	ticket.Phase = model.PhaseImplement

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = vm.Calculate(ticket, i)
	}
}

// BenchmarkRunSprint measures a complete sprint execution.
func BenchmarkRunSprint(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		sim := setupBenchmarkSimulation(5, 20)
		eng := engine.NewEngine(sim.Seed)
		eng.EmitLoadedState(sim)

		// Assign tickets
		state := eng.Sim()
		for j := 0; j < 5 && j < len(state.Backlog); j++ {
			eng.AssignTicket(state.Backlog[0].ID, state.Developers[j].ID)
			state = eng.Sim()
		}

		b.StartTimer()
		eng.RunSprint()
	}
}
