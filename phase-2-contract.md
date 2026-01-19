# Phase 2 Contract: Full Event Sourcing

**Created:** 2026-01-19

## Step 1 Checklist
- [x] 1a: Presented understanding
- [x] 1b: Asked clarifying questions
- [x] 1b-answer: Received answers (one phase, strict TDD)
- [x] 1c: Contract created (this file)
- [x] 1d: Approval received
- [x] 1e: Plan + contract archived

## Objective

Make events THE source of truth. Currently hybrid (emit + mutate) - target is emit-only with state derived from Projection.

## Success Criteria

- [x] Projection.Apply() handles all 12 event types (pure, no mutation)
- [ ] Engine uses Projection (no `*Simulation` field)
- [ ] API uses `engine.Sim()` (not direct `*sim` read)
- [ ] TUI derives state from Projection
- [x] All existing tests pass
- [x] `BenchmarkProjection_Apply_SingleEvent` < 1μs/op (actual: 46ns/op)
- [x] `BenchmarkProjection_ReplayFull` (1000 events) < 1ms (actual: 38μs)
- [ ] No coverage regression

## Event Type Inventory

**Existing events (8):**
| Event | Purpose | Projection Action |
|-------|---------|-------------------|
| `SimulationCreated` | Initialize simulation | Set ID, policy, seed, init slices |
| `Ticked` | Advance tick | Increment CurrentTick |
| `SprintStarted` | Begin sprint | Set CurrentSprintOption |
| `SprintEnded` | End sprint | Clear CurrentSprintOption |
| `TicketAssigned` | Assign ticket | Move backlog→active, set developer.CurrentTicket |
| `TicketCompleted` | Complete ticket | Move active→completed, clear developer.CurrentTicket |
| `IncidentStarted` | Start incident | Add to ActiveIncidents |
| `IncidentResolved` | Resolve incident | Move to ResolvedIncidents |

**New events to add (4):**
| Event | Purpose | Projection Action |
|-------|---------|-------------------|
| `DeveloperAdded` | Add team member | Append to Developers slice |
| `TicketCreated` | Create ticket | Append to Backlog slice |
| `WorkProgressed` | Apply effort | Update RemainingEffort, ActualDays |
| `TicketPhaseChanged` | Advance phase | Update ticket.Phase |

**Total: 12 events**

## TUI Event Notification

TUI already has subscription mechanism:
- `store.Subscribe(simID)` returns `<-chan Event`
- `store.Replay(simID)` returns `[]Event` for initial state
- TUI applies events to local Projection when received via channel

No new notification mechanism needed - just change TUI to use Projection instead of `*Simulation`.

## Approach

**Strict TDD:** Write failing test before ANY implementation code.

| Step | Scope | TDD Cycle |
|------|-------|-----------|
| 1 | Projection type | Test Apply() per event type FIRST |
| 2 | Event types | Add DeveloperAdded, TicketCreated, WorkProgressed, TicketPhaseChanged |
| 3 | Engine refactor | Remove `*Simulation`, use Projection |
| 4 | API refactor | Use `engine.Sim()`, remove `inst.sim` |
| 5 | TUI refactor | Derive state from Projection |
| 6 | Documentation | Update docs/design.md |

## Token Budget

Estimated: 100-120K tokens (revised up from 80-100K)
- Projection type + tests: 35K (TDD cycles × 12 event types)
- Event types: 10K
- Engine refactor: 20K (significant changes)
- API refactor: 10K
- TUI refactor: 15K (touches tea.Model)
- Tests + iteration: 20K
- Documentation: 5K

## Intermediate Commit Strategy

Commit after each step passes tests (allows git revert per step):

| Step | Commit Message | Checkpoint |
|------|----------------|------------|
| 1 | `Add Projection type with Apply()` | Tests pass for all 12 events |
| 2 | `Add new event types` | Compiles, existing tests pass |
| 3 | `Engine: use Projection instead of *Simulation` | All engine tests pass |
| 4 | `API: use engine.Sim()` | All API tests pass |
| 5 | `TUI: derive state from Projection` | TUI tests pass |
| 6 | `docs: document event sourcing architecture` | N/A |

**Rollback:** If step N breaks, `git revert HEAD` returns to step N-1.

## Out of Scope

- Persistence changes (save/load)
- New API endpoints
- Performance optimization beyond benchmark gates

## Files to Change

| File | Change |
|------|--------|
| `internal/events/projection.go` | NEW - Projection type with Apply() |
| `internal/events/projection_test.go` | NEW - TDD tests |
| `internal/events/types.go` | Add 4 new event types |
| `internal/engine/engine.go` | Replace `*Simulation` with Projection |
| `internal/api/registry.go` | Remove `sim` from SimInstance |
| `internal/api/handlers.go` | Use `engine.Sim()` |
| `internal/tui/app.go` | Use Projection |
| `docs/design.md` | Document event sourcing |

## Step 2 Progress

### Step 2.1: Event Types ✓ COMPLETE
- Added `Policy` field to `SimConfig`
- Added 4 new event types: `DeveloperAdded`, `TicketCreated`, `WorkProgressed`, `TicketPhaseChanged`
- All with proper constructors and `EventType()` methods

### Step 2.2: Projection Type ✓ COMPLETE
- Created `internal/events/projection.go` (~150 lines)
- TDD: Wrote failing tests first for each event type
- `Projection.Apply()` handles all 12 event types
- Value semantics: returns new Projection, no mutation
- Benchmarks pass: 46ns/op single event, 38μs for 1000 events

### Step 2.2b: FluentFP Enhancement (UNPLANNED)
**Added to scope during TUI fix** - needed `IsZero()` pattern for value-type pseudo-options.
- Added `ZeroChecker` interface and `IfNotZero` to fluentfp v0.7.0
- Added `IsZero()` to `api.SimRegistry`
- Fixed TUI to use `!registry.IsZero()` pattern
- Updated Go Development Guide with pseudo-option conventions

### Pending Steps
- Step 2.3: Engine refactor
- Step 2.4: API refactor
- Step 2.5: TUI refactor (partial - fixed SimRegistry, still needs Projection)
- Step 2.6: Documentation
