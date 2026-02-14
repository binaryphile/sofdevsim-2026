package tui

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"time"

	"github.com/binaryphile/fluentfp/either"
	"github.com/binaryphile/fluentfp/must"
	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/export"
	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
	"github.com/binaryphile/sofdevsim-2026/internal/persistence"
	"github.com/binaryphile/sofdevsim-2026/internal/registry"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ClientMode holds all HTTP client state.
// Value type: stored by value in Either. Client itself is a value type
// containing a shared *http.Client reference.
type ClientMode struct {
	Client Client
	SimID  string
}

// EngineMode holds all local engine state.
// Value type: stored by value in Either.
//
// Error handling: Engine methods return errors for event store conflicts,
// which only occur in concurrent multi-user scenarios. TUI runs in single-user
// mode with no concurrent access, so errors represent invariant violations.
// We use must.Get/must.Get2 to panic on errors - if they occur, it's a bug.
type EngineMode struct {
	Engine   engine.Engine // Value type - immutable pattern
	Tracker  metrics.Tracker
	Store    events.Store
	EventSub <-chan events.Event
	Registry *registry.SimRegistry
}

// View represents the current screen
type View int

const (
	ViewPlanning View = iota
	ViewExecution
	ViewMetrics
	ViewComparison
)

// App is the main bubbletea model.
//
// Pointer receiver: Bubbletea's tea.Model interface returns Model from Update(),
// but the framework expects mutations to persist across the event loop.
type App struct {
	// Mode: Either EngineMode (Left) or ClientMode (Right)
	// Replaces scattered nil checks with explicit sum type
	mode either.Either[EngineMode, ClientMode]

	// State (shared, updated from either mode)
	state    SimulationState // current state from HTTP or engine
	inFlight bool            // true while HTTP request in progress (Phase 8D)

	// UI state (mode-independent)
	currentView View
	paused      bool
	speed       int // ticks per update
	selected    int // selected item in lists

	// Events log (model events for display)
	modelEvents []model.Event

	// Comparison mode
	comparisonResult option.Basic[metrics.ComparisonResult]
	comparisonSeed   int64

	// Lessons panel
	lessonState LessonState

	// Dimensions
	width, height int

	// Tick timer
	tickInterval time.Duration

	// Status message (for export feedback)
	statusMessage string
	statusExpiry  time.Time

	// Input event projection (ES read model for UI state)
	uiProjection UIProjection

	// Office visualization state (event-sourced projection)
	officeProjection OfficeProjection

	// Staggered animation state (TUI-local, not projection state)
	staggeredAnimator StaggeredAnimator

	// Clock for time injection (defaults to time.Now, injectable for tests)
	clock func() time.Time

	// Randomness injection (defaults to rand.Float64, injectable for tests)
	randFloat func() float64
}

// tickMsg is sent on each simulation tick (legacy engine mode)
type tickMsg time.Time

// animationTickMsg is sent for animation frame updates (office visualization)
type animationTickMsg struct{}

// eventMsg is sent when an event is received from the store subscription
type eventMsg events.Event

// httpResultMsg is returned by async HTTP operations (client mode)
type httpResultMsg struct {
	operation string          // "tick", "assign", "sprint", "policy", "decompose"
	state     SimulationState // populated on success
	err       error           // populated on failure
}

// NewAppWithSeed creates a new App with the specified random seed.
// If seed is 0, uses current time for randomness.
// Deprecated: Use NewAppWithRegistry for shared simulation access.
func NewAppWithSeed(seed int64) *App {
	return NewAppWithRegistry(seed, nil) // nil = standalone mode
}

// NewAppWithRegistry creates a new App that shares simulations via the registry.
// If registry is nil, creates a standalone app with its own event store.
// If seed is 0, uses current time for randomness.
func NewAppWithRegistry(seed int64, reg *registry.SimRegistry) *App {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	simID := fmt.Sprintf("sim-%d", seed)
	sim := model.NewSimulation(simID, model.PolicyDORAStrict, seed)

	tracker := metrics.NewTracker()

	var store events.Store
	var eng engine.Engine

	if reg != nil {
		// Use shared registry - simulation accessible by both TUI and API
		store = reg.Store()
	} else {
		// Standalone mode - own event store
		store = events.NewMemoryStore()
	}

	eng = engine.NewEngineWithStore(sim.Seed, store)
	eng = must.Get(eng.EmitCreated(sim.ID, sim.CurrentTick, events.SimConfig{
		TeamSize:     6, // Will add 6 developers below
		SprintLength: sim.SprintLength,
		Seed:         sim.Seed,
	}))

	// Add default team via engine (emits DeveloperAdded events)
	// Names from DefaultDeveloperNames: diverse, inclusive
	eng = must.Get(eng.AddDeveloper("dev-1", "Mei", 1.0))
	eng = must.Get(eng.AddDeveloper("dev-2", "Amir", 0.8))
	eng = must.Get(eng.AddDeveloper("dev-3", "Suki", 1.2))
	eng = must.Get(eng.AddDeveloper("dev-4", "Jay", 1.0))
	eng = must.Get(eng.AddDeveloper("dev-5", "Priya", 0.9))
	eng = must.Get(eng.AddDeveloper("dev-6", "Kofi", 1.1))

	// Generate initial backlog via engine (emits TicketCreated events)
	gen := engine.Scenarios["healthy"]
	rng := rand.New(rand.NewSource(seed))
	tickets := gen.Generate(rng, 12)
	for _, t := range tickets {
		eng = must.Get(eng.AddTicket(t))
	}

	// Initialize office projection with developer IDs (event-sourced)
	// Devs start at cubicles then move to conference for initial planning
	devIDs := []string{"dev-1", "dev-2", "dev-3", "dev-4", "dev-5", "dev-6"}
	officeProjection := NewOfficeProjection(devIDs)

	// Move all developers to conference for initial sprint planning
	now := time.Now()
	recordConferenceEntry := func(proj OfficeProjection, devID string) OfficeProjection {
		return proj.Record(DevEnteredConference{DevID: devID}, 0, now)
	}
	officeProjection = slice.Fold(devIDs, officeProjection, recordConferenceEntry)

	// Store fully-populated engine in registry (design invariant: never store empty)
	if reg != nil {
		reg.SetInstance(simID, registry.SimInstance{
			Sim:     sim,
			Engine:  eng,
			Tracker: tracker,
			Office:  officeProjection,
		})
	}

	// Subscribe to event store for live updates
	eventSub := store.Subscribe(simID)

	// Create EngineMode (Left variant)
	engineMode := EngineMode{
		Engine:   eng,
		Tracker:  tracker,
		Store:    store,
		EventSub: eventSub,
		Registry: reg,
	}

	return &App{
		mode:              either.Left[EngineMode, ClientMode](engineMode),
		currentView:       ViewPlanning,
		paused:            true,
		speed:             1,
		modelEvents:       make([]model.Event, 0),
		tickInterval:      500 * time.Millisecond,
		uiProjection:      NewUIProjection(),
		officeProjection:  officeProjection,
		staggeredAnimator: StaggeredAnimator{LastChangedIndex: -1},
		clock:             time.Now,
		randFloat:         rand.Float64,
	}
}

// NewAppWithClient creates a new App that uses HTTP client for all operations.
// This is the Phase 8C constructor - TUI as pure HTTP client.
// Client is passed by value - it contains a shared *http.Client reference.
func NewAppWithClient(client Client, initialState SimulationState) *App {
	// Create ClientMode (Right variant)
	clientMode := ClientMode{
		Client: client,
		SimID:  initialState.ID,
	}

	// Initialize office projection from client state (developers from HTTP response)
	// Devs start in cubicles (StateIdle), will move to conference when planning begins
	devIDs := slice.From(initialState.Developers).ToString(DeveloperState.GetID)
	officeProjection := NewOfficeProjection(devIDs)

	return &App{
		mode:              either.Right[EngineMode](clientMode),
		state:             initialState,
		currentView:       ViewPlanning,
		paused:            true,
		speed:             1,
		modelEvents:       make([]model.Event, 0),
		tickInterval:      500 * time.Millisecond,
		uiProjection:      NewUIProjection(),
		officeProjection:  officeProjection,
		staggeredAnimator: StaggeredAnimator{LastChangedIndex: -1},
		clock:             time.Now,
		randFloat:         rand.Float64,
	}
}

// Init implements tea.Model
func (a *App) Init() tea.Cmd {
	// Start listening for events and animation timer
	return tea.Batch(a.listenForEvents(), a.animationTickCmd())
}

// listenForEvents returns a Cmd that waits for the next event from the subscription.
// Only applicable in engine mode - client mode returns nil.
func (a *App) listenForEvents() tea.Cmd {
	eng, ok := a.mode.GetLeft()
	if !ok {
		return nil // Client mode has no event subscription
	}
	eventSub := eng.EventSub
	return func() tea.Msg {
		if eventSub == nil {
			return nil
		}
		evt, ok := <-eventSub
		if !ok {
			return nil // Channel closed
		}
		return eventMsg(evt)
	}
}

// Update implements tea.Model
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return a.handleKey(msg)

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case httpResultMsg:
		// Handle async HTTP operation result (client mode)
		a.inFlight = false // Clear in-flight flag
		if msg.err != nil {
			a.statusMessage = fmt.Sprintf("%s failed: %v", msg.operation, msg.err)
			a.statusExpiry = time.Now().Add(5 * time.Second)
			// Record failed input events
			switch msg.operation {
			case "sprint":
				a.recordInputEvent(SprintStartAttempted{Outcome: Failed{Category: BusinessRule, Reason: msg.err.Error()}})
			case "tick":
				a.recordInputEvent(TickAttempted{Outcome: Failed{Category: BusinessRule, Reason: msg.err.Error()}})
			case "assign":
				a.recordInputEvent(AssignmentAttempted{TicketID: a.selectedTicketID(), Outcome: Failed{Category: BusinessRule, Reason: msg.err.Error()}})
			}
		} else {
			a.state = msg.state
			// Record successful input events
			switch msg.operation {
			case "sprint":
				// Switch to execution view and unpause
				a.currentView = ViewExecution
				a.paused = false
				a.recordInputEvent(SprintStartAttempted{Outcome: Succeeded{}})
				a.recordInputEvent(ViewSwitched{To: ViewExecution})
			case "tick":
				a.recordInputEvent(TickAttempted{Outcome: Succeeded{}})
				// Check if sprint ended
				if !a.state.SprintActive {
					a.paused = true
					a.statusMessage = "Sprint complete - press 's' for next sprint"
					a.statusExpiry = time.Now().Add(5 * time.Second)
				}
			case "assign":
				a.recordInputEvent(AssignmentAttempted{TicketID: a.selectedTicketID(), Outcome: Succeeded{}})
			}
		}
		return a, nil

	case eventMsg:
		// Received event from subscription - update display
		// This enables live updates when API modifies the simulation
		// Only applicable in engine mode (client mode doesn't use eventSub)
		eng, ok := a.mode.GetLeft()
		if !ok {
			return a, nil // Client mode - ignore
		}
		sim := eng.Engine.Sim()
		eng.Tracker = eng.Tracker.Updated(sim)
		a.mode = either.Left[EngineMode, ClientMode](eng)
		// Show status for significant events
		switch events.Event(msg).EventType() {
		case "SprintStarted":
			a.statusMessage = "Sprint started (external)"
			a.statusExpiry = time.Now().Add(2 * time.Second)
			a.currentView = ViewExecution
			a.paused = false
		case "TicketAssigned":
			a.statusMessage = "Ticket assigned (external)"
			a.statusExpiry = time.Now().Add(2 * time.Second)
		case "Ticked":
			a.statusMessage = fmt.Sprintf("Tick %d (external)", sim.CurrentTick)
			a.statusExpiry = time.Now().Add(1 * time.Second)
		}
		// Continue listening for more events
		return a, a.listenForEvents()

	case animationTickMsg:
		// Animation frame update for office visualization
		// Use staggered animator to pick which dev's face advances
		devCount := len(a.officeProjection.State().Animations)
		shouldPause := a.randFloat() < 0.2
		var devIdx int
		a.staggeredAnimator, devIdx, _ = a.staggeredAnimator.NextToAnimate(devCount, shouldPause)
		a.officeProjection = a.officeProjection.Record(
			AnimationFrameAdvanced{DevIdxToAdvance: devIdx},
			a.state.CurrentTick, a.clock())
		return a, a.animationTickCmd()

	case tickMsg:
		// Automatic tick timer - only applicable in engine mode
		// Client mode uses HTTP calls instead
		eng, ok := a.mode.GetLeft()
		if !ok {
			return a, nil
		}
		if !a.paused && a.currentView == ViewExecution {
			var tickEvents []model.Event
			eng.Engine, tickEvents = must.Get2(eng.Engine.Tick())
			a.modelEvents = append(a.modelEvents, tickEvents...)

			// Get current state from projection
			sim := eng.Engine.Sim()
			eng.Tracker = eng.Tracker.Updated(sim)
			a.mode = either.Left[EngineMode, ClientMode](eng)

			// Update developer animation states based on ticket progress
			a.updateDeveloperAnimationStates(sim)

			// Check if sprint ended (SprintEnded event already cleared it in projection)
			if _, sprintActive := sim.CurrentSprintOption.Get(); !sprintActive {
				for _, dev := range sim.Developers {
					a.officeProjection = a.officeProjection.Record(DevEnteredConference{DevID: dev.ID}, sim.CurrentTick, a.clock())
				}
				a.syncOfficeToRegistry()
				a.paused = true
				a.statusMessage = "Sprint complete - press 's' for next sprint"
				a.statusExpiry = time.Now().Add(5 * time.Second)
			}
		}
		return a, a.tickCmd()
	}

	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return a, tea.Quit

	case "tab":
		nextView := (a.currentView + 1) % 4
		a.currentView = nextView
		a.recordInputEvent(ViewSwitched{To: nextView})
		return a, nil

	case " ":
		// Check mode for spacebar behavior
		if _, isClient := a.mode.Get(); isClient {
			// Client mode: trigger HTTP tick (blocked if in-flight)
			if a.inFlight || a.paused || !a.state.SprintActive {
				// Precondition failed
				a.recordInputEvent(TickAttempted{Outcome: Failed{Category: BusinessRule, Reason: "tick blocked (in-flight, paused, or no sprint)"}})
				return a, nil // Blocked
			}
			a.inFlight = true
			// HTTP result handler will record success/failure
			return a, a.doHTTPTick()
		}
		// Engine mode: toggle pause (not a TickAttempted - engine ticks automatically)
		a.paused = !a.paused
		if !a.paused {
			return a, a.tickCmd()
		}
		return a, nil

	case "up", "k":
		if a.selected > 0 {
			a.selected--
			a.recordInputEvent(TicketSelected{ID: a.selectedTicketID()})
		}
		return a, nil

	case "down", "j":
		a.selected++
		a.recordInputEvent(TicketSelected{ID: a.selectedTicketID()})
		return a, nil

	case "p":
		// Cycle policy
		if _, isClient := a.mode.Get(); isClient {
			// Client mode: HTTP call - cycle through policy names
			policies := []string{"none", "dora-strict", "tameflow-cognitive"}
			currentIdx := 0
			for i, p := range []string{"None", "DORA-Strict", "TameFlow-Cognitive"} {
				if a.state.SizingPolicy == p {
					currentIdx = i
					break
				}
			}
			nextIdx := (currentIdx + 1) % len(policies)
			return a, a.doHTTPSetPolicy(policies[nextIdx])
		}
		// Engine mode - SetPolicy returns new engine (value semantics)
		eng, _ := a.mode.GetLeft()
		sim := eng.Engine.Sim()
		newPolicy := (sim.SizingPolicy + 1) % 4
		eng.Engine = must.Get(eng.Engine.SetPolicy(newPolicy))
		a.mode = either.Left[EngineMode, ClientMode](eng)
		return a, nil

	case "s":
		// Start sprint (from planning view)
		if _, isClient := a.mode.Get(); isClient {
			// Client mode: HTTP call - outcome determined by HTTP response (recorded in handleHTTPResult)
			if a.currentView == ViewPlanning && !a.state.SprintActive {
				return a, a.doHTTPStartSprint()
			}
			// Precondition failed: not in planning or sprint already active
			a.recordInputEvent(SprintStartAttempted{Outcome: Failed{Category: BusinessRule, Reason: "sprint already active or wrong view"}})
			return a, nil
		}
		// Engine mode - StartSprint returns new engine (value semantics)
		eng, _ := a.mode.GetLeft()
		sim := eng.Engine.Sim()
		if _, ok := sim.CurrentSprintOption.Get(); a.currentView == ViewPlanning && !ok {
			eng.Engine = must.Get(eng.Engine.StartSprint())
			a.mode = either.Left[EngineMode, ClientMode](eng)
			a.currentView = ViewExecution
			a.paused = false
			a.recordInputEvent(SprintStartAttempted{Outcome: Succeeded{}})
			a.recordInputEvent(ViewSwitched{To: ViewExecution})
			return a, a.tickCmd()
		}
		// Precondition failed: sprint already active or wrong view
		a.recordInputEvent(SprintStartAttempted{Outcome: Failed{Category: BusinessRule, Reason: "sprint already active or wrong view"}})
		return a, nil

	case "a":
		// Assign selected ticket to first idle developer
		ticketID := a.selectedTicketID()
		if _, isClient := a.mode.Get(); isClient {
			// Client mode: HTTP call
			if a.currentView == ViewPlanning && a.selected < len(a.state.Backlog) {
				ticket := a.state.Backlog[a.selected]
				// Find first idle developer
				var devID string
				for _, dev := range a.state.Developers {
					if dev.IsIdle {
						devID = dev.ID
						break
					}
				}
				if devID != "" {
					// HTTP result handler will record success/failure
					return a, a.doHTTPAssign(ticket.ID, devID)
				}
				// No idle developer
				a.recordInputEvent(AssignmentAttempted{TicketID: ticketID, Outcome: Failed{Category: Conflict, Reason: "no idle developer"}})
			} else {
				// Wrong view or no ticket selected
				a.recordInputEvent(AssignmentAttempted{TicketID: ticketID, Outcome: Failed{Category: BusinessRule, Reason: "wrong view or no ticket selected"}})
			}
			return a, nil
		}
		// Engine mode - AssignTicket returns new engine (value semantics)
		eng, _ := a.mode.GetLeft()
		sim := eng.Engine.Sim()
		if a.currentView == ViewPlanning && a.selected < len(sim.Backlog) {
			ticket := sim.Backlog[a.selected]
			assigned := false
			cubiclePositions := CubicleLayout(len(sim.Developers))
			for devIdx, dev := range sim.Developers {
				if dev.IsIdle() {
					eng.Engine = must.Get(eng.Engine.AssignTicket(ticket.ID, dev.ID))
					a.mode = either.Left[EngineMode, ClientMode](eng)
					// Record office animation event: developer moves to cubicle
					target := cubiclePositions[devIdx]
					a.officeProjection = a.officeProjection.Record(DevAssignedToTicket{
						DevID:    dev.ID,
						TicketID: ticket.ID,
						Target:   target,
					}, a.state.CurrentTick, a.clock())
					a.syncOfficeToRegistry()
					a.recordInputEvent(AssignmentAttempted{TicketID: ticket.ID, Outcome: Succeeded{}})
					assigned = true
					break
				}
			}
			if !assigned {
				a.recordInputEvent(AssignmentAttempted{TicketID: ticket.ID, Outcome: Failed{Category: Conflict, Reason: "no idle developer"}})
			}
		} else {
			// Wrong view or no ticket selected
			a.recordInputEvent(AssignmentAttempted{TicketID: ticketID, Outcome: Failed{Category: BusinessRule, Reason: "wrong view or no ticket selected"}})
		}
		return a, nil

	case "d":
		// Decompose selected ticket
		if _, isClient := a.mode.Get(); isClient {
			// Client mode: HTTP call
			if a.currentView == ViewPlanning && a.selected < len(a.state.Backlog) {
				ticket := a.state.Backlog[a.selected]
				return a, a.doHTTPDecompose(ticket.ID)
			}
			return a, nil
		}
		// Engine mode - TryDecompose returns new engine (value semantics)
		eng, _ := a.mode.GetLeft()
		sim := eng.Engine.Sim()
		if a.currentView == ViewPlanning && a.selected < len(sim.Backlog) {
			ticket := sim.Backlog[a.selected]
			eng.Engine, _ = must.Get2(eng.Engine.TryDecompose(ticket.ID))
			a.mode = either.Left[EngineMode, ClientMode](eng)
		}
		return a, nil

	case "+", "=":
		if a.tickInterval > 100*time.Millisecond {
			a.tickInterval -= 100 * time.Millisecond
		}
		return a, nil

	case "-":
		if a.tickInterval < 2*time.Second {
			a.tickInterval += 100 * time.Millisecond
		}
		return a, nil

	case "c":
		// Run comparison mode (requires engine mode for local simulation)
		if _, isClient := a.mode.Get(); isClient {
			a.statusMessage = "Comparison requires local mode (run without --client)"
			a.statusExpiry = time.Now().Add(3 * time.Second)
			return a, nil
		}
		a.runComparison()
		a.currentView = ViewComparison
		return a, nil

	case "e":
		// Export HTML report (requires engine mode for local data)
		eng, isEngine := a.mode.GetLeft()
		if !isEngine {
			a.statusMessage = "Export requires local mode (run without --client)"
			a.statusExpiry = time.Now().Add(3 * time.Second)
			return a, nil
		}
		sim := eng.Engine.Sim()
		if len(sim.CompletedTickets) == 0 {
			a.statusMessage = "Nothing to export - no completed tickets"
			a.statusExpiry = time.Now().Add(3 * time.Second)
			return a, nil
		}

		// Build export data types from TUI state
		simParams := export.SimulationParams{
			Seed:           sim.Seed,
			Policy:         sim.SizingPolicy.String(),
			DeveloperCount: len(sim.Developers),
		}

		trackerData := export.TrackerData{
			LeadTime:        eng.Tracker.DORA.LeadTimeAvg.Hours() / 24,
			DeployFrequency: eng.Tracker.DORA.DeployFrequency,
			ChangeFailRate:  eng.Tracker.DORA.ChangeFailRate,
			MTTR:            eng.Tracker.DORA.MTTRAvg.Hours() / 24,
			LeadTimeHistory: extractLeadTimeHistory(eng.Tracker.DORA.History),
			FeverHistory:    extractFeverHistory(eng.Tracker.Fever.History),
		}

		var comparison *export.ComparisonSummary
		if result, ok := a.comparisonResult.Get(); ok {
			comparison = &export.ComparisonSummary{
				PolicyA:     result.PolicyA.String(),
				PolicyB:     result.PolicyB.String(),
				Winner:      result.OverallWinner.String(),
				LeadTimeA:   result.ResultsA.FinalMetrics.LeadTimeAvg.Hours() / 24,
				LeadTimeB:   result.ResultsB.FinalMetrics.LeadTimeAvg.Hours() / 24,
				DeployFreqA: result.ResultsA.FinalMetrics.DeployFrequency,
				DeployFreqB: result.ResultsB.FinalMetrics.DeployFrequency,
			}
		}

		exporter := export.NewHTMLExporter(simParams, trackerData, a.lessonState.SeenMap, comparison)
		path := fmt.Sprintf("exports/simulation-report-%s.html", time.Now().Format("20060102-150405"))
		if err := exporter.ExportToFile(path); err != nil {
			a.statusMessage = fmt.Sprintf("Export failed: %v", err)
			a.statusExpiry = time.Now().Add(5 * time.Second)
			return a, nil
		}
		a.statusMessage = fmt.Sprintf("Exported to %s", path)
		a.statusExpiry = time.Now().Add(5 * time.Second)
		return a, nil

	case "ctrl+s":
		// Save simulation state (requires engine mode for local state)
		eng, isEngine := a.mode.GetLeft()
		if !isEngine {
			a.statusMessage = "Save requires local mode (run without --client)"
			a.statusExpiry = time.Now().Add(3 * time.Second)
			return a, nil
		}
		sim := eng.Engine.Sim()
		saveName := fmt.Sprintf("sim-%d-%s", sim.Seed, time.Now().Format("150405"))
		savePath := persistence.GenerateSavePath(persistence.DefaultSavesDir(), saveName)
		err := persistence.Save(savePath, saveName, sim, eng.Tracker)
		if err != nil {
			a.statusMessage = fmt.Sprintf("Save failed: %v", err)
			a.statusExpiry = time.Now().Add(5 * time.Second)
			return a, nil
		}
		a.statusMessage = fmt.Sprintf("Saved to %s", savePath)
		a.statusExpiry = time.Now().Add(3 * time.Second)
		return a, nil

	case "ctrl+o":
		// Load simulation state (requires engine mode for local state)
		currentEng, isEngine := a.mode.GetLeft()
		if !isEngine {
			a.statusMessage = "Load requires local mode (run without --client)"
			a.statusExpiry = time.Now().Add(3 * time.Second)
			return a, nil
		}
		// Load most recent save
		saves, err := persistence.ListSaves(persistence.DefaultSavesDir())
		if err != nil || len(saves) == 0 {
			a.statusMessage = "No saves found in saves/ directory"
			a.statusExpiry = time.Now().Add(3 * time.Second)
			return a, nil
		}
		// Find most recent save
		latest := saves[0]
		for _, s := range saves[1:] {
			if s.Timestamp.After(latest.Timestamp) {
				latest = s
			}
		}
		sim, tracker, err := persistence.Load(latest.Path)
		if err != nil {
			a.statusMessage = fmt.Sprintf("Load failed: %v", err)
			a.statusExpiry = time.Now().Add(5 * time.Second)
			return a, nil
		}
		// Ensure simulation has ID for event sourcing
		if sim.ID == "" {
			sim.ID = fmt.Sprintf("sim-%d", sim.Seed)
		}
		// Rebuild EngineMode with loaded state
		var newStore events.Store
		var newEngine engine.Engine
		reg := currentEng.Registry

		if !reg.IsZero() {
			newStore = reg.Store()
		} else {
			newStore = events.NewMemoryStore()
		}

		newEngine = engine.NewEngineWithStore(sim.Seed, newStore)
		newEngine = must.Get(newEngine.EmitLoadedState(sim))

		// Store fully-populated engine in registry
		if !reg.IsZero() {
			reg.SetInstance(sim.ID, registry.SimInstance{
				Sim:     sim,
				Engine:  newEngine,
				Tracker: tracker,
			})
		}

		// Re-subscribe to new simulation's events
		eventSub := newStore.Subscribe(sim.ID)

		// Update mode with new EngineMode
		a.mode = either.Left[EngineMode, ClientMode](EngineMode{
			Engine:   newEngine,
			Tracker:  tracker,
			Store:    newStore,
			EventSub: eventSub,
			Registry: reg,
		})
		a.paused = true
		a.statusMessage = fmt.Sprintf("Loaded %s (Day %d)", filepath.Base(latest.Path), sim.CurrentTick)
		a.statusExpiry = time.Now().Add(3 * time.Second)
		// Start listening for events from new subscription
		return a, a.listenForEvents()

	case "h":
		// Toggle lessons panel
		a.lessonState = a.lessonState.WithVisible(!a.lessonState.Visible)
		a.recordInputEvent(LessonPanelToggled{})
		if a.lessonState.Visible {
			a.statusMessage = "Lessons enabled"
		} else {
			a.statusMessage = "Lessons hidden"
		}
		a.statusExpiry = time.Now().Add(2 * time.Second)
		return a, nil
	}

	return a, nil
}

// runComparison runs simulations with DORA vs TameFlow policies
func (a *App) runComparison() {
	seed := time.Now().UnixNano()
	a.comparisonSeed = seed

	// Run simulation with DORA-Strict policy
	engA := a.createSimulationEngine(model.PolicyDORAStrict, seed)
	trackerA := metrics.NewTracker()

	// Run simulation with TameFlow-Cognitive policy
	engB := a.createSimulationEngine(model.PolicyTameFlowCognitive, seed)
	trackerB := metrics.NewTracker()

	// Run 3 sprints each
	for i := 0; i < 3; i++ {
		engA, trackerA = a.runSprintWithAutoAssign(engA, trackerA)
		engB, trackerB = a.runSprintWithAutoAssign(engB, trackerB)
	}

	// Get results and compare
	stateA := engA.Sim()
	stateB := engB.Sim()
	resultA := trackerA.GetResult(model.PolicyDORAStrict, stateA)
	resultB := trackerB.GetResult(model.PolicyTameFlowCognitive, stateB)

	comparison := metrics.Compare(resultA, resultB, seed)
	a.comparisonResult = option.Of(comparison)
}

// createSimulationEngine creates a fresh engine with identical setup
func (a *App) createSimulationEngine(policy model.SizingPolicy, seed int64) engine.Engine {
	simID := fmt.Sprintf("cmp-%d-%s", seed, policy)
	sim := model.NewSimulation(simID, policy, seed)

	eng := engine.NewEngine(sim.Seed)
	eng = must.Get(eng.EmitCreated(sim.ID, sim.CurrentTick, events.SimConfig{
		TeamSize:     6,
		SprintLength: sim.SprintLength,
		Seed:         sim.Seed,
		Policy:       policy,
	}))

	// Same team (6 developers)
	eng = must.Get(eng.AddDeveloper("dev-1", "Mei", 1.0))
	eng = must.Get(eng.AddDeveloper("dev-2", "Amir", 0.8))
	eng = must.Get(eng.AddDeveloper("dev-3", "Suki", 1.2))
	eng = must.Get(eng.AddDeveloper("dev-4", "Jay", 1.0))
	eng = must.Get(eng.AddDeveloper("dev-5", "Priya", 0.9))
	eng = must.Get(eng.AddDeveloper("dev-6", "Kofi", 1.1))

	// Same backlog (using same seed)
	gen := engine.Scenarios["mixed"] // Use mixed for more interesting comparison
	rng := rand.New(rand.NewSource(seed))
	tickets := gen.Generate(rng, 15)
	for _, t := range tickets {
		eng = must.Get(eng.AddTicket(t))
	}

	return eng
}

// runSprintWithAutoAssign runs a sprint with automatic ticket assignment
func (a *App) runSprintWithAutoAssign(eng engine.Engine, tracker metrics.Tracker) (engine.Engine, metrics.Tracker) {
	eng = must.Get(eng.StartSprint())

	// Auto-assign tickets to idle developers at start
	// Use index-based iteration so re-reads affect subsequent checks
	state := eng.Sim()
	for i := 0; i < len(state.Developers); i++ {
		dev := state.Developers[i]
		if dev.IsIdle() && len(state.Backlog) > 0 {
			ticket := state.Backlog[0]
			// Try decomposition first based on policy
			var result either.Either[engine.NotDecomposable, []model.Ticket]
			eng, result = must.Get2(eng.TryDecompose(ticket.ID))
			if children, decomposed := result.Get(); decomposed {
				// Assign first child
				if len(children) > 0 {
					eng = must.Get(eng.AssignTicket(children[0].ID, dev.ID))
				}
			} else {
				eng = must.Get(eng.AssignTicket(ticket.ID, dev.ID))
			}
			// Re-read state after assignment - affects subsequent iterations
			state = eng.Sim()
		}
	}

	// Invariant: StartSprint() guarantees CurrentSprintOption.IsOk() - safe to MustGet
	sprint := eng.Sim().CurrentSprintOption.MustGet()
	for eng.Sim().CurrentTick < sprint.EndDay {
		eng, _ = must.Get2(eng.Tick())
		state = eng.Sim()
		tracker = tracker.Updated(state)

		// Re-assign idle developers mid-sprint
		for i := 0; i < len(state.Developers); i++ {
			dev := state.Developers[i]
			if dev.IsIdle() && len(state.Backlog) > 0 {
				ticket := state.Backlog[0]
				var result either.Either[engine.NotDecomposable, []model.Ticket]
				eng, result = must.Get2(eng.TryDecompose(ticket.ID))
				if children, decomposed := result.Get(); decomposed {
					if len(children) > 0 {
						eng = must.Get(eng.AssignTicket(children[0].ID, dev.ID))
					}
				} else {
					eng = must.Get(eng.AssignTicket(ticket.ID, dev.ID))
				}
				// Re-read state after assignment - affects subsequent iterations
				state = eng.Sim()
			}
		}
	}
	return eng, tracker
}

func (a *App) tickCmd() tea.Cmd {
	return tea.Tick(a.tickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// animationTickCmd returns a Cmd that triggers animation frame updates.
// Runs at 100ms intervals for smooth animation.
func (a *App) animationTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return animationTickMsg{}
	})
}

// doHTTPTick returns a Cmd that makes an HTTP tick request.
// Only valid in client mode - caller must verify.
func (a *App) doHTTPTick() tea.Cmd {
	cli, _ := a.mode.Get() // Caller verified client mode
	return func() tea.Msg {
		resp, err := cli.Client.Tick(cli.SimID)
		if err != nil {
			return httpResultMsg{operation: "tick", err: err}
		}
		return httpResultMsg{operation: "tick", state: resp.Simulation}
	}
}

// doHTTPStartSprint returns a Cmd that makes an HTTP start sprint request.
// Only valid in client mode - caller must verify.
func (a *App) doHTTPStartSprint() tea.Cmd {
	cli, _ := a.mode.Get() // Caller verified client mode
	return func() tea.Msg {
		resp, err := cli.Client.StartSprint(cli.SimID)
		if err != nil {
			return httpResultMsg{operation: "sprint", err: err}
		}
		return httpResultMsg{operation: "sprint", state: resp.Simulation}
	}
}

// doHTTPAssign returns a Cmd that makes an HTTP assign request.
// Only valid in client mode - caller must verify.
func (a *App) doHTTPAssign(ticketID, devID string) tea.Cmd {
	cli, _ := a.mode.Get() // Caller verified client mode
	return func() tea.Msg {
		err := cli.Client.Assign(cli.SimID, ticketID, devID)
		if err != nil {
			return httpResultMsg{operation: "assign", err: err}
		}
		// Need to fetch updated state
		resp, err := cli.Client.GetSimulation(cli.SimID)
		if err != nil {
			return httpResultMsg{operation: "assign", err: err}
		}
		return httpResultMsg{operation: "assign", state: resp.Simulation}
	}
}

// doHTTPSetPolicy returns a Cmd that makes an HTTP set policy request.
// Only valid in client mode - caller must verify.
func (a *App) doHTTPSetPolicy(policy string) tea.Cmd {
	cli, _ := a.mode.Get() // Caller verified client mode
	return func() tea.Msg {
		resp, err := cli.Client.SetPolicy(cli.SimID, policy)
		if err != nil {
			return httpResultMsg{operation: "policy", err: err}
		}
		return httpResultMsg{operation: "policy", state: resp.Simulation}
	}
}

// doHTTPDecompose returns a Cmd that makes an HTTP decompose request.
// Only valid in client mode - caller must verify.
func (a *App) doHTTPDecompose(ticketID string) tea.Cmd {
	cli, _ := a.mode.Get() // Caller verified client mode
	return func() tea.Msg {
		resp, err := cli.Client.Decompose(cli.SimID, ticketID)
		if err != nil {
			return httpResultMsg{operation: "decompose", err: err}
		}
		return httpResultMsg{operation: "decompose", state: resp.Simulation}
	}
}

// View implements tea.Model
func (a *App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	var content string
	switch a.currentView {
	case ViewPlanning:
		content = a.planningView()
	case ViewExecution:
		content = a.executionView()
	case ViewMetrics:
		content = a.metricsView()
	case ViewComparison:
		content = a.comparisonView()
	}

	// Compose with lessons panel when visible
	if a.lessonState.Visible {
		var hasActiveSprint bool
		var triggers TriggerState
		// Trigger detection differs between client and engine modes:
		// - Client mode uses primitives (strings) to avoid import cycles (lessons can't import tui)
		// - Engine mode uses CQRS TriggerProjection for state-based triggers + event detection
		if _, isClient := a.mode.Get(); isClient {
			hasActiveSprint = a.state.SprintActive
			triggers = BuildTriggersFromClientState(a.state)
		} else {
			eng, _ := a.mode.GetLeft()
			sim := eng.Engine.Sim()
			_, hasActiveSprint = sim.CurrentSprintOption.Get()
			// Use CQRS projection for SprintCount + event detection for UC19/20/21
			triggers = BuildTriggerStateFromEngine(sim, eng.Tracker.Fever.Status, sim.ActiveTickets, sim.CompletedTickets)
		}
		// Build comparison summary for UC23 dynamic lesson content
		comparison := BuildComparisonSummary(a.comparisonResult)
		lesson := SelectLesson(a.currentView, a.lessonState, hasActiveSprint, a.comparisonResult.IsOk(), triggers, comparison)
		a.lessonState = a.lessonState.WithSeen(lesson.ID)
		lessonPanel := a.lessonsPanel(lesson)
		content = lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(a.width*2/3-2).Render(content),
			lessonPanel,
		)
	}

	// Add header and help
	header := a.headerView()
	help := a.helpView()

	// Add status message if present and not expired
	if a.statusMessage != "" && time.Now().Before(a.statusExpiry) {
		status := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(a.statusMessage)
		return lipgloss.JoinVertical(lipgloss.Left, header, content, status, help)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, content, help)
}

// headerState holds values extracted from either mode for header rendering.
type headerState struct {
	policy         string
	currentTick    int
	backlogCount   int
	completedCount int
	seed           int64
}

func (a *App) headerView() string {
	viewNames := []string{"Planning", "Execution", "Metrics", "Comparison"}
	tabs := ""
	for i, name := range viewNames {
		style := MutedStyle
		if View(i) == a.currentView {
			style = TitleStyle
		}
		tabs += style.Render(fmt.Sprintf(" %s ", name))
	}

	// Extract header state using Fold for exhaustive pattern matching
	h := either.Fold(a.mode,
		func(eng EngineMode) headerState {
			sim := eng.Engine.Sim()
			return headerState{
				policy:         sim.SizingPolicy.String(),
				currentTick:    sim.CurrentTick,
				backlogCount:   len(sim.Backlog),
				completedCount: len(sim.CompletedTickets),
				seed:           sim.Seed,
			}
		},
		func(_ ClientMode) headerState {
			return headerState{
				policy:         a.state.SizingPolicy,
				currentTick:    a.state.CurrentTick,
				backlogCount:   a.state.BacklogCount,
				completedCount: a.state.CompletedTicketCount,
				seed:           a.state.Seed,
			}
		},
	)

	policyStr := fmt.Sprintf("Policy: %s", h.policy)
	status := "PAUSED"
	if !a.paused {
		status = "RUNNING"
	}

	right := MutedStyle.Render(fmt.Sprintf("%s | %s | Day %d | Backlog: %d | Done: %d | Seed %d", policyStr, status, h.currentTick, h.backlogCount, h.completedCount, h.seed))

	return BoxStyle.Width(a.width - 2).Render(
		lipgloss.JoinHorizontal(lipgloss.Top, tabs, "  ", right),
	)
}

func (a *App) helpView() string {
	keys := []struct{ key, desc string }{
		{"Tab", "switch view"},
		{"Space", "pause/resume"},
		{"+/-", "speed"},
		{"c", "compare policies"},
		{"e", "export"},
		{"h", "lessons"},
		{"^s", "save"},
		{"^o", "load"},
		{"q", "quit"},
	}

	if a.currentView == ViewPlanning {
		keys = append([]struct{ key, desc string }{
			{"a", "assign"},
			{"d", "decompose"},
			{"p", "policy"},
			{"s", "start sprint"},
		}, keys...)
	}

	help := ""
	for _, k := range keys {
		help += HelpKeyStyle.Render(k.key) + HelpDescStyle.Render(" "+k.desc+"  ")
	}

	return MutedStyle.Render(help)
}

// Action: records input event, mutates App.uiProjection.
// Used by key handlers to track user interactions for UI state projection.
func (a *App) recordInputEvent(evt InputEvent) {
	a.uiProjection = a.uiProjection.Record(evt)
}

// Calculation: App → string (selected ticket ID, empty if none).
// Returns the ticket ID at the current selection index, or empty if out of bounds.
func (a *App) selectedTicketID() string {
	if _, isClient := a.mode.Get(); isClient {
		if a.selected < len(a.state.Backlog) {
			return a.state.Backlog[a.selected].ID
		}
		return ""
	}
	// Engine mode
	eng, _ := a.mode.GetLeft()
	sim := eng.Engine.Sim()
	if a.selected < len(sim.Backlog) {
		return sim.Backlog[a.selected].ID
	}
	return ""
}

// extractLeadTimeHistory extracts lead time values from DORA snapshots.
// Calculation: []DORASnapshot → []float64
func extractLeadTimeHistory(history []metrics.DORASnapshot) []float64 {
	return slice.From(history).ToFloat64(metrics.DORASnapshot.GetLeadTimeAvg)
}

// extractFeverHistory extracts percent used values from fever snapshots.
// Calculation: []FeverSnapshot → []float64
func extractFeverHistory(history []metrics.FeverSnapshot) []float64 {
	return slice.From(history).ToFloat64(metrics.FeverSnapshot.GetPercentUsed)
}

// updateDeveloperAnimationStates syncs office animation with simulation state.
// Records animation events based on developer/ticket status using predicates.
// Re-reads projection state after each event to avoid stale predicates.
func (a *App) updateDeveloperAnimationStates(sim model.Simulation) {
	for _, dev := range sim.Developers {
		// Re-read state each iteration to see effects of prior recordings
		state := a.officeProjection.State()

		if dev.IsIdle() {
			// recordCompletion records ticket completion event for the animation.
			recordCompletion := option.Lift(func(anim DeveloperAnimation) {
				a.officeProjection = a.officeProjection.Record(DevCompletedTicket{
					DevID:    dev.ID,
					TicketID: dev.CurrentTicket,
				}, sim.CurrentTick, a.clock())
			})
			recordCompletion(state.GetActiveAnimationOption(dev.ID))
			continue
		}

		// Find the active ticket for this developer
		ticketIdx := sim.FindActiveTicketIndex(dev.CurrentTicket)
		if ticketIdx == -1 {
			continue
		}
		ticket := sim.ActiveTickets[ticketIdx]

		// updateAnimationState transitions animation to frustrated or working state.
		updateAnimationState := option.Lift(func(anim DeveloperAnimation) {
			switch {
			case anim.ShouldBecomeFrustrated(ticket.ActualDays, ticket.EstimatedDays):
				a.officeProjection = a.officeProjection.Record(DevBecameFrustrated{
					DevID:    dev.ID,
					TicketID: ticket.ID,
				}, sim.CurrentTick, a.clock())
			case anim.ShouldStartWorking():
				a.officeProjection = a.officeProjection.Record(DevStartedWorking{DevID: dev.ID}, sim.CurrentTick, a.clock())
			}
		})
		updateAnimationState(state.GetAnimationOption(dev.ID))
	}
	a.syncOfficeToRegistry()
}

// syncOfficeToRegistry updates the registry's SimInstance.Office with current projection.
// Called after state-changing office events (not AnimationFrameAdvanced).
func (a *App) syncOfficeToRegistry() {
	eng, isEngine := a.mode.GetLeft()
	if !isEngine || eng.Registry == nil {
		return // Client mode or no registry
	}
	simID := fmt.Sprintf("sim-%d", eng.Engine.Sim().Seed)
	eng.Registry.UpdateOffice(simID, a.officeProjection)
}

// getDeveloperNames returns developer names for office visualization.
// Calculation: App → []string
func (a *App) getDeveloperNames() []string {
	// engineDevNames extracts developer names from engine mode simulation.
	engineDevNames := func(eng EngineMode) []string {
		return slice.From(eng.Engine.Sim().Developers).ToString(model.Developer.GetName)
	}
	// clientDevNames extracts developer names from client mode state.
	clientDevNames := func(_ ClientMode) []string {
		return slice.From(a.state.Developers).ToString(DeveloperState.GetName)
	}
	return either.Fold(a.mode, engineDevNames, clientDevNames)
}
