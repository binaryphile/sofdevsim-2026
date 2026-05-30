package batch

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/export"
	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
	"github.com/binaryphile/sofdevsim-2026/internal/registry"
)

// Runner executes a batch experiment per the supplied Config.
//
// Field StoreFactory is the per-run isolation seam (P3 mitigation per
// CQRS/ES guide §11 event-store-per-aggregate-lifetime + /grade R2-F3
// pointer-distinctness contract). Default: events.NewMemoryStore.
//
// Field RegistryFactory is retained as a test seam for callers that
// want to verify per-run registry isolation, but is NOT consumed by
// Run() — the 3a→1c loopback (interaction id 21885 → contract #21890
// supersedes #21828) switched the construction path to engine-direct
// (matching runComparison at internal/api/handlers.go:418-433) because
// registry.CreateSimulation hardcodes a 6-default team + 12-default
// backlog (internal/registry/registry.go:143-160) incompatible with
// the team_size + scenario contract.
type Runner struct {
	StoreFactory    func() events.Store
	RegistryFactory func() *registry.SimRegistry
}

// NewRunner constructs a Runner with default factories.
func NewRunner() *Runner {
	return &Runner{
		StoreFactory:    func() events.Store { return events.NewMemoryStore() },
		RegistryFactory: registry.NewSimRegistry,
	}
}

// Run executes the batch experiment described by cfg and writes outputs
// to outDir.
//
// Run() error semantics (per /i I2):
//   - returns non-nil error ONLY if (a) config validation fails (before
//     any run starts) OR (b) outDir setup fails (before any run starts)
//     OR (c) writing runs.csv/experiment.json fails (after all runs)
//   - returns NIL error with Results populated when individual per-run
//     failures occur (captured as Results.Runs[i].Status="failed" + .Error)
//   - batch continues past per-run failures so partial results are
//     inspectable; caller decides whether to treat all-runs-failed as
//     success or surface separately.
func (r *Runner) Run(cfg Config, outDir string) (Results, error) {
	if err := cfg.Validate(); err != nil {
		return Results{}, err
	}
	// Validated above; ignored errors are unreachable.
	policy, _ := cfg.ResolvePolicy()
	releaseMode, _ := cfg.ResolveReleaseMode()
	phaseWIPConfig, _ := cfg.ResolvePhaseWIPCaps()
	seeds := cfg.ResolveSeeds()

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return Results{}, fmt.Errorf("create outDir: %w", err)
	}

	// Provenance lands before per-run work so even a fatal mid-batch panic
	// leaves the audit trail intact.
	startedAt := time.Now().UTC().Format(time.RFC3339)
	if err := WriteExperimentJSON(outDir, cfg, captureGitSHA(), startedAt); err != nil {
		return Results{}, err
	}

	runs := make([]RunResult, 0, len(seeds))
	for i, seed := range seeds {
		runs = append(runs, r.runOne(i, seed, cfg, policy, phaseWIPConfig, releaseMode, outDir))
	}

	if err := WriteRunsCSV(outDir, runs); err != nil {
		return Results{Runs: runs}, err
	}
	return Results{Runs: runs}, nil
}

// runOne executes a single simulation against the supplied seed and
// returns a RunResult capturing the outcome. Per-run failures are
// recorded in the RunResult — they do NOT propagate as errors so the
// batch can continue past them.
func (r *Runner) runOne(
	i int,
	seed int64,
	cfg Config,
	policy model.SizingPolicy,
	phaseWIPConfig map[model.WorkflowPhase]int,
	releaseMode model.ReleaseMode,
	baseOutDir string,
) RunResult {
	rr := RunResult{
		Index:    i,
		Seed:     seed,
		Policy:   cfg.Policy,
		Scenario: cfg.Scenario,
		Status:   "succeeded",
	}

	runDir := filepath.Join(baseOutDir, fmt.Sprintf("run-%d-seed-%d", i, seed))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		rr.Status = "failed"
		rr.Error = "mkdir runDir: " + err.Error()
		rr.OutputDir = runDir
		return rr
	}
	rr.OutputDir = runDir

	// Defer-recover so a panic in any engine call (or scenario generator)
	// is captured into rr.Error rather than aborting the batch. Per-run
	// failure semantics per criterion 3.
	defer func() {
		if p := recover(); p != nil {
			rr.Status = "failed"
			rr.Error = fmt.Sprintf("panic: %v", p)
		}
	}()

	// Engine-direct construction per 3a→1c supersedes. Per-run isolation
	// (P3) via fresh StoreFactory() call — captured by isolation test.
	store := r.StoreFactory()
	eng := engine.NewEngineWithStore(seed, store)

	simID := fmt.Sprintf("sim-%d", seed)
	const sprintLength = 10 // model.NewSimulation default — kept here so we
	// don't construct a throwaway model.Simulation just to read this constant.

	var err error
	eng, err = eng.EmitCreated(simID, 0, events.SimConfig{
		TeamSize:       cfg.TeamSize,
		SprintLength:   sprintLength,
		Seed:           seed,
		Policy:         policy,
		PhaseWIPConfig: phaseWIPConfig,
		ReleaseMode:    releaseMode,
	})
	if err != nil {
		rr.Status = "failed"
		rr.Error = "emit created: " + err.Error()
		return rr
	}

	// Add cfg.TeamSize developers at uniform velocity 1.0. Per-dev velocity
	// customization is deferred to fu3 (task #21834).
	for d := 1; d <= cfg.TeamSize; d++ {
		id := fmt.Sprintf("dev-%d", d)
		eng, err = eng.AddDeveloper(id, id, 1.0)
		if err != nil {
			rr.Status = "failed"
			rr.Error = "add developer: " + err.Error()
			return rr
		}
	}

	// Generate backlog via scenario generator with its built-in default
	// ticket count. Custom counts deferred to fu5 (task #21836).
	gen := engine.Scenarios[cfg.Scenario]
	rng := rand.New(rand.NewSource(seed))
	for _, t := range gen.Generate(rng, gen.TicketsPerSprint) {
		eng, err = eng.AddTicket(t)
		if err != nil {
			rr.Status = "failed"
			rr.Error = "add ticket: " + err.Error()
			return rr
		}
	}

	// Run sprints. Loop mirrors runSprintsWithTracking precedent at
	// internal/api/handlers.go:394-414.
	tracker := metrics.NewTracker()
	for s := 0; s < cfg.Sprints; s++ {
		eng = decomposeEligible(eng)
		eng, err = eng.StartSprint()
		if err != nil {
			rr.Status = "failed"
			rr.Error = fmt.Sprintf("sprint %d StartSprint: %v", s, err)
			return rr
		}
		eng = autoAssignIdle(eng)
		sprint, ok := eng.Sim().CurrentSprintOption.Get()
		if !ok {
			rr.Status = "failed"
			rr.Error = fmt.Sprintf("sprint %d: no active sprint after StartSprint", s)
			return rr
		}
		for eng.Sim().CurrentTick < sprint.EndDay {
			var tickErr error
			eng, _, tickErr = eng.Tick()
			if tickErr != nil {
				rr.Status = "failed"
				rr.Error = fmt.Sprintf("sprint %d tick: %v", s, tickErr)
				return rr
			}
			eng = autoAssignIdle(eng)
		}
		tracker = tracker.Updated(eng.Sim())
		rr.SprintsRun++
	}

	// Per-run export to runDir; the export package creates a nested
	// sofdevsim-export-<timestamp>/ subdir inside runDir per writers.go:64.
	res, err := export.New(eng.Sim(), tracker, nil).ExportTo(runDir)
	if err != nil {
		rr.Status = "failed"
		rr.Error = "export: " + err.Error()
		return rr
	}
	rr.OutputDir = res.Path // record the nested-dir path so consumers can find CSVs
	return rr
}

// decomposeEligible decomposes all backlog tickets matching policy
// criteria. Mirror of internal/api/handlers.go::decomposeEligibleTickets.
// Idempotent; safe to call repeatedly.
func decomposeEligible(eng engine.Engine) engine.Engine {
	for {
		anyDecomposed := false
		state := eng.Sim()
		for _, ticket := range state.Backlog {
			var (
				err error
				res any
			)
			eng, res, err = tryDecomposeOnce(eng, ticket.ID)
			if err != nil {
				return eng
			}
			if res != nil {
				anyDecomposed = true
				break // backlog changed, re-scan from start
			}
		}
		if !anyDecomposed {
			break
		}
	}
	return eng
}

// tryDecomposeOnce wraps engine.TryDecompose's either-typed result to a
// simpler (engine, decomposed-or-nil, error) tuple for the batch loop.
func tryDecomposeOnce(eng engine.Engine, ticketID string) (engine.Engine, any, error) {
	eng2, result, err := eng.TryDecompose(ticketID)
	if err != nil {
		return eng2, nil, err
	}
	if children, ok := result.Get(); ok {
		return eng2, children, nil
	}
	return eng2, nil, nil
}

// autoAssignIdle assigns committed (or backlog) tickets to idle developers.
// Mirror of internal/api/handlers.go::autoAssignForComparison.
func autoAssignIdle(eng engine.Engine) engine.Engine {
	state := eng.Sim()
	idle := state.IdleDevelopers()
	for i := 0; i < len(idle); i++ {
		state = eng.Sim()
		var ticketID string
		if len(state.CommittedTickets) > 0 {
			ticketID = state.CommittedTickets[0].ID
		} else if len(state.Backlog) > 0 {
			ticketID = state.Backlog[0].ID
		} else {
			break
		}
		dev := idle[i]
		newEng, err := eng.AssignTicket(ticketID, dev.ID)
		if err != nil {
			return eng
		}
		eng = newEng
	}
	return eng
}
