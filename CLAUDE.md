@/home/ted/projects/share/tandem-protocol/tandem-protocol.md

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

## Development Environment

- **Language**: Go
- **Package Management**: Nix flakes

## Code Style: FluentFP

Use `github.com/binaryphile/fluentfp` for data transformation. **Full guide:** `docs/fluentfp-guide.md`
```go
// Filter and count (2 ops = multiline)
count := slice.From(tickets).
    KeepIf(Ticket.IsActive).
    Len()

// Extract field (1 op = single line)
ids := slice.From(tickets).ToString(Ticket.GetID)

// Optional values
assignee := option.Of(ticket.AssignedTo).Or("unassigned")
```

### Conventions

- **Chain formatting**: 1 operation = single line; 2+ = multiline with trailing dot
- **Prefer**: method expressions > named functions > inline lambdas
- **Named functions**: leading comment, single-expression on one line
- **Accessors for FluentFP**: Add `Get*` methods to types used in ToFloat64/Unzip operations
- **Invariants**: Use `must.Get` when errors indicate bugs, not runtime conditions

### When NOT to Use

- Performance-critical hot paths (use plain loops)
- Complex control flow (break/continue/early return)
- Channel consumption

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
