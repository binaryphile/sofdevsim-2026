package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/binaryphile/fluentfp/either"
	"github.com/binaryphile/fluentfp/must"
	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/sofdevsim-2026/internal/api"
	"github.com/binaryphile/sofdevsim-2026/internal/office"
	"github.com/binaryphile/sofdevsim-2026/internal/registry"
	tea "github.com/charmbracelet/bubbletea"
)

// TestNewAppWithSeed_Reproducibility verifies same seed produces identical initial state.
func TestNewAppWithSeed_Reproducibility(t *testing.T) {
	app1 := NewAppWithSeed(42)
	app2 := NewAppWithSeed(42)

	eng1, _ := app1.mode.GetLeft()
	eng2, _ := app2.mode.GetLeft()
	sim1 := eng1.Engine.Sim()
	sim2 := eng2.Engine.Sim()

	if sim1.Backlog[0].ID != sim2.Backlog[0].ID {
		t.Errorf("Seed 42 should produce identical backlogs, got %s and %s",
			sim1.Backlog[0].ID, sim2.Backlog[0].ID)
	}

	if sim1.Seed != sim2.Seed {
		t.Errorf("Seed should be stored identically, got %d and %d",
			sim1.Seed, sim2.Seed)
	}
}

// TestNewAppWithSeed_ZeroUsesRandomSeed verifies seed 0 produces different results.
func TestNewAppWithSeed_ZeroUsesRandomSeed(t *testing.T) {
	app1 := NewAppWithSeed(0)
	time.Sleep(time.Nanosecond) // Ensure different time
	app2 := NewAppWithSeed(0)

	eng1, _ := app1.mode.GetLeft()
	eng2, _ := app2.mode.GetLeft()
	sim1 := eng1.Engine.Sim()
	sim2 := eng2.Engine.Sim()

	// Different seeds should (almost always) produce different backlogs
	// This is probabilistic but failure is astronomically unlikely
	if sim1.Seed == sim2.Seed {
		t.Errorf("Seed 0 should use current time, producing different seeds")
	}
}

// TestSprintEndsWhenDurationReached verifies sprint is cleared after end day.
func TestSprintEndsWhenDurationReached(t *testing.T) {
	app := NewAppWithSeed(42)
	eng, _ := app.mode.GetLeft()
	eng.Engine = must.Get(eng.Engine.StartSprint())
	app.mode = either.Left[EngineMode, ClientMode](eng)
	app.paused = false              // Enable tick processing
	app.currentView = ViewExecution // Required for tick processing

	// Get sprint end day from engine projection
	sim := eng.Engine.Sim()
	sprint, ok := sim.CurrentSprintOption.Get()
	if !ok {
		t.Fatal("Expected sprint to be started")
	}

	// Run ticks until sprint ends (engine handles everything via events)
	for i := 0; i < sprint.DurationDays+1; i++ {
		app.Update(tickMsg(time.Now()))
	}

	// Sprint should be cleared in projection
	eng, _ = app.mode.GetLeft()
	sim = eng.Engine.Sim()
	if _, ok := sim.CurrentSprintOption.Get(); ok {
		t.Error("Sprint should be cleared after end day reached")
	}
	if !app.paused {
		t.Error("Should be paused after sprint ends")
	}
}

// TestNewAppWithRegistry_SubscribesToEvents verifies TUI subscribes to event store.
func TestNewAppWithRegistry_SubscribesToEvents(t *testing.T) {
	reg := registry.NewSimRegistry()
	app := NewAppWithRegistry(42, reg)
	eng, _ := app.mode.GetLeft()

	// TUI should have a subscription channel
	if eng.EventSub == nil {
		t.Fatal("Expected EventSub channel to be set")
	}

	// TUI should be registered in the shared registry
	// (accessible via API)
	sim := eng.Engine.Sim()
	evts := reg.Store().Replay(sim.ID)
	if len(evts) == 0 {
		t.Error("Expected SimulationCreated event in shared store")
	}
}

// TestNewAppWithRegistry_RegistryHasPopulatedState verifies API can see TUI's full state.
// Per design doc "TUI/API Shared Access": registry always holds fully-populated SimInstance.
// Tests ALL SimInstance fields at initialization (Khorikov: test boundary conditions).
func TestNewAppWithRegistry_RegistryHasPopulatedState(t *testing.T) {
	reg := registry.NewSimRegistry()
	_ = NewAppWithRegistry(42, reg)

	// API queries the registry - should see TUI's populated simulation
	inst, ok := reg.GetInstanceOption("sim-42").Get()
	if !ok {
		t.Fatal("Registry should contain TUI's simulation")
	}

	// Check Engine (primary state)
	sim := inst.Engine.Sim()
	if len(sim.Developers) != 6 {
		t.Errorf("Engine should have 6 developers, got %d", len(sim.Developers))
	}
	if len(sim.Backlog) != 12 {
		t.Errorf("Engine should have 12 tickets, got %d", len(sim.Backlog))
	}
	if sim.Developers[0].Name != "Mei" {
		t.Errorf("First developer should be Mei, got %s", sim.Developers[0].Name)
	}

	// Check Tracker (metrics)
	if inst.Tracker.DORA.DeploysLast7Days != 0 {
		t.Errorf("Initial tracker should have 0 deploys, got %d", inst.Tracker.DORA.DeploysLast7Days)
	}

	// Check Office (animation state) - regression: was missing, caused empty /office
	officeState := inst.Office.State()
	if len(officeState.Animations) != 6 {
		t.Errorf("Office should have 6 developer animations, got %d", len(officeState.Animations))
	}
}

// TestNewAppWithRegistry_HTTPCanSeeTUISimulation verifies HTTP API returns TUI's full state.
// Integration test: TUI creates simulation, HTTP endpoint returns it with all developers.
func TestNewAppWithRegistry_HTTPCanSeeTUISimulation(t *testing.T) {
	reg := api.NewSimRegistry()
	_ = NewAppWithRegistry(42, reg.SimRegistry)

	// Start HTTP server with shared registry
	router := api.NewRouter(reg)
	server := httptest.NewServer(router)
	defer server.Close()

	// Query via HTTP - should see TUI's simulation with 6 developers
	client := NewClient(server.URL)
	resp, err := client.GetSimulation("sim-42")
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}

	sim := resp.Simulation
	if len(sim.Developers) != 6 {
		t.Errorf("HTTP should return 6 developers, got %d", len(sim.Developers))
	}
	if len(sim.Backlog) != 12 {
		t.Errorf("HTTP should return 12 tickets, got %d", len(sim.Backlog))
	}
	if sim.Developers[0].Name != "Mei" {
		t.Errorf("First developer should be Mei, got %s", sim.Developers[0].Name)
	}
}

// TestNewAppWithSeed_ProjectionHasInitialState verifies projection has devs and tickets.
func TestNewAppWithSeed_ProjectionHasInitialState(t *testing.T) {
	app := NewAppWithSeed(42)
	eng, _ := app.mode.GetLeft()

	// Projection should have the developers (6 from DefaultDeveloperNames)
	sim := eng.Engine.Sim()
	if len(sim.Developers) != 6 {
		t.Errorf("Projection should have 6 developers, got %d", len(sim.Developers))
	}

	// Projection should have the backlog
	if len(sim.Backlog) != 12 {
		t.Errorf("Projection should have 12 tickets in backlog, got %d", len(sim.Backlog))
	}

	// First developer should be Mei (from DefaultDeveloperNames)
	if sim.Developers[0].Name != "Mei" {
		t.Errorf("First developer should be Mei, got %s", sim.Developers[0].Name)
	}
}

// TestTUI_ReceivesExternalEvents verifies the eventMsg handler applies external events
// to the local projection, updates office animations, and sets status messages.
func TestTUI_ReceivesExternalEvents(t *testing.T) {
	reg := registry.NewSimRegistry()
	app := NewAppWithRegistry(42, reg)

	// drainAndApply reads all buffered events from subscription and applies them via handler.
	// Returns the last event type seen, or "" if no events.
	// Channel is buffered (100), so events are available immediately after SetInstance.
	drainAndApply := func() string {
		eng, _ := app.mode.GetLeft()
		lastType := ""
		for {
			select {
			case evt := <-eng.EventSub:
				lastType = evt.EventType()
				newApp, _ := app.Update(eventMsg(evt))
				app = newApp.(*App)
				eng, _ = app.mode.GetLeft()
			default:
				return lastType
			}
		}
	}

	t.Run("SprintStarted", func(t *testing.T) {
		// Simulate API starting a sprint externally
		inst, ok := reg.GetInstanceOption("sim-42").Get()
		if !ok {
			t.Fatal("expected sim-42 in registry")
		}
		newEngine := must.Get(inst.Engine.StartSprint())
		inst.Engine = newEngine
		reg.SetInstance("sim-42", inst)

		if lastType := drainAndApply(); lastType == "" {
			t.Fatal("expected at least one event from StartSprint")
		}

		// Assert: TUI projection now reflects the sprint start
		eng, _ := app.mode.GetLeft()
		if _, ok := eng.Engine.Sim().CurrentSprintOption.Get(); !ok {
			t.Error("Expected TUI projection to show active sprint after external event")
		}

		// Assert: handler switched to execution view
		if app.currentView != ViewExecution {
			t.Errorf("Expected ViewExecution, got %v", app.currentView)
		}

		// Assert: status message indicates API origin
		if app.statusMessage != "Sprint started (via API)" {
			t.Errorf("Expected 'Sprint started (via API)', got %q", app.statusMessage)
		}
	})

	t.Run("SprintEnded", func(t *testing.T) {
		// Tick the engine externally until the sprint ends
		for i := 0; i < 100; i++ {
			inst, ok := reg.GetInstanceOption("sim-42").Get()
			if !ok {
				t.Fatal("expected sim-42 in registry")
			}
			newEngine, _, err := inst.Engine.Tick()
			if err != nil {
				t.Fatalf("Tick: %v", err)
			}
			inst.Engine = newEngine
			reg.SetInstance("sim-42", inst)

			lastType := drainAndApply()

			if lastType == "SprintEnded" {
				break
			}
		}

		// Assert: TUI projection shows sprint ended
		eng, _ := app.mode.GetLeft()
		if _, ok := eng.Engine.Sim().CurrentSprintOption.Get(); ok {
			t.Error("Expected sprint to be inactive after SprintEnded event")
		}

		// Assert: handler paused the simulation
		if !app.paused {
			t.Error("Expected app.paused=true after SprintEnded")
		}

		// Assert: status message indicates API origin
		if app.statusMessage != "Sprint complete (via API) — press 's' for next sprint" {
			t.Errorf("Expected sprint complete message, got %q", app.statusMessage)
		}
	})
}

// TestApp_UsesHTTPClient verifies app makes HTTP calls instead of direct engine access.
func TestApp_UsesHTTPClient(t *testing.T) {
	// Setup: Create test server
	reg := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(reg))
	defer srv.Close()

	client := NewClient(srv.URL)

	// Create simulation via HTTP
	createResp, err := client.CreateSimulation(42, "dora-strict")
	if err != nil {
		t.Fatalf("CreateSimulation failed: %v", err)
	}

	// Start sprint so tick is allowed - get updated state
	sprintResp, err := client.StartSprint(createResp.Simulation.ID)
	if err != nil {
		t.Fatalf("StartSprint failed: %v", err)
	}

	// Create app with HTTP client using state AFTER sprint started
	app := NewAppWithClient(client, sprintResp.Simulation)
	app.paused = false
	app.currentView = ViewExecution

	// Verify sprint is active in app state
	if !app.state.SprintActive {
		t.Fatal("Expected SprintActive to be true after StartSprint")
	}

	// Trigger tick via Update - should make HTTP call
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeySpace})

	// Verify a command was returned (the HTTP tick command)
	if cmd == nil {
		t.Fatal("Expected tick command to be returned")
	}

	// Execute the command to make the HTTP call
	msg := cmd()

	// Process the result
	app.Update(msg)

	// Verify state was updated from HTTP response
	if app.state.CurrentTick < 1 {
		t.Errorf("Expected CurrentTick >= 1 after tick, got %d", app.state.CurrentTick)
	}
}

// TestApp_DisablesWhileInFlight verifies spacebar is ignored during in-flight requests.
// RED: This test fails because app doesn't have inFlight field yet.
func TestApp_DisablesWhileInFlight(t *testing.T) {
	reg := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(reg))
	defer srv.Close()

	client := NewClient(srv.URL)
	createResp, err := client.CreateSimulation(42, "dora-strict")
	if err != nil {
		t.Fatalf("CreateSimulation failed: %v", err)
	}

	// Start sprint so tick would normally be allowed
	sprintResp, err := client.StartSprint(createResp.Simulation.ID)
	if err != nil {
		t.Fatalf("StartSprint failed: %v", err)
	}

	app := NewAppWithClient(client, sprintResp.Simulation)
	app.paused = false
	app.currentView = ViewExecution

	// Set inFlight to true - simulating a request in progress
	app.inFlight = true

	// Trigger spacebar - should be ignored because inFlight
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeySpace})

	// Verify no command was returned (request blocked)
	if cmd != nil {
		t.Error("Expected no command when inFlight is true")
	}
}

// TestApp_HasClientMode verifies app is in client mode when created with NewAppWithClient.
func TestApp_HasClientMode(t *testing.T) {
	reg := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(reg))
	defer srv.Close()

	client := NewClient(srv.URL)
	createResp, err := client.CreateSimulation(42, "dora-strict")
	if err != nil {
		t.Fatalf("CreateSimulation failed: %v", err)
	}

	app := NewAppWithClient(client, createResp.Simulation)

	// App should be in client mode (Right variant)
	cli, isClient := app.mode.Get()
	if !isClient {
		t.Error("Expected app to be in client mode")
	}

	// App should have client in mode (check internal httpClient is set)
	if cli.Client.httpClient == nil {
		t.Error("Expected cli.Client.httpClient to be non-nil")
	}

	// App should have state field (SimulationState from HTTP)
	if app.state.ID == "" {
		t.Error("Expected app.state.ID to be set")
	}

	// App should have simID in mode
	if cli.SimID == "" {
		t.Error("Expected cli.SimID to be non-empty")
	}
}

// TestApp_UC19TriggerIntegration verifies trigger wiring between app.go and lessons package.
// This tests the full integration: client mode state → trigger detection → lesson selection.
func TestApp_UC19TriggerIntegration(t *testing.T) {
	reg := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(reg))
	defer srv.Close()

	client := NewClient(srv.URL)
	createResp, err := client.CreateSimulation(42, "dora-strict")
	if err != nil {
		t.Fatalf("CreateSimulation failed: %v", err)
	}

	app := NewAppWithClient(client, createResp.Simulation)
	app.lessonState = app.lessonState.WithVisible(true)
	app.lessonState = app.lessonState.WithSeen(LessonOrientation) // Past orientation

	t.Run("UC19 triggers on red buffer with LOW ticket", func(t *testing.T) {
		// Setup: simulate red buffer (>66% consumed) with LOW understanding ticket
		app.state.SprintOption = option.Of(SprintState{
			BufferDays:     3.0,
			BufferConsumed: 2.5, // 83% consumed = red
		})
		app.state.ActiveTickets = []TicketState{
			{Understanding: "LOW"},
		}

		// Uses same BuildTriggersFromClientState as app.go View()
		triggers := BuildTriggersFromClientState(app.state)
		lesson := SelectLesson(ViewExecution, app.lessonState, true, false, triggers, ComparisonSummary{})

		if lesson.ID != LessonUncertaintyConstraint {
			t.Errorf("Expected UncertaintyConstraint lesson, got %s", lesson.ID)
		}
	})

	t.Run("UC19 does not trigger when buffer is green", func(t *testing.T) {
		app.state.SprintOption = option.Of(SprintState{
			BufferDays:     3.0,
			BufferConsumed: 0.5, // 17% consumed = green
		})
		app.state.ActiveTickets = []TicketState{
			{Understanding: "LOW"},
		}

		triggers := BuildTriggersFromClientState(app.state)
		lesson := SelectLesson(ViewExecution, app.lessonState, true, false, triggers, ComparisonSummary{})

		if lesson.ID == LessonUncertaintyConstraint {
			t.Error("Should not trigger UncertaintyConstraint when buffer is green")
		}
	})

	t.Run("UC19 does not trigger without LOW ticket", func(t *testing.T) {
		app.state.SprintOption = option.Of(SprintState{
			BufferDays:     3.0,
			BufferConsumed: 2.5, // red
		})
		app.state.ActiveTickets = []TicketState{
			{Understanding: "HIGH"},
		}

		triggers := BuildTriggersFromClientState(app.state)
		lesson := SelectLesson(ViewExecution, app.lessonState, true, false, triggers, ComparisonSummary{})

		if lesson.ID == LessonUncertaintyConstraint {
			t.Error("Should not trigger UncertaintyConstraint without LOW ticket")
		}
	})
}

// TestApp_UC20TriggerIntegration verifies UC20 (ConstraintHunt) wiring.
func TestApp_UC20TriggerIntegration(t *testing.T) {
	reg := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(reg))
	defer srv.Close()

	client := NewClient(srv.URL)
	createResp, err := client.CreateSimulation(42, "dora-strict")
	if err != nil {
		t.Fatalf("CreateSimulation failed: %v", err)
	}

	app := NewAppWithClient(client, createResp.Simulation)
	app.lessonState = app.lessonState.WithVisible(true)
	app.lessonState = app.lessonState.WithSeen(LessonOrientation)
	app.lessonState = app.lessonState.WithSeen(LessonUncertaintyConstraint) // UC19 prerequisite

	t.Run("UC20 triggers on queue imbalance", func(t *testing.T) {
		// 5 implement + 1 verify + 1 cicd = 7 total, 3 phases
		// avg = 7/3 = 2.33, 2*avg = 4.66, implement(5) > 4.66 ✓
		app.state.ActiveTickets = []TicketState{
			{Phase: "implement"}, {Phase: "implement"}, {Phase: "implement"},
			{Phase: "implement"}, {Phase: "implement"},
			{Phase: "verify"},
			{Phase: "cicd"},
		}

		triggers := BuildTriggersFromClientState(app.state)
		lesson := SelectLesson(ViewExecution, app.lessonState, true, false, triggers, ComparisonSummary{})

		if lesson.ID != LessonConstraintHunt {
			t.Errorf("Expected ConstraintHunt lesson, got %s", lesson.ID)
		}
	})

	t.Run("UC20 does not trigger without UC19 seen", func(t *testing.T) {
		// Fresh app without UC19 seen
		freshApp := NewAppWithClient(client, createResp.Simulation)
		freshApp.lessonState = freshApp.lessonState.WithVisible(true)
		freshApp.lessonState = freshApp.lessonState.WithSeen(LessonOrientation)
		// NOT seen: LessonUncertaintyConstraint

		freshApp.state.ActiveTickets = []TicketState{
			{Phase: "implement"}, {Phase: "implement"}, {Phase: "implement"},
			{Phase: "implement"}, {Phase: "implement"},
			{Phase: "verify"},
			{Phase: "cicd"},
		}

		triggers := BuildTriggersFromClientState(freshApp.state)
		lesson := SelectLesson(ViewExecution, freshApp.lessonState, true, false, triggers, ComparisonSummary{})

		if lesson.ID == LessonConstraintHunt {
			t.Error("Should not trigger ConstraintHunt without UC19 seen")
		}
	})
}

// TestApp_RecordsInputEvents_ClientMode verifies HTTP result handler records events in client mode.
func TestApp_RecordsInputEvents_ClientMode(t *testing.T) {
	reg := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(reg))
	defer srv.Close()

	client := NewClient(srv.URL)
	createResp, err := client.CreateSimulation(42, "dora-strict")
	if err != nil {
		t.Fatalf("CreateSimulation failed: %v", err)
	}

	app := NewAppWithClient(client, createResp.Simulation)
	app.currentView = ViewPlanning

	// Press s to start sprint (triggers HTTP call)
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})

	// Execute the HTTP command and process result
	if cmd != nil {
		msg := cmd()
		app.Update(msg)
	}

	// UIProjection should record successful sprint start
	uiState := app.uiProjection.State()
	if uiState.ErrorMessage != "" {
		t.Errorf("After successful HTTP sprint start: ErrorMessage = %q, want empty", uiState.ErrorMessage)
	}
}

// TestApp_UC21TriggerIntegration verifies UC21 (ExploitFirst) wiring.
func TestApp_UC21TriggerIntegration(t *testing.T) {
	reg := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(reg))
	defer srv.Close()

	client := NewClient(srv.URL)
	createResp, err := client.CreateSimulation(42, "dora-strict")
	if err != nil {
		t.Fatalf("CreateSimulation failed: %v", err)
	}

	app := NewAppWithClient(client, createResp.Simulation)
	app.lessonState = app.lessonState.WithVisible(true)
	app.lessonState = app.lessonState.WithSeen(LessonOrientation)
	app.lessonState = app.lessonState.WithSeen(LessonUncertaintyConstraint) // UC19 prerequisite

	t.Run("UC21 triggers on high child variance", func(t *testing.T) {
		// Parent with child that has ratio 1.5 (> 1.3 threshold)
		app.state.CompletedTickets = []TicketState{
			{ID: "parent", ChildIDs: []string{"child"}},
			{ID: "child", EstimatedDays: 2.0, ActualDays: 3.0}, // ratio 1.5
		}

		triggers := BuildTriggersFromClientState(app.state)
		lesson := SelectLesson(ViewExecution, app.lessonState, true, false, triggers, ComparisonSummary{})

		if lesson.ID != LessonExploitFirst {
			t.Errorf("Expected ExploitFirst lesson, got %s", lesson.ID)
		}
	})

	t.Run("UC21 does not trigger without UC19 seen", func(t *testing.T) {
		freshApp := NewAppWithClient(client, createResp.Simulation)
		freshApp.lessonState = freshApp.lessonState.WithVisible(true)
		freshApp.lessonState = freshApp.lessonState.WithSeen(LessonOrientation)
		// NOT seen: LessonUncertaintyConstraint

		freshApp.state.CompletedTickets = []TicketState{
			{ID: "parent", ChildIDs: []string{"child"}},
			{ID: "child", EstimatedDays: 2.0, ActualDays: 3.0},
		}

		triggers := BuildTriggersFromClientState(freshApp.state)
		lesson := SelectLesson(ViewExecution, freshApp.lessonState, true, false, triggers, ComparisonSummary{})

		if lesson.ID == LessonExploitFirst {
			t.Error("Should not trigger ExploitFirst without UC19 seen")
		}
	})
}

// TestApp_UC22TriggerIntegration verifies UC22 (FiveFocusing) wiring.
func TestApp_UC22TriggerIntegration(t *testing.T) {
	reg := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(reg))
	defer srv.Close()

	client := NewClient(srv.URL)
	createResp, err := client.CreateSimulation(42, "dora-strict")
	if err != nil {
		t.Fatalf("CreateSimulation failed: %v", err)
	}

	app := NewAppWithClient(client, createResp.Simulation)
	app.lessonState = app.lessonState.WithVisible(true)
	app.lessonState = app.lessonState.WithSeen(LessonOrientation)
	app.lessonState = app.lessonState.WithSeen(LessonUncertaintyConstraint) // UC19
	app.lessonState = app.lessonState.WithSeen(LessonConstraintHunt)        // UC20 prereq

	t.Run("UC22 triggers after 3 sprints with UC20 seen", func(t *testing.T) {
		// SprintCount >= 3 via TriggerState (simulated)
		triggers := TriggerState{SprintCount: 3}
		lesson := SelectLesson(ViewExecution, app.lessonState, true, false, triggers, ComparisonSummary{})

		if lesson.ID != LessonFiveFocusing {
			t.Errorf("Expected FiveFocusing lesson, got %s", lesson.ID)
		}
	})

	t.Run("UC22 triggers with UC21 instead of UC20", func(t *testing.T) {
		// Alternative prereq: ExploitFirst instead of ConstraintHunt
		altApp := NewAppWithClient(client, createResp.Simulation)
		altApp.lessonState = altApp.lessonState.WithVisible(true)
		altApp.lessonState = altApp.lessonState.WithSeen(LessonOrientation)
		altApp.lessonState = altApp.lessonState.WithSeen(LessonUncertaintyConstraint)
		altApp.lessonState = altApp.lessonState.WithSeen(LessonExploitFirst) // UC21 instead of UC20

		triggers := TriggerState{SprintCount: 3}
		lesson := SelectLesson(ViewExecution, altApp.lessonState, true, false, triggers, ComparisonSummary{})

		if lesson.ID != LessonFiveFocusing {
			t.Errorf("Expected FiveFocusing lesson, got %s", lesson.ID)
		}
	})

	t.Run("UC22 does not trigger without UC20/UC21 seen", func(t *testing.T) {
		freshApp := NewAppWithClient(client, createResp.Simulation)
		freshApp.lessonState = freshApp.lessonState.WithVisible(true)
		freshApp.lessonState = freshApp.lessonState.WithSeen(LessonOrientation)
		freshApp.lessonState = freshApp.lessonState.WithSeen(LessonUncertaintyConstraint)
		// NOT seen: ConstraintHunt OR ExploitFirst

		triggers := TriggerState{SprintCount: 3}
		lesson := SelectLesson(ViewExecution, freshApp.lessonState, true, false, triggers, ComparisonSummary{})

		if lesson.ID == LessonFiveFocusing {
			t.Error("Should not trigger FiveFocusing without UC20 or UC21 seen")
		}
	})

	t.Run("UC22 does not trigger with < 3 sprints", func(t *testing.T) {
		triggers := TriggerState{SprintCount: 2} // Not enough sprints
		lesson := SelectLesson(ViewExecution, app.lessonState, true, false, triggers, ComparisonSummary{})

		if lesson.ID == LessonFiveFocusing {
			t.Error("Should not trigger FiveFocusing with only 2 sprints")
		}
	})
}

// TestApp_UC23TriggerIntegration verifies UC23 (ManagerTakeaways) wiring.
func TestApp_UC23TriggerIntegration(t *testing.T) {
	reg := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(reg))
	defer srv.Close()

	client := NewClient(srv.URL)
	createResp, err := client.CreateSimulation(42, "dora-strict")
	if err != nil {
		t.Fatalf("CreateSimulation failed: %v", err)
	}

	app := NewAppWithClient(client, createResp.Simulation)
	app.lessonState = app.lessonState.WithVisible(true)
	app.lessonState = app.lessonState.WithSeen(LessonOrientation)
	app.lessonState = app.lessonState.WithSeen(LessonFiveFocusing) // UC22 prereq

	t.Run("UC23 triggers in Comparison view with result and UC22 seen", func(t *testing.T) {
		comparison := ComparisonSummary{
			HasResult:    true,
			WinnerPolicy: "TameFlow-Cognitive",
			LeadTimeA:    5.0,
			LeadTimeB:    3.5,
		}

		lesson := SelectLesson(ViewComparison, app.lessonState, false, true, TriggerState{}, comparison)

		if lesson.ID != LessonManagerTakeaways {
			t.Errorf("Expected ManagerTakeaways lesson, got %s", lesson.ID)
		}
	})

	t.Run("UC23 does not trigger without UC22 seen", func(t *testing.T) {
		freshApp := NewAppWithClient(client, createResp.Simulation)
		freshApp.lessonState = freshApp.lessonState.WithVisible(true)
		freshApp.lessonState = freshApp.lessonState.WithSeen(LessonOrientation)
		// NOT seen: FiveFocusing

		comparison := ComparisonSummary{HasResult: true, WinnerPolicy: "TameFlow-Cognitive"}
		lesson := SelectLesson(ViewComparison, freshApp.lessonState, false, true, TriggerState{}, comparison)

		if lesson.ID == LessonManagerTakeaways {
			t.Error("Should not trigger ManagerTakeaways without UC22 seen")
		}
	})

	t.Run("UC23 does not trigger without comparison result", func(t *testing.T) {
		comparison := ComparisonSummary{HasResult: false} // No result yet
		lesson := SelectLesson(ViewComparison, app.lessonState, false, false, TriggerState{}, comparison)

		if lesson.ID == LessonManagerTakeaways {
			t.Error("Should not trigger ManagerTakeaways without comparison result")
		}
	})

	t.Run("UC23 does not trigger outside Comparison view", func(t *testing.T) {
		comparison := ComparisonSummary{HasResult: true, WinnerPolicy: "TameFlow-Cognitive"}
		lesson := SelectLesson(ViewExecution, app.lessonState, false, true, TriggerState{}, comparison)

		if lesson.ID == LessonManagerTakeaways {
			t.Error("Should not trigger ManagerTakeaways outside Comparison view")
		}
	})
}

// TestApp_FullSessionWalkthrough simulates a complete TUI session via UIProjection.
// This replaces manual testing by programmatically verifying key→event→state flow.
func TestApp_FullSessionWalkthrough(t *testing.T) {
	app := NewAppWithSeed(42)

	// Helper to send key
	sendKey := func(key string) {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	}
	sendTab := func() {
		app.Update(tea.KeyMsg{Type: tea.KeyTab})
	}

	// 1. INITIAL STATE
	state := app.uiProjection.State()
	if state.CurrentView != ViewPlanning {
		t.Errorf("Initial: CurrentView = %v, want ViewPlanning", state.CurrentView)
	}
	if state.LessonVisible {
		t.Error("Initial: LessonVisible should be false")
	}

	// 2. NAVIGATION - cycle through views
	sendTab() // → Execution
	if app.uiProjection.State().CurrentView != ViewExecution {
		t.Error("After Tab 1: want ViewExecution")
	}
	sendTab() // → Metrics
	sendTab() // → Comparison
	sendTab() // → Planning
	if app.uiProjection.State().CurrentView != ViewPlanning {
		t.Error("After Tab 4: want ViewPlanning (cycle)")
	}

	// 3. TICKET SELECTION
	eng, _ := app.mode.GetLeft()
	sim := eng.Engine.Sim()
	sendKey("j") // select second ticket
	sendKey("j") // select third ticket
	state = app.uiProjection.State()
	if state.SelectedTicket != sim.Backlog[2].ID {
		t.Errorf("After j j: SelectedTicket = %q, want %q", state.SelectedTicket, sim.Backlog[2].ID)
	}
	sendKey("k") // back to second
	state = app.uiProjection.State()
	if state.SelectedTicket != sim.Backlog[1].ID {
		t.Errorf("After k: SelectedTicket = %q, want %q", state.SelectedTicket, sim.Backlog[1].ID)
	}

	// 4. LESSONS PANEL
	sendKey("h") // toggle on
	if !app.uiProjection.State().LessonVisible {
		t.Error("After h: LessonVisible should be true")
	}
	sendKey("h") // toggle off
	if app.uiProjection.State().LessonVisible {
		t.Error("After h h: LessonVisible should be false")
	}

	// 5. ASSIGNMENT
	sendKey("k") // back to first ticket (from second after step 3)
	sendKey("a") // assign
	state = app.uiProjection.State()
	if state.ErrorMessage != "" {
		t.Errorf("After assign: ErrorMessage = %q, want empty", state.ErrorMessage)
	}
	if state.SelectedTicket != "" {
		t.Errorf("After assign: SelectedTicket = %q, want empty (cleared on success)", state.SelectedTicket)
	}

	// 6. START SPRINT
	sendKey("s")
	state = app.uiProjection.State()
	if state.ErrorMessage != "" {
		t.Errorf("After sprint start: ErrorMessage = %q, want empty", state.ErrorMessage)
	}

	// 7. TRY SPRINT AGAIN (should fail)
	app.currentView = ViewPlanning // force back for test
	sendKey("s")
	state = app.uiProjection.State()
	if state.ErrorMessage == "" {
		t.Error("After second sprint start: ErrorMessage should be set (already active)")
	}

	// 8. VIEW SWITCH CLEARS ERROR
	sendTab()
	state = app.uiProjection.State()
	if state.ErrorMessage != "" {
		t.Errorf("After Tab: ErrorMessage = %q, want empty (cleared on navigation)", state.ErrorMessage)
	}

	// 9. RUN SPRINT TO COMPLETION
	app.currentView = ViewExecution
	app.paused = false
	for i := 0; i < 11; i++ { // 10 days + 1 to trigger end
		app.Update(tickMsg(time.Now()))
	}
	if !app.paused {
		t.Error("Should be paused after sprint ends")
	}

	// 10. VERIFY METRICS VIEW
	sendTab() // → Metrics (from Execution)
	state = app.uiProjection.State()
	if state.CurrentView != ViewMetrics {
		t.Errorf("After Tab: want ViewMetrics, got %v", state.CurrentView)
	}
	// Verify sprint completed
	eng, _ = app.mode.GetLeft()
	sim = eng.Engine.Sim()
	if sim.SprintNumber < 1 {
		t.Error("Sprint should have completed (SprintNumber >= 1)")
	}

	t.Logf("Session walkthrough complete: %d events recorded", len(app.uiProjection.events))
}

// TestWorkflow_SprintCycle_ClientMode verifies complete sprint cycle via HTTP backend.
func TestWorkflow_SprintCycle_ClientMode(t *testing.T) {
	// Setup test server
	reg := api.NewSimRegistry()
	srv := httptest.NewServer(api.NewRouter(reg))
	defer srv.Close()

	client := NewClient(srv.URL)
	createResp, err := client.CreateSimulation(42, "dora-strict")
	if err != nil {
		t.Fatalf("CreateSimulation failed: %v", err)
	}

	app := NewAppWithClient(client, createResp.Simulation)
	app.currentView = ViewPlanning

	// Start sprint via key
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if cmd != nil {
		msg := cmd()
		app.Update(msg)
	}

	// Verify sprint started
	if app.uiProjection.State().ErrorMessage != "" {
		t.Errorf("Sprint start should succeed, got error: %s", app.uiProjection.State().ErrorMessage)
	}

	// Run ticks via Space (client mode)
	app.currentView = ViewExecution
	app.paused = false
	for i := 0; i < 11; i++ {
		_, cmd := app.Update(tea.KeyMsg{Type: tea.KeySpace})
		if cmd != nil {
			msg := cmd()
			app.Update(msg)
		}
	}

	// Verify completion (client mode pauses when sprint ends)
	if !app.paused {
		t.Error("Should pause after sprint ends")
	}

	// Verify metrics view accessible (parity with engine mode)
	app.Update(tea.KeyMsg{Type: tea.KeyTab}) // → Metrics
	if app.uiProjection.State().CurrentView != ViewMetrics {
		t.Errorf("After Tab: want ViewMetrics, got %v", app.uiProjection.State().CurrentView)
	}
	// Verify sprint data in state (client mode uses app.state)
	if app.state.SprintNumber < 1 {
		t.Error("Sprint should have completed (SprintNumber >= 1)")
	}
}

// TestWorkflow_PolicyComparison verifies comparison workflow runs and produces results.
func TestWorkflow_PolicyComparison(t *testing.T) {
	app := NewAppWithSeed(42)

	// Navigate to Comparison view (Tab×3: Planning → Execution → Metrics → Comparison)
	for i := 0; i < 3; i++ {
		app.Update(tea.KeyMsg{Type: tea.KeyTab})
	}
	if app.uiProjection.State().CurrentView != ViewComparison {
		t.Fatalf("Want ViewComparison, got %v", app.uiProjection.State().CurrentView)
	}

	// Press 'c' to run comparison
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})

	// Verify comparison populated
	result, ok := app.comparisonResult.Get()
	if !ok {
		t.Fatal("ComparisonResult should be populated after pressing 'c'")
	}
	// LeadTimeWinner should be one of the policies (not default zero)
	if result.LeadTimeWinner == 0 && result.PolicyA != 0 {
		t.Error("ComparisonResult.LeadTimeWinner should be set to a policy")
	}
	if result.ResultsA.FinalMetrics.LeadTimeAvg == 0 {
		t.Error("ResultsA.FinalMetrics should be populated")
	}
	if result.ResultsB.FinalMetrics.LeadTimeAvg == 0 {
		t.Error("ResultsB.FinalMetrics should be populated")
	}
}

// TestTUI_SyncsOfficeToRegistry verifies TUI office events sync to registry.
// UC35: Claude queries /office endpoint to see TUI animation state.
func TestTUI_SyncsOfficeToRegistry(t *testing.T) {
	reg := registry.NewSimRegistry()
	app := NewAppWithRegistry(42, reg)

	// Start sprint and assign ticket
	eng, _ := app.mode.GetLeft()
	eng.Engine = must.Get(eng.Engine.StartSprint())
	app.mode = either.Left[EngineMode, ClientMode](eng)
	app.currentView = ViewPlanning
	app.selected = 0

	// Press 'a' to assign ticket - this should record DevAssignedToTicket
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})

	// Registry office projection should have the animation state
	inst, ok := reg.GetInstanceOption("sim-42").Get()
	if !ok {
		t.Fatal("Registry should contain TUI's simulation")
	}

	// Office projection should have developer animation (not empty)
	officeState := inst.Office.State()
	if len(officeState.Animations) == 0 {
		t.Error("Registry office should have developer animations after assignment")
	}

	// Check the assigned developer has working/moving state (not idle)
	devAnim, hasAnim := officeState.GetAnimationOption("dev-1").Get()
	if !hasAnim {
		t.Fatal("Expected dev-1 to have animation state")
	}
	if devAnim.State == office.StateIdle {
		t.Errorf("Expected dev-1 state to change from Idle after assignment, got %v", devAnim.State)
	}
}

// TestTUI_SyncsOfficeOnTick verifies office state syncs during tick processing.
// Tests that transitions recorded during tick are synced to registry.
func TestTUI_SyncsOfficeOnTick(t *testing.T) {
	reg := registry.NewSimRegistry()
	app := NewAppWithRegistry(42, reg)

	// Start sprint and assign ticket
	eng, _ := app.mode.GetLeft()
	eng.Engine = must.Get(eng.Engine.StartSprint())
	app.mode = either.Left[EngineMode, ClientMode](eng)
	app.currentView = ViewPlanning
	app.selected = 0

	// Assign ticket - this syncs initial state
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})

	// Get initial transition count from registry
	inst, _ := reg.GetInstanceOption("sim-42").Get()
	initialTransitions := len(inst.Office.Transitions())

	// Switch to execution view and run ticks
	app.currentView = ViewExecution
	app.paused = false

	// Run several ticks - should trigger DevStartedWorking and sync
	for i := 0; i < 5; i++ {
		app.Update(tickMsg(time.Now()))
	}

	// Check registry office was updated with new transitions
	inst, ok := reg.GetInstanceOption("sim-42").Get()
	if !ok {
		t.Fatal("Registry should contain TUI's simulation")
	}

	// Transitions should have increased (DevStartedWorking at minimum)
	newTransitions := len(inst.Office.Transitions())
	if newTransitions <= initialTransitions {
		t.Errorf("Expected transitions to increase after ticks, got %d (was %d)", newTransitions, initialTransitions)
	}
}

// TestTUI_SyncsOfficeOnSprintEnd verifies DevEnteredConference syncs when sprint ends.
func TestTUI_SyncsOfficeOnSprintEnd(t *testing.T) {
	reg := registry.NewSimRegistry()
	app := NewAppWithRegistry(42, reg)

	// Start sprint and assign ticket
	eng, _ := app.mode.GetLeft()
	eng.Engine = must.Get(eng.Engine.StartSprint())
	app.mode = either.Left[EngineMode, ClientMode](eng)
	app.currentView = ViewPlanning
	app.selected = 0

	// Assign ticket
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})

	// Switch to execution view
	app.currentView = ViewExecution
	app.paused = false

	// Run ticks until sprint ends (10 day sprint)
	for i := 0; i < 15; i++ {
		app.Update(tickMsg(time.Now()))
		if app.paused {
			break // Sprint ended
		}
	}

	// Check registry office has DevEnteredConference transition
	inst, ok := reg.GetInstanceOption("sim-42").Get()
	if !ok {
		t.Fatal("Registry should contain TUI's simulation")
	}

	// Find a conference transition
	hasConferenceTransition := false
	for _, tr := range inst.Office.Transitions() {
		if tr.ToState == "conference" {
			hasConferenceTransition = true
			break
		}
	}

	if !hasConferenceTransition {
		t.Error("Expected DevEnteredConference transition when sprint ends")
	}
}

func TestTUI_WindowResizeSyncsOfficeSize(t *testing.T) {
	reg := registry.NewSimRegistry()
	app := NewAppWithRegistry(42, reg)
	app.currentView = ViewExecution

	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	w, h := reg.OfficeSize()
	if w != 120 {
		t.Errorf("OfficeSize width = %d, want 120", w)
	}
	if h != 40 {
		t.Errorf("OfficeSize height = %d, want 40", h)
	}
}

func TestTUI_TabSyncsOfficeSizeForNewView(t *testing.T) {
	reg := registry.NewSimRegistry()
	app := NewAppWithRegistry(42, reg)
	app.width = 200
	app.height = 50
	app.currentView = ViewPlanning

	// Tab → Execution (full width)
	app.Update(tea.KeyMsg{Type: tea.KeyTab})

	w, _ := reg.OfficeSize()
	if w != 200 {
		t.Errorf("After Tab to execution: OfficeSize width = %d, want 200", w)
	}
}

func TestTUI_HTTPOfficeUsesRegistryDimensions(t *testing.T) {
	reg := api.NewSimRegistry()
	_ = NewAppWithRegistry(42, reg.SimRegistry)

	reg.UpdateOfficeSize(60, 30)

	srv := httptest.NewServer(api.NewRouter(reg))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/simulations/sim-42/office")
	if err != nil {
		t.Fatalf("GET /office failed: %v", err)
	}
	defer resp.Body.Close()
	var result struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Width != 60 || result.Height != 30 {
		t.Errorf("Office dimensions = (%d, %d), want (60, 30)", result.Width, result.Height)
	}
}

func TestTUI_OfficeSizeSyncLifecycle(t *testing.T) {
	reg := api.NewSimRegistry()
	app := NewAppWithRegistry(42, reg.SimRegistry)

	// 1. Before WindowSizeMsg: registry should have defaults (80×24)
	w, h := reg.OfficeSize()
	if w != 80 || h != 24 {
		t.Errorf("Before resize: OfficeSize = (%d, %d), want (80, 24)", w, h)
	}

	// 2. WindowSizeMsg arrives (Bubble Tea sends this on startup)
	// NewAppWithRegistry does NOT call syncOfficeToRegistry, so defaults until here.
	app.Update(tea.WindowSizeMsg{Width: 160, Height: 45})
	w, _ = reg.OfficeSize()
	// Planning view (default): 160 * 40/100 = 64
	if w != 64 {
		t.Errorf("After resize in planning: width = %d, want 64", w)
	}

	// 3. Tab to Execution — full width
	app.Update(tea.KeyMsg{Type: tea.KeyTab})
	w, _ = reg.OfficeSize()
	if w != 160 {
		t.Errorf("After tab to execution: width = %d, want 160", w)
	}

	// 4. Terminal resize while in Execution
	app.Update(tea.WindowSizeMsg{Width: 60, Height: 30})
	w, h = reg.OfficeSize()
	if w != 60 || h != 30 {
		t.Errorf("After resize in execution: = (%d, %d), want (60, 30)", w, h)
	}

	// 5. Tab to Metrics — pass-through width
	app.Update(tea.KeyMsg{Type: tea.KeyTab})
	w, _ = reg.OfficeSize()
	if w != 60 {
		t.Errorf("After tab to metrics: width = %d, want 60", w)
	}

	// 6. Tab to Comparison, then back to Planning
	app.Update(tea.KeyMsg{Type: tea.KeyTab}) // → Comparison
	app.Update(tea.KeyMsg{Type: tea.KeyTab}) // → Planning
	w, _ = reg.OfficeSize()
	// Planning: 60 * 40/100 = 24, clamped to 40
	if w != 40 {
		t.Errorf("After tab to planning (narrow): width = %d, want 40", w)
	}

	// 7. API sees the same dimensions
	srv := httptest.NewServer(api.NewRouter(reg))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/simulations/sim-42/office")
	if err != nil {
		t.Fatalf("GET /office failed: %v", err)
	}
	defer resp.Body.Close()
	var result struct{ Width int `json:"width"` }
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Width != 40 {
		t.Errorf("API width = %d, want 40", result.Width)
	}
}

func TestComputeOfficeWidth(t *testing.T) {
	tests := []struct {
		name          string
		view          View
		terminalWidth int
		want          int
	}{
		{"planning 200px", ViewPlanning, 200, 80},
		{"planning 80px", ViewPlanning, 80, 40},
		{"planning 50px", ViewPlanning, 50, 40},
		{"execution full", ViewExecution, 200, 200},
		{"execution narrow", ViewExecution, 60, 60},
		{"metrics passthrough", ViewMetrics, 120, 120},
		{"comparison passthrough", ViewComparison, 120, 120},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeOfficeWidth(tt.view, tt.terminalWidth)
			if got != tt.want {
				t.Errorf("computeOfficeWidth(%v, %d) = %d, want %d",
					tt.view, tt.terminalWidth, got, tt.want)
			}
		})
	}
}
