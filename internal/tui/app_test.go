package tui

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/binaryphile/fluentfp/either"
	"github.com/binaryphile/fluentfp/must"
	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/sofdevsim-2026/internal/api"
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

// TestNewAppWithSeed_ProjectionHasInitialState verifies projection has devs and tickets.
func TestNewAppWithSeed_ProjectionHasInitialState(t *testing.T) {
	app := NewAppWithSeed(42)
	eng, _ := app.mode.GetLeft()

	// Projection should have the developers
	sim := eng.Engine.Sim()
	if len(sim.Developers) != 3 {
		t.Errorf("Projection should have 3 developers, got %d", len(sim.Developers))
	}

	// Projection should have the backlog
	if len(sim.Backlog) != 12 {
		t.Errorf("Projection should have 12 tickets in backlog, got %d", len(sim.Backlog))
	}

	// First developer should be Alice
	if sim.Developers[0].Name != "Alice" {
		t.Errorf("First developer should be Alice, got %s", sim.Developers[0].Name)
	}
}

// TestTUI_ReceivesExternalEvents verifies TUI receives events from API actions.
func TestTUI_ReceivesExternalEvents(t *testing.T) {
	reg := registry.NewSimRegistry()
	app := NewAppWithRegistry(42, reg)
	eng, _ := app.mode.GetLeft()

	// Simulate API starting a sprint (external to TUI)
	// This goes through the same engine, which emits to shared store
	eng.Engine.StartSprint()

	// TUI should receive the event via subscription
	select {
	case evt := <-eng.EventSub:
		if evt.EventType() != "SprintStarted" {
			t.Errorf("Expected SprintStarted event, got %s", evt.EventType())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timed out waiting for SprintStarted event")
	}
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

// TestApp_RecordsViewSwitched_EngineMode verifies tab key records ViewSwitched event.
func TestApp_RecordsViewSwitched_EngineMode(t *testing.T) {
	app := NewAppWithSeed(42)

	// Initial state should be ViewPlanning
	uiState := app.uiProjection.State()
	if uiState.CurrentView != ViewPlanning {
		t.Errorf("Initial CurrentView = %v, want ViewPlanning", uiState.CurrentView)
	}

	// Press tab (KeyTab type, not runes)
	app.Update(tea.KeyMsg{Type: tea.KeyTab})

	// UIProjection should record ViewSwitched
	uiState = app.uiProjection.State()
	if uiState.CurrentView != ViewExecution {
		t.Errorf("After tab: CurrentView = %v, want ViewExecution", uiState.CurrentView)
	}
}

// TestApp_RecordsLessonPanelToggled_EngineMode verifies h key records LessonPanelToggled event.
func TestApp_RecordsLessonPanelToggled_EngineMode(t *testing.T) {
	app := NewAppWithSeed(42)

	// Initially not visible in projection
	if app.uiProjection.State().LessonVisible {
		t.Error("Initial LessonVisible should be false")
	}

	// Press h
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})

	// UIProjection should record toggle
	if !app.uiProjection.State().LessonVisible {
		t.Error("After h: LessonVisible should be true")
	}
}

// TestApp_RecordsTicketSelected_EngineMode verifies j/k keys record TicketSelected event.
func TestApp_RecordsTicketSelected_EngineMode(t *testing.T) {
	app := NewAppWithSeed(42)
	eng, _ := app.mode.GetLeft()
	sim := eng.Engine.Sim()

	// Initial selection is 0
	if app.selected != 0 {
		t.Fatalf("Initial selected = %d, want 0", app.selected)
	}

	// Press j (down)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})

	// UIProjection should have second ticket selected
	uiState := app.uiProjection.State()
	expectedID := sim.Backlog[1].ID
	if uiState.SelectedTicket != expectedID {
		t.Errorf("After j: SelectedTicket = %q, want %q", uiState.SelectedTicket, expectedID)
	}

	// Press k (up)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})

	// UIProjection should have first ticket selected
	uiState = app.uiProjection.State()
	expectedID = sim.Backlog[0].ID
	if uiState.SelectedTicket != expectedID {
		t.Errorf("After k: SelectedTicket = %q, want %q", uiState.SelectedTicket, expectedID)
	}
}

// TestApp_RecordsSprintStartAttempted_Success_EngineMode verifies s key records SprintStartAttempted on success.
func TestApp_RecordsSprintStartAttempted_Success_EngineMode(t *testing.T) {
	app := NewAppWithSeed(42)
	app.currentView = ViewPlanning

	// Press s to start sprint
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})

	// UIProjection should show no error (success clears error)
	uiState := app.uiProjection.State()
	if uiState.ErrorMessage != "" {
		t.Errorf("After successful sprint start: ErrorMessage = %q, want empty", uiState.ErrorMessage)
	}
}

// TestApp_RecordsSprintStartAttempted_Failure_EngineMode verifies s key records failure when sprint already active.
func TestApp_RecordsSprintStartAttempted_Failure_EngineMode(t *testing.T) {
	app := NewAppWithSeed(42)
	app.currentView = ViewPlanning

	// Start a sprint first
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})

	// Go back to planning view
	app.currentView = ViewPlanning

	// Try to start another sprint (should fail)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})

	// UIProjection should have error message
	uiState := app.uiProjection.State()
	if uiState.ErrorMessage == "" {
		t.Error("After failed sprint start: ErrorMessage should not be empty")
	}
}

// TestApp_RecordsAssignmentAttempted_Success_EngineMode verifies a key records AssignmentAttempted on success.
func TestApp_RecordsAssignmentAttempted_Success_EngineMode(t *testing.T) {
	app := NewAppWithSeed(42)
	app.currentView = ViewPlanning
	app.selected = 0

	// Press a to assign
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})

	// UIProjection should show no error (success clears error) and clear selection
	uiState := app.uiProjection.State()
	if uiState.ErrorMessage != "" {
		t.Errorf("After successful assignment: ErrorMessage = %q, want empty", uiState.ErrorMessage)
	}
	if uiState.SelectedTicket != "" {
		t.Errorf("After successful assignment: SelectedTicket = %q, want empty", uiState.SelectedTicket)
	}
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
	sendKey("j")                       // select a ticket
	app.selected = 0                   // reset to first ticket for clean test
	sendKey("a")                       // assign
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

	t.Logf("Session walkthrough complete: %d events recorded", len(app.uiProjection.events))
}
