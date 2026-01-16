package tui

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/engine"
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
	// Simulation state
	sim     *model.Simulation
	engine  *engine.Engine
	tracker *metrics.Tracker

	// UI state
	currentView View
	paused      bool
	speed       int // ticks per update
	selected    int // selected item in lists

	// Events log
	events []model.Event

	// Comparison mode
	comparisonResult *metrics.ComparisonResult
	comparisonSeed   int64

	// Dimensions
	width, height int

	// Tick timer
	tickInterval time.Duration

	// Status message (for export feedback)
	statusMessage string
	statusExpiry  time.Time
}

// tickMsg is sent on each simulation tick
type tickMsg time.Time

// NewAppWithSeed creates a new App with the specified random seed.
// If seed is 0, uses current time for randomness.
func NewAppWithSeed(seed int64) *App {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	sim := model.NewSimulation(model.PolicyDORAStrict, seed)

	// Add default team
	sim.AddDeveloper(model.NewDeveloper("dev-1", "Alice", 1.0))
	sim.AddDeveloper(model.NewDeveloper("dev-2", "Bob", 0.8))
	sim.AddDeveloper(model.NewDeveloper("dev-3", "Carol", 1.2))

	// Generate initial backlog
	gen := engine.Scenarios["healthy"]
	rng := rand.New(rand.NewSource(seed))
	tickets := gen.Generate(rng, 12)
	for _, t := range tickets {
		sim.AddTicket(t)
	}

	eng := engine.NewEngine(sim)
	tracker := metrics.NewTracker()

	return &App{
		sim:          sim,
		engine:       eng,
		tracker:      tracker,
		currentView:  ViewPlanning,
		paused:       true,
		speed:        1,
		events:       make([]model.Event, 0),
		tickInterval: 500 * time.Millisecond,
	}
}

// Init implements tea.Model
func (a *App) Init() tea.Cmd {
	return nil
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

	case tickMsg:
		if !a.paused && a.currentView == ViewExecution {
			events := a.engine.Tick()
			a.events = append(a.events, events...)
			a.tracker.Update(a.sim)

			// End sprint when duration reached
			if sprint, ok := a.sim.CurrentSprintOption.Get(); ok && a.sim.CurrentTick >= sprint.EndDay {
				a.sim.CurrentSprintOption = model.NoSprint // Clear sprint
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
		a.sim.SizingPolicy = (a.sim.SizingPolicy + 1) % 4
		return a, nil

	case "s":
		// Start sprint (from planning view)
		if _, ok := a.sim.CurrentSprintOption.Get(); a.currentView == ViewPlanning && !ok {
			a.sim.StartSprint()
			a.currentView = ViewExecution
			a.paused = false
			return a, a.tickCmd()
		}
		return a, nil

	case "a":
		// Assign selected ticket to first idle developer
		if a.currentView == ViewPlanning && a.selected < len(a.sim.Backlog) {
			ticket := a.sim.Backlog[a.selected]
			for _, dev := range a.sim.Developers {
				if dev.IsIdle() {
					a.engine.AssignTicket(ticket.ID, dev.ID)
					break
				}
			}
		}
		return a, nil

	case "d":
		// Decompose selected ticket
		if a.currentView == ViewPlanning && a.selected < len(a.sim.Backlog) {
			ticket := a.sim.Backlog[a.selected]
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
		// Run comparison mode
		a.runComparison()
		a.currentView = ViewComparison
		return a, nil

	case "e":
		// Export simulation data
		if len(a.sim.CompletedTickets) == 0 {
			a.statusMessage = "Nothing to export - no completed tickets"
			a.statusExpiry = time.Now().Add(3 * time.Second)
			return a, nil
		}
		exporter := export.New(a.sim, a.tracker, a.comparisonResult)
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
		// Save simulation state
		saveName := fmt.Sprintf("sim-%d-%s", a.sim.Seed, time.Now().Format("150405"))
		savePath := persistence.GenerateSavePath(persistence.DefaultSavesDir(), saveName)
		err := persistence.Save(savePath, saveName, a.sim, a.tracker)
		if err != nil {
			a.statusMessage = fmt.Sprintf("Save failed: %v", err)
			a.statusExpiry = time.Now().Add(5 * time.Second)
			return a, nil
		}
		a.statusMessage = fmt.Sprintf("Saved to %s", savePath)
		a.statusExpiry = time.Now().Add(3 * time.Second)
		return a, nil

	case "ctrl+o":
		// Load simulation state (most recent save)
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
		a.sim = sim
		a.tracker = tracker
		a.engine = engine.NewEngine(sim)
		a.paused = true
		a.statusMessage = fmt.Sprintf("Loaded %s (Day %d)", filepath.Base(latest.Path), sim.CurrentTick)
		a.statusExpiry = time.Now().Add(3 * time.Second)
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
	engA := engine.NewEngine(simA)
	trackerA := metrics.NewTracker()

	// Run simulation with TameFlow-Cognitive policy
	simB := a.createSimulation(model.PolicyTameFlowCognitive, seed)
	engB := engine.NewEngine(simB)
	trackerB := metrics.NewTracker()

	// Run 3 sprints each
	for i := 0; i < 3; i++ {
		a.runSprintWithAutoAssign(simA, engA, trackerA)
		a.runSprintWithAutoAssign(simB, engB, trackerB)
	}

	// Get results and compare
	resultA := trackerA.GetResult(model.PolicyDORAStrict, simA)
	resultB := trackerB.GetResult(model.PolicyTameFlowCognitive, simB)

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
func (a *App) runSprintWithAutoAssign(sim *model.Simulation, eng *engine.Engine, tracker *metrics.Tracker) {
	sim.StartSprint()

	// Auto-assign tickets to idle developers at start
	for _, dev := range sim.Developers {
		if dev.IsIdle() && len(sim.Backlog) > 0 {
			ticket := sim.Backlog[0]
			// Try decomposition first based on policy
			if children, decomposed := eng.TryDecompose(ticket.ID); decomposed {
				// Assign first child
				if len(children) > 0 {
					eng.AssignTicket(children[0].ID, dev.ID)
				}
			} else {
				eng.AssignTicket(ticket.ID, dev.ID)
			}
		}
	}

	// Run the sprint
	sprint, _ := sim.CurrentSprintOption.Get()
	for sim.CurrentTick < sprint.EndDay {
		eng.Tick()
		tracker.Update(sim)

		// Re-assign idle developers mid-sprint
		for i := range sim.Developers {
			dev := &sim.Developers[i]
			if dev.IsIdle() && len(sim.Backlog) > 0 {
				ticket := sim.Backlog[0]
				if children, decomposed := eng.TryDecompose(ticket.ID); decomposed {
					if len(children) > 0 {
						eng.AssignTicket(children[0].ID, dev.ID)
					}
				} else {
					eng.AssignTicket(ticket.ID, dev.ID)
				}
			}
		}
	}
}

func (a *App) tickCmd() tea.Cmd {
	return tea.Tick(a.tickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
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

	policy := fmt.Sprintf("Policy: %s", a.sim.SizingPolicy)
	status := "PAUSED"
	if !a.paused {
		status = "RUNNING"
	}

	right := MutedStyle.Render(fmt.Sprintf("%s | %s | Day %d | Backlog: %d | Done: %d | Seed %d", policy, status, a.sim.CurrentTick, len(a.sim.Backlog), len(a.sim.CompletedTickets), a.sim.Seed))

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
