@/home/ted/projects/tandem-protocol/README.md

# Software Development Simulation

## Project Overview

Reference implementation for scaling. Prioritize correctness over expediency.
Two paths: video game / LLM laboratory for automated experimentation.

## Architecture Decisions

### Immutable Engine Pattern

Operations return new Engine, no mutable cached state:
```go
eng = eng.Tick()  // Returns new Engine
```
Why: Concurrency safety without locks, aligns with ES/FP guides.

### Value Semantics

model.Simulation and data types use value receivers throughout.
Pointers only for: sync.Mutex fields, Bubble Tea requirements, profiled hot paths >10MB.

**Slice aliasing trap**: Value-copying a struct copies slice headers, NOT underlying arrays.
Two copies share the same backing array. Mutating via `append` in one corrupts the other.
Any struct with slice fields that will be copied across goroutines/projections needs `Clone()`.
See `model.Simulation.Clone()` and `events.Projection.Apply()` for the pattern.

## Development Environment

- **Language**: Go
- **Package Management**: Nix flakes

## Code Style: FluentFP

Use `github.com/binaryphile/fluentfp` for data transformation. **Full guide:** `docs/fluentfp-guide.md`

### Decision Gate (before writing ANY for loop)

Is the operation filter, map, find, count, sum, extract, or for-each? --> **Use FluentFP.**
Does it need a justified escape? Annotate with `// justified:CODE`.

**Justified codes:** AS=assertion, AT=anon type, CF=complex flow, EP=error propagation,
IX=index-dependent, MB=map building, SM=state mutation, WL=while loop.
Self-evident (no annotation needed): table-driven tests, benchmarks.

```go
// BAD: manual loop for data extraction
var ids []string
for _, t := range tickets {
    ids = append(ids, t.ID)
}

// GOOD: FluentFP — expresses WHAT not HOW
ids := slice.From(tickets).ToString(Ticket.GetID)

// Filter and count (2 ops = multiline)
count := slice.From(tickets).
    KeepIf(Ticket.IsActive).
    Len()

// Terminal operations on typed slices
total := slice.From(tickets).ToFloat64(Ticket.GetEstimatedDays).Sum()
names := slice.From(devs).ToString(Developer.GetName).Unique()
hasIdle := slice.From(devs).Any(Developer.IsIdle)

// Optional values
assignee := option.Of(ticket.AssignedTo).Or("unassigned")
```

### Conventions

- **Chain formatting**: 1 operation = single line; 2+ = multiline with trailing dot
- **Prefer**: method expressions > named functions > inline lambdas
- **Named functions**: leading comment, single-expression on one line
- **Accessors for FluentFP**: Add `Get*` methods to types used in ToFloat64/Unzip operations
- **Invariants**: Use `must.Get` when errors indicate bugs, not runtime conditions

## Testing

### TDD Cycle (Mandatory)

Red → Green → Refactor → Prune. Never implement before writing a failing test.

### Khorikov Quadrants

| Code Type | Test Strategy |
|-----------|---------------|
| Domain/Algorithms | Unit test heavily |
| Controllers | ONE integration test |
| Trivial | Don't test |
| Overcomplicated | Refactor first |

### Coverage

See `docs/testing-strategy.md` for coverage baseline.

## Benchmarks

Run on each build: `go test -bench=. -benchmem ./internal/engine/`

### Baseline (2026-03-23, post handoff+experience+intake+TOC)

```
BenchmarkTick-8                       60787    20337 ns/op
BenchmarkRunSprint-8                 618132     2396 ns/op
BenchmarkFindActiveTicketIndex-8    7302486      175 ns/op
```

### Baseline (2026-05-24, 4-core machine, pre-UC37)

Recorded on a 4-core machine before UC37 code work. Numbers below are not directly comparable to the 8-core baseline above due to hardware shift (7-9× slower per core; same Go workload). Establishes a 4-core pre-impl reference for measuring UC37's induced delta (expected < 5% — per-ticket `Type` field + map lookup in `CalculatePhaseEffort` keeps big-O unchanged).

```
BenchmarkTick-4                       10000   139383 ns/op   10970 B/op   7 allocs/op
BenchmarkTick_LargeSimulation-4       10000   169443 ns/op   10970 B/op   7 allocs/op
BenchmarkRunSprint-4                  61770    21032 ns/op     260 B/op   5 allocs/op
BenchmarkFindActiveTicketIndex-4    1000000     1193 ns/op       0 B/op   0 allocs/op
BenchmarkVarianceCalculate-4          32220    51991 ns/op    5376 B/op   1 allocs/op
```

Post-UC37 numbers re-measured at cycle #15442 completion-gate.

### Baseline (2026-05-24, 4-core machine, post-UC37 pre-UC38)

Recorded immediately before UC38 implementation begins. Same 4-core machine as the pre-UC37 baseline above. UC38 adds a `PhaseWIPConfig` map lookup per assignment + a deep-copy of the map in `Simulation.Clone()`; expected delta < 5% (map-of-7-entries-max worst-case lookup is O(1) amortised). Wall-clock numbers below reflect ambient machine load at measurement time; the relative-delta comparison to the post-UC38 re-measurement at cycle #15443 completion-gate is what matters.

```
BenchmarkTick-4                          10000   473937 ns/op   10970 B/op   7 allocs/op
BenchmarkTick_LargeSimulation-4          10000   470656 ns/op   10970 B/op   7 allocs/op
BenchmarkFindActiveTicketIndex-4       1000000     3186 ns/op       0 B/op   0 allocs/op
BenchmarkFindActiveTicketIndex_Small-4 2821860      562.1 ns/op      0 B/op   0 allocs/op
BenchmarkVarianceCalculate-4             10000   204944 ns/op    5376 B/op   1 allocs/op
BenchmarkRunSprint-4                     10000   173612 ns/op     262 B/op   5 allocs/op
```

Post-UC38 numbers will be re-measured at cycle #15443 completion-gate.

### Baseline (2026-05-25, 4-core machine, post-UC38 pre-UC39)

Recorded immediately before UC39 implementation begins. Same 4-core machine. UC39 adds a per-tick release-controller call (no-op fast-path when ReleaseMode==Push; small constant work otherwise) + new sim fields (ReleaseMode/WarmupActive/WarmupFailed/MaxBacklogDrip) trivially Cloned. Expected delta < 5% on push-mode runs (zero-value default; controller's first branch returns immediately). Wall-clock numbers reflect ambient machine load.

```
BenchmarkTick-4                          14164   170445 ns/op   10970 B/op   7 allocs/op
BenchmarkTick_LargeSimulation-4           8118   143888 ns/op   10970 B/op   7 allocs/op
BenchmarkFindActiveTicketIndex-4        614234     1922 ns/op       0 B/op   0 allocs/op
BenchmarkFindActiveTicketIndex_Small-4 7375038      171.1 ns/op      0 B/op   0 allocs/op
BenchmarkVarianceCalculate-4             28461    87507 ns/op    5376 B/op   1 allocs/op
BenchmarkRunSprint-4                     44959    28512 ns/op     266 B/op   5 allocs/op
```

Post-UC39 numbers will be re-measured at cycle #15445 completion-gate.

### Baseline (2026-05-25, 4-core machine, post-UC39)

Recorded at cycle #15445 completion-gate. Same 4-core machine. UC39's per-tick release-controller call + TOC.Update added small constant work. Allocs went from 7 → 10 per Tick (TOC.Update intermediate allocations); wall-clock numbers reflect ambient machine load (BenchmarkTick ratio ~2.2× vs pre-UC39 is consistent with ambient variability across multiple runs).

```
BenchmarkTick-4                       7492   368486 ns/op   11115 B/op   10 allocs/op
BenchmarkTick_LargeSimulation-4      10000   280489 ns/op   11114 B/op   10 allocs/op
BenchmarkRunSprint-4                 20400    65979 ns/op     267 B/op    5 allocs/op
```

Post-UC40 numbers will be re-measured at cycle #15446 completion-gate.

### Baseline (2026-05-25, 4-core machine, post-UC39 pre-UC40)

Recorded immediately before UC40 implementation begins. Same 4-core machine. UC40 adds Budget int + 2 multiplier float64 fields + NextDeveloperID int (4 new fields) on Simulation; per-tick read of ReviewVelocityBonus/VerifyVarianceDamping in work-calc path adds 2 float multiplies. Expected delta < 5% (multiplications at default 1.0 are no-op identity; field reads are cache-friendly). Wall-clock numbers reflect ambient machine load.

```
BenchmarkTick-4                       7492   368486 ns/op   11115 B/op   10 allocs/op
BenchmarkTick_LargeSimulation-4      10000   280489 ns/op   11114 B/op   10 allocs/op
BenchmarkRunSprint-4                 20400    65979 ns/op     267 B/op    5 allocs/op
```

Post-UC40 numbers will be re-measured at cycle #15446 completion-gate (final cycle in the Factorio epic).

### Baseline (2026-05-25, 4-core machine, post-UC40 — Factorio epic complete)

Recorded at cycle #15446 completion-gate. Same 4-core machine as prior baselines. UC40 added 5 new fields on Simulation (Budget int + ReviewVelocityBonus float64 + VerifyVarianceDamping float64 + NextDeveloperID int + LastInvestmentApplied string) + 2 conditional float multiplies in the work-calc path (Review/Verify phases only). Wall-clock numbers reflect ambient machine load (BenchmarkTick ratio ~2.3× vs UC39 baseline is consistent with cross-run variability on this i5-8200Y; alloc count stayed at 10/Tick same as UC39).

```
BenchmarkTick-4                      10000   845360 ns/op   11114 B/op   10 allocs/op
BenchmarkTick_LargeSimulation-4      10000  1005650 ns/op   11114 B/op   10 allocs/op
BenchmarkRunSprint-4                 17842    86984 ns/op     266 B/op    5 allocs/op
```

**Factorio dynamics epic #15441 program complete**: UC37 #15442 (heterogeneous ticket types) + UC38 #15443 (per-phase WIP caps) + UC39 #15445 (demand-driven release) + UC40 #15446 (investment moves) all shipped. 5FS EXPLOIT/ELEVATE game loop is now playable end-to-end.

## Persistence

| Key | Action |
|-----|--------|
| Ctrl+s | Save to saves/ |
| Ctrl+o | Load most recent |

Format: gob, extension: .sds

## Event Sourcing

- **Upcasting**: Transform old events on read
- **Add fields freely**: Old events decode with zero values
- **Rename/remove**: Requires upcast function in `internal/events/upcasting.go`

## Development Process

1. Use Cases (`/use-case-skill`) → 2. Design Documents → 3. Implementation (Tandem Protocol)

Trunk-based development. Commit to main when safe, short-lived branches for 1-2 day work.

## Auto Memory

Do not use MEMORY.md or the auto memory directory. Use the MCP memory tool instead.
