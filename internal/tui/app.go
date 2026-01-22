package tui

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"time"

	"github.com/binaryphile/fluentfp/either"
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
type EngineMode struct {
	Engine   *engine.Engine
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

// App is the main bubbletea model
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
	comparisonResult *metrics.ComparisonResult
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
}

// tickMsg is sent on each simulation tick (legacy engine mode)
type tickMsg time.Time

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
	sim := model.NewSimulation(model.PolicyDORAStrict, seed)
	sim.ID = simID

	tracker := metrics.NewTracker()

	var store events.Store
	var eng *engine.Engine

	if reg != nil {
		// Use shared registry - simulation accessible by both TUI and API
		store = reg.Store()
		eng = reg.RegisterSimulation(sim, tracker)
	} else {
		// Standalone mode - own event store
		store = events.NewMemoryStore()
		eng = engine.NewEngineWithStore(sim.Seed, store)
		eng.EmitCreated(sim.ID, sim.CurrentTick, events.SimConfig{
			TeamSize:     len(sim.Developers),
			SprintLength: sim.SprintLength,
			Seed:         sim.Seed,
		})
	}

	// Add default team via engine (emits DeveloperAdded events)
	eng.AddDeveloper("dev-1", "Alice", 1.0)
	eng.AddDeveloper("dev-2", "Bob", 0.8)
	eng.AddDeveloper("dev-3", "Carol", 1.2)

	// Generate initial backlog via engine (emits TicketCreated events)
	gen := engine.Scenarios["healthy"]
	rng := rand.New(rand.NewSource(seed))
	tickets := gen.Generate(rng, 12)
	for _, t := range tickets {
		eng.AddTicket(t)
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
		mode:         either.Left[EngineMode, ClientMode](engineMode),
		currentView:  ViewPlanning,
		paused:       true,
		speed:        1,
		modelEvents:  make([]model.Event, 0),
		tickInterval: 500 * time.Millisecond,
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

	return &App{
		mode:         either.Right[EngineMode](clientMode),
		state:        initialState,
		currentView:  ViewPlanning,
		paused:       true,
		speed:        1,
		modelEvents:  make([]model.Event, 0),
		tickInterval: 500 * time.Millisecond,
	}
}

// Init implements tea.Model
func (a *App) Init() tea.Cmd {
	// Start listening for events from the store subscription
	return a.listenForEvents()
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
		} else {
			a.state = msg.state
			switch msg.operation {
			case "sprint":
				// Switch to execution view and unpause
				a.currentView = ViewExecution
				a.paused = false
			case "tick":
				// Check if sprint ended
				if !a.state.SprintActive {
					a.paused = true
					a.statusMessage = "Sprint complete - press 's' for next sprint"
					a.statusExpiry = time.Now().Add(5 * time.Second)
				}
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
		eng.Tracker = eng.Tracker.Updated(&sim)
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

	case tickMsg:
		// Automatic tick timer - only applicable in engine mode
		// Client mode uses HTTP calls instead
		eng, ok := a.mode.GetLeft()
		if !ok {
			return a, nil
		}
		if !a.paused && a.currentView == ViewExecution {
			tickEvents := eng.Engine.Tick()
			a.modelEvents = append(a.modelEvents, tickEvents...)

			// Get current state from projection
			sim := eng.Engine.Sim()
			eng.Tracker = eng.Tracker.Updated(&sim)
			a.mode = either.Left[EngineMode, ClientMode](eng)

			// Check if sprint ended (SprintEnded event already cleared it in projection)
			if _, sprintActive := sim.CurrentSprintOption.Get(); !sprintActive {
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
		a.currentView = (a.currentView + 1) % 4
		return a, nil

	case " ":
		// Check mode for spacebar behavior
		if _, isClient := a.mode.Get(); isClient {
			// Client mode: trigger HTTP tick (blocked if in-flight)
			if a.inFlight || a.paused || !a.state.SprintActive {
				return a, nil // Blocked
			}
			a.inFlight = true
			return a, a.doHTTPTick()
		}
		// Engine mode: toggle pause
		a.paused = !a.paused
		if !a.paused {
			return a, a.tickCmd()
		}
		return a, nil

	case "up", "k":
		if a.selected > 0 {
			a.selected--
		}
		return a, nil

	case "down", "j":
		a.selected++
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
		// Engine mode
		eng, _ := a.mode.GetLeft()
		sim := eng.Engine.Sim()
		newPolicy := (sim.SizingPolicy + 1) % 4
		eng.Engine.SetPolicy(newPolicy)
		return a, nil

	case "s":
		// Start sprint (from planning view)
		if _, isClient := a.mode.Get(); isClient {
			// Client mode: HTTP call
			if a.currentView == ViewPlanning && !a.state.SprintActive {
				return a, a.doHTTPStartSprint()
			}
			return a, nil
		}
		// Engine mode
		eng, _ := a.mode.GetLeft()
		sim := eng.Engine.Sim()
		if _, ok := sim.CurrentSprintOption.Get(); a.currentView == ViewPlanning && !ok {
			eng.Engine.StartSprint()
			a.currentView = ViewExecution
			a.paused = false
			return a, a.tickCmd()
		}
		return a, nil

	case "a":
		// Assign selected ticket to first idle developer
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
					return a, a.doHTTPAssign(ticket.ID, devID)
				}
			}
			return a, nil
		}
		// Engine mode
		eng, _ := a.mode.GetLeft()
		sim := eng.Engine.Sim()
		if a.currentView == ViewPlanning && a.selected < len(sim.Backlog) {
			ticket := sim.Backlog[a.selected]
			for _, dev := range sim.Developers {
				if dev.IsIdle() {
					eng.Engine.AssignTicket(ticket.ID, dev.ID)
					break
				}
			}
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
		// Engine mode
		eng, _ := a.mode.GetLeft()
		sim := eng.Engine.Sim()
		if a.currentView == ViewPlanning && a.selected < len(sim.Backlog) {
			ticket := sim.Backlog[a.selected]
			eng.Engine.TryDecompose(ticket.ID)
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
		// Export simulation data (requires engine mode for local data)
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
		exporter := export.New(&sim, eng.Tracker, a.comparisonResult)
		result, err := exporter.Export()
		if err != nil {
			a.statusMessage = fmt.Sprintf("Export failed: %v", err)
			a.statusExpiry = time.Now().Add(5 * time.Second)
			return a, nil
		}
		a.statusMessage = result.Summary()
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
		err := persistence.Save(savePath, saveName, &sim, eng.Tracker)
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
		var newEngine *engine.Engine
		reg := currentEng.Registry

		if !reg.IsZero() {
			newStore = reg.Store()
			newEngine = reg.RegisterSimulation(sim, tracker)
			// RegisterSimulation already calls EmitLoadedState
		} else {
			newStore = events.NewMemoryStore()
			newEngine = engine.NewEngineWithStore(sim.Seed, newStore)
			// Emit events to populate projection from loaded state
			newEngine.EmitLoadedState(*sim)
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
		trackerA = a.runSprintWithAutoAssign(engA, trackerA)
		trackerB = a.runSprintWithAutoAssign(engB, trackerB)
	}

	// Get results and compare
	stateA := engA.Sim()
	stateB := engB.Sim()
	resultA := trackerA.GetResult(model.PolicyDORAStrict, &stateA)
	resultB := trackerB.GetResult(model.PolicyTameFlowCognitive, &stateB)

	comparison := metrics.Compare(resultA, resultB, seed)
	a.comparisonResult = &comparison
}

// createSimulationEngine creates a fresh engine with identical setup
func (a *App) createSimulationEngine(policy model.SizingPolicy, seed int64) *engine.Engine {
	sim := model.NewSimulation(policy, seed)
	sim.ID = fmt.Sprintf("cmp-%d-%s", seed, policy)

	eng := engine.NewEngine(sim.Seed)
	eng.EmitCreated(sim.ID, sim.CurrentTick, events.SimConfig{
		TeamSize:     3,
		SprintLength: sim.SprintLength,
		Seed:         sim.Seed,
		Policy:       policy,
	})

	// Same team
	eng.AddDeveloper("dev-1", "Alice", 1.0)
	eng.AddDeveloper("dev-2", "Bob", 0.8)
	eng.AddDeveloper("dev-3", "Carol", 1.2)

	// Same backlog (using same seed)
	gen := engine.Scenarios["mixed"] // Use mixed for more interesting comparison
	rng := rand.New(rand.NewSource(seed))
	tickets := gen.Generate(rng, 15)
	for _, t := range tickets {
		eng.AddTicket(t)
	}

	return eng
}

// runSprintWithAutoAssign runs a sprint with automatic ticket assignment
func (a *App) runSprintWithAutoAssign(eng *engine.Engine, tracker metrics.Tracker) metrics.Tracker {
	eng.StartSprint()

	// Auto-assign tickets to idle developers at start
	// Use index-based iteration so re-reads affect subsequent checks
	state := eng.Sim()
	for i := 0; i < len(state.Developers); i++ {
		dev := state.Developers[i]
		if dev.IsIdle() && len(state.Backlog) > 0 {
			ticket := state.Backlog[0]
			// Try decomposition first based on policy
			if children, decomposed := eng.TryDecompose(ticket.ID).Get(); decomposed {
				// Assign first child
				if len(children) > 0 {
					eng.AssignTicket(children[0].ID, dev.ID)
				}
			} else {
				eng.AssignTicket(ticket.ID, dev.ID)
			}
			// Re-read state after assignment - affects subsequent iterations
			state = eng.Sim()
		}
	}

	// Run the sprint
	sprint, _ := eng.Sim().CurrentSprintOption.Get()
	for eng.Sim().CurrentTick < sprint.EndDay {
		eng.Tick()
		state = eng.Sim()
		tracker = tracker.Updated(&state)

		// Re-assign idle developers mid-sprint
		for i := 0; i < len(state.Developers); i++ {
			dev := state.Developers[i]
			if dev.IsIdle() && len(state.Backlog) > 0 {
				ticket := state.Backlog[0]
				if children, decomposed := eng.TryDecompose(ticket.ID).Get(); decomposed {
					if len(children) > 0 {
						eng.AssignTicket(children[0].ID, dev.ID)
					}
				} else {
					eng.AssignTicket(ticket.ID, dev.ID)
				}
				// Re-read state after assignment - affects subsequent iterations
				state = eng.Sim()
			}
		}
	}
	return tracker
}

func (a *App) tickCmd() tea.Cmd {
	return tea.Tick(a.tickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
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
		if _, isClient := a.mode.Get(); isClient {
			hasActiveSprint = a.state.SprintActive
		} else {
			eng, _ := a.mode.GetLeft()
			sim := eng.Engine.Sim()
			_, hasActiveSprint = sim.CurrentSprintOption.Get()
		}
		lesson := SelectLesson(a.currentView, a.lessonState, hasActiveSprint, a.comparisonResult != nil)
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
