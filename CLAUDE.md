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

### Baseline (2026-01-15)

```
BenchmarkTick-8                       63327    18776 ns/op
BenchmarkProjection_Apply-8        22574644       45 ns/op
BenchmarkProjection_ReplayFull-8      29217    36419 ns/op
```

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
