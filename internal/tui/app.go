package tui

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/registry"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/export"
	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
	"github.com/binaryphile/sofdevsim-2026/internal/persistence"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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
	// HTTP client mode (Phase 8C) - used when client != nil
	client   *Client          // HTTP client for API calls
	simID    string           // current simulation ID
	state    SimulationState  // current state from HTTP responses
	inFlight bool             // true while HTTP request in progress (Phase 8D)

	// Legacy engine mode - used when engine != nil (comparison mode, standalone)
	engine   *engine.Engine
	tracker  metrics.Tracker
	store    events.Store          // event store for event sourcing
	registry registry.SimRegistry // optional shared registry (zero value = no registry)
	eventSub <-chan events.Event // subscription channel for live updates

	// UI state
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
	return NewAppWithRegistry(seed, registry.SimRegistry{}) // zero value = standalone mode
}

// NewAppWithRegistry creates a new App that shares simulations via the registry.
// If registry is zero value, creates a standalone app with its own event store.
// If seed is 0, uses current time for randomness.
func NewAppWithRegistry(seed int64, reg registry.SimRegistry) *App {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	simID := fmt.Sprintf("sim-%d", seed)
	sim := model.NewSimulation(model.PolicyDORAStrict, seed)
	sim.ID = simID

	tracker := metrics.NewTracker()

	var store events.Store
	var eng *engine.Engine

	if !reg.IsZero() {
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

	return &App{
		engine:       eng,
		tracker:      tracker,
		store:        store,
		registry:     reg,
		eventSub:     eventSub,
		currentView:  ViewPlanning,
		paused:       true,
		speed:        1,
		modelEvents:  make([]model.Event, 0),
		tickInterval: 500 * time.Millisecond,
	}
}

// NewAppWithClient creates a new App that uses HTTP client for all operations.
// This is the Phase 8C constructor - TUI as pure HTTP client.
func NewAppWithClient(client *Client, initialState SimulationState) *App {
	return &App{
		client:       client,
		simID:        initialState.ID,
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

// listenForEvents returns a Cmd that waits for the next event from the subscription
func (a *App) listenForEvents() tea.Cmd {
	return func() tea.Msg {
		if a.eventSub == nil {
			return nil
		}
		evt, ok := <-a.eventSub
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
		if a.client != nil {
			return a, nil
		}
		sim := a.engine.Sim()
		a.tracker = a.tracker.Updated(&sim)
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
		if a.client != nil {
			return a, nil
		}
		if !a.paused && a.currentView == ViewExecution {
			tickEvents := a.engine.Tick()
			a.modelEvents = append(a.modelEvents, tickEvents...)

			// Get current state from projection
			sim := a.engine.Sim()
			a.tracker = a.tracker.Updated(&sim)

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
		// Client mode: trigger HTTP tick (blocked if in-flight)
		if a.client != nil {
			if a.inFlight || a.paused || !a.state.SprintActive {
				return a, nil // Blocked
			}
			a.inFlight = true
			return a, a.doHTTPTick()
		}
		// Legacy engine mode: toggle pause
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
		if a.client != nil {
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
		// Legacy engine mode
		sim := a.engine.Sim()
		newPolicy := (sim.SizingPolicy + 1) % 4
		a.engine.SetPolicy(newPolicy)
		return a, nil

	case "s":
		// Start sprint (from planning view)
		if a.client != nil {
			// Client mode: HTTP call
			if a.currentView == ViewPlanning && !a.state.SprintActive {
				return a, a.doHTTPStartSprint()
			}
			return a, nil
		}
		// Legacy engine mode
		sim := a.engine.Sim()
		if _, ok := sim.CurrentSprintOption.Get(); a.currentView == ViewPlanning && !ok {
			a.engine.StartSprint()
			a.currentView = ViewExecution
			a.paused = false
			return a, a.tickCmd()
		}
		return a, nil

	case "a":
		// Assign selected ticket to first idle developer
		if a.client != nil {
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
		// Legacy engine mode
		sim := a.engine.Sim()
		if a.currentView == ViewPlanning && a.selected < len(sim.Backlog) {
			ticket := sim.Backlog[a.selected]
			for _, dev := range sim.Developers {
				if dev.IsIdle() {
					a.engine.AssignTicket(ticket.ID, dev.ID)
					break
				}
			}
		}
		return a, nil

	case "d":
		// Decompose selected ticket
		if a.client != nil {
			// Client mode: HTTP call
			if a.currentView == ViewPlanning && a.selected < len(a.state.Backlog) {
				ticket := a.state.Backlog[a.selected]
				return a, a.doHTTPDecompose(ticket.ID)
			}
			return a, nil
		}
		// Legacy engine mode
		sim := a.engine.Sim()
		if a.currentView == ViewPlanning && a.selected < len(sim.Backlog) {
			ticket := sim.Backlog[a.selected]
			a.engine.TryDecompose(ticket.ID)
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
		if a.client != nil {
			a.statusMessage = "Comparison requires local mode (run without --client)"
			a.statusExpiry = time.Now().Add(3 * time.Second)
			return a, nil
		}
		a.runComparison()
		a.currentView = ViewComparison
		return a, nil

	case "e":
		// Export simulation data (requires engine mode for local data)
		if a.client != nil {
			a.statusMessage = "Export requires local mode (run without --client)"
			a.statusExpiry = time.Now().Add(3 * time.Second)
			return a, nil
		}
		sim := a.engine.Sim()
		if len(sim.CompletedTickets) == 0 {
			a.statusMessage = "Nothing to export - no completed tickets"
			a.statusExpiry = time.Now().Add(3 * time.Second)
			return a, nil
		}
		exporter := export.New(&sim, a.tracker, a.comparisonResult)
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
		if a.client != nil {
			a.statusMessage = "Save requires local mode (run without --client)"
			a.statusExpiry = time.Now().Add(3 * time.Second)
			return a, nil
		}
		sim := a.engine.Sim()
		saveName := fmt.Sprintf("sim-%d-%s", sim.Seed, time.Now().Format("150405"))
		savePath := persistence.GenerateSavePath(persistence.DefaultSavesDir(), saveName)
		err := persistence.Save(savePath, saveName, &sim, a.tracker)
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
		if a.client != nil {
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
		// Restore state
		a.tracker = tracker
		// Ensure simulation has ID for event sourcing
		if sim.ID == "" {
			sim.ID = fmt.Sprintf("sim-%d", sim.Seed)
		}
		// Re-register with shared registry if available, else use standalone store
		if !a.registry.IsZero() {
			a.store = a.registry.Store()
			a.engine = a.registry.RegisterSimulation(sim, tracker)
			// RegisterSimulation already calls EmitLoadedState
		} else {
			a.store = events.NewMemoryStore()
			a.engine = engine.NewEngineWithStore(sim.Seed, a.store)
			// Emit events to populate projection from loaded state
			a.engine.EmitLoadedState(*sim)
		}
		// Re-subscribe to new simulation's events
		a.eventSub = a.store.Subscribe(sim.ID)
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
	simA := a.createSimulation(model.PolicyDORAStrict, seed)
	engA := engine.NewEngine(simA.Seed)
	engA.EmitLoadedState(*simA)
	trackerA := metrics.NewTracker()

	// Run simulation with TameFlow-Cognitive policy
	simB := a.createSimulation(model.PolicyTameFlowCognitive, seed)
	engB := engine.NewEngine(simB.Seed)
	engB.EmitLoadedState(*simB)
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

// createSimulation creates a fresh simulation with identical setup
func (a *App) createSimulation(policy model.SizingPolicy, seed int64) *model.Simulation {
	sim := model.NewSimulation(policy, seed)

	// Same team
	sim.AddDeveloper(model.NewDeveloper("dev-1", "Alice", 1.0))
	sim.AddDeveloper(model.NewDeveloper("dev-2", "Bob", 0.8))
	sim.AddDeveloper(model.NewDeveloper("dev-3", "Carol", 1.2))

	// Same backlog (using same seed)
	gen := engine.Scenarios["mixed"] // Use mixed for more interesting comparison
	rng := rand.New(rand.NewSource(seed))
	tickets := gen.Generate(rng, 15)
	for _, t := range tickets {
		sim.AddTicket(t)
	}

	return sim
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
			if children, decomposed := eng.TryDecompose(ticket.ID); decomposed {
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
				if children, decomposed := eng.TryDecompose(ticket.ID); decomposed {
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
func (a *App) doHTTPTick() tea.Cmd {
	return func() tea.Msg {
		resp, err := a.client.Tick(a.simID)
		if err != nil {
			return httpResultMsg{operation: "tick", err: err}
		}
		return httpResultMsg{operation: "tick", state: resp.Simulation}
	}
}

// doHTTPStartSprint returns a Cmd that makes an HTTP start sprint request.
func (a *App) doHTTPStartSprint() tea.Cmd {
	return func() tea.Msg {
		resp, err := a.client.StartSprint(a.simID)
		if err != nil {
			return httpResultMsg{operation: "sprint", err: err}
		}
		return httpResultMsg{operation: "sprint", state: resp.Simulation}
	}
}

// doHTTPAssign returns a Cmd that makes an HTTP assign request.
func (a *App) doHTTPAssign(ticketID, devID string) tea.Cmd {
	return func() tea.Msg {
		err := a.client.Assign(a.simID, ticketID, devID)
		if err != nil {
			return httpResultMsg{operation: "assign", err: err}
		}
		// Need to fetch updated state
		resp, err := a.client.GetSimulation(a.simID)
		if err != nil {
			return httpResultMsg{operation: "assign", err: err}
		}
		return httpResultMsg{operation: "assign", state: resp.Simulation}
	}
}

// doHTTPSetPolicy returns a Cmd that makes an HTTP set policy request.
func (a *App) doHTTPSetPolicy(policy string) tea.Cmd {
	return func() tea.Msg {
		resp, err := a.client.SetPolicy(a.simID, policy)
		if err != nil {
			return httpResultMsg{operation: "policy", err: err}
		}
		return httpResultMsg{operation: "policy", state: resp.Simulation}
	}
}

// doHTTPDecompose returns a Cmd that makes an HTTP decompose request.
func (a *App) doHTTPDecompose(ticketID string) tea.Cmd {
	return func() tea.Msg {
		resp, err := a.client.Decompose(a.simID, ticketID)
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
		if a.client != nil {
			hasActiveSprint = a.state.SprintActive
		} else {
			sim := a.engine.Sim()
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

	// Get values from state source (client mode vs engine mode)
	var policy string
	var currentTick, backlogCount, completedCount int
	var seed int64

	if a.client != nil {
		// Client mode: use HTTP state
		policy = a.state.SizingPolicy
		currentTick = a.state.CurrentTick
		backlogCount = a.state.BacklogCount
		completedCount = a.state.CompletedTicketCount
		seed = a.state.Seed
	} else {
		// Engine mode: use local simulation
		sim := a.engine.Sim()
		policy = sim.SizingPolicy.String()
		currentTick = sim.CurrentTick
		backlogCount = len(sim.Backlog)
		completedCount = len(sim.CompletedTickets)
		seed = sim.Seed
	}

	policyStr := fmt.Sprintf("Policy: %s", policy)
	status := "PAUSED"
	if !a.paused {
		status = "RUNNING"
	}

	right := MutedStyle.Render(fmt.Sprintf("%s | %s | Day %d | Backlog: %d | Done: %d | Seed %d", policyStr, status, currentTick, backlogCount, completedCount, seed))

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
