# sofdevsim-2026 Compliance Grading

**Graded:** 2026-01-21 (Updated: 2026-01-23)
**Project:** binaryphile/sofdevsim-2026
**Files reviewed:** 33 test files, 45 source files

## Executive Scorecard

| Guide | Grade | Score | Summary |
|-------|-------|-------|---------|
| Khorikov Unit Testing | A | 99/100 | Behavior-focused tests with explicit guide references; full edge case coverage |
| Event Sourcing | A | 96/100 | Full CQRS with optimistic concurrency; minor handler mixing |
| Functional Programming | A | 96/100 | Strong ACD separation; Simulation now pure Data type |
| Go Development | A | 98/100 | Value semantics conversion done; Simulation uses value receivers only |
| **Overall** | **A** | **97/100** | Well-designed codebase with proper ACD separation |

---

## Detailed Findings

### 1. Khorikov Unit Testing Guide

**Score: 99/100**

#### Strengths

- **Explicit guide reference** in `tutorial_walkthrough_test.go:13-14`:
  ```go
  // TestTutorialWalkthrough is the ONE controller integration test for the API.
  // Per Khorikov, controllers get one integration test covering the happy path.
  ```
- **Behavior-focused tests** in `ticket_test.go` - tests verify ranges, not exact values:
  ```go
  wantMin: 0.20,
  wantMax: 0.30,  // Tests behavior: "research is quick", not implementation details
  ```
- **Table-driven tests** throughout - every test file uses the pattern consistently
- **No internal mocks** - tests use real collaborators; only boundary mocks (HTTP)
- **Two-phase testing**: Domain logic unit tested (`model/*_test.go`), controllers integration tested (`tutorial_walkthrough_test.go`)
- **AAA pattern** visible in all tests with clear Arrange/Act/Assert sections

#### Compliance Issues

**Resolved** (2026-01-23):
- ~~Some test names describe implementation rather than behavior~~ → **FIXED**: 29 tests renamed to behavior-focused names (e.g., `TestClient_Tick` → `TestClient_Tick_AdvancesSimulationTime`)

**Resolved** (2026-01-23):
- ~~Missing edge case tests~~ → **FIXED**: All 4 edge cases now covered:
  - `variance_test.go:99` - `TestVarianceModel_ZeroSeed`
  - `projection_test.go:452` - `TestProjection_ReplayManyEvents_Correctness`
  - `store_test.go:403` - `TestMemoryStore_ConcurrentAppend`
  - `engine_integration_test.go:275` - `TestEngine_SprintEndsExactlyOnBoundary`

**Nitpick** (0 points):
- `decode_test.go:133-147` - `limitedReader` test helper could be documented more clearly

#### Recommendations

| Recommendation | Effort | Impact |
|----------------|--------|--------|
| ~~Rename tests to describe behaviors, not methods~~ | ~~Quick win (<1hr)~~ | ✅ DONE (2026-01-23) |
| ~~Add seed=0 and boundary tests~~ | ~~Quick win (<1hr)~~ | ✅ DONE (2026-01-23) |
| Add concurrent append stress test | Medium (1 day) | +1 point, catches race |
| Add behavior descriptions to test table comments | Quick win (<1hr) | Clarity |

---

### 2. CQRS/Event Sourcing Guide

**Score: 96/100**

#### Strengths

- **Event immutability documented** in `events/types.go:94-100`:
  ```go
  // Event represents a domain event that occurred in a simulation.
  // All events are immutable value types.
  //
  // Per CQRS/Event Sourcing patterns (see cqrs-event-sourcing-guide.md):
  ```
- **Optimistic concurrency** in `events/store.go:50-60`:
  ```go
  // Append adds events to a simulation's event stream.
  // Uses optimistic concurrency: fails if expectedVersion doesn't match current version.
  func (m *MemoryStore) Append(simID string, expectedVersion int, events ...Event) error {
      currentVersion := len(m.events[simID])
      if expectedVersion != currentVersion {
          return fmt.Errorf("concurrency conflict: expected version %d, got %d", ...)
      }
  ```
- **Idempotent projection** in `events/projection.go:31-45`:
  ```go
  // Apply processes a single event, returning new Projection.
  // Pure function: no side effects. Creates new Projection, doesn't mutate receiver.
  // Idempotent: duplicate events (same EventID) return unchanged projection.
  func (p Projection) Apply(evt Event) Projection {
      eventID := evt.EventID()
      if eventID != "" && p.processed[eventID] {
          return p // Already processed, return unchanged
      }
  ```
- **Command/Query separation** - Engine methods emit events (commands), Projection.State() returns data (query)
- **Event upcasting** in `events/upcasting.go` for schema evolution
- **Tracing context** - events carry TraceID, SpanID, ParentSpanID for correlation
- **Tests verify replay** - `projection_test.go` covers rehydration from event stream

#### Compliance Issues

**Minor** (-2 points):
- `handlers.go:146-147`: `ToState(inst.Engine.Sim(), inst.Tracker)` - query after command in same handler
  - Technically fine but could be cleaner with dedicated read model

**Nitpick** (0 points):
- Could add event versioning for schema evolution (currently uses upcaster pattern instead)

#### Recommendations

| Recommendation | Effort | Impact |
|----------------|--------|--------|
| Extract read model from handler response building | Medium (1 day) | +2 points, cleaner CQRS |
| Add explicit event version field | Medium (1 day) | Future-proofing |

---

### 3. Functional Programming Guide

**Score: 88/100**

#### Strengths

- **Extensive ACD annotations** throughout codebase:
  - `lessons.go:68`: "Pure function: (view, state, hasActiveSprint, hasComparison) → Lesson"
  - `engine.go:23`: "variance VarianceModel // Value type: pure calculation"
  - `projection.go:29`: "Pure function: no side effects"
  - `hypermedia.go:4`: "Pure function: state -> links"
  - `decode.go:15`: "Pure calculation - returns error, no I/O"
- **Pure calculations** in variance model (`variance.go`):
  ```go
  // NewVarianceModel creates a variance model with a seed.
  // Returns value type: pure calculation, no mutation.
  func NewVarianceModel(seed int64) VarianceModel {
      return VarianceModel{seed: seed}
  }
  ```
- **Immutable projection** - `Projection.Apply()` returns new Projection, never mutates
- **Value types** - Events, Lesson, State all use copy-on-write patterns
- **Clear I/O boundaries** - Handlers are Actions, engine calculations are pure

#### Compliance Issues

**Resolved** (2026-01-22):
- ~~`model/simulation.go:58-72` - Data type had mutating methods~~ → **FIXED**: Methods removed. Simulation is now a pure Data type. All mutation goes through Engine (the Action layer).

**Minor** (-4 points):
- `engine.go:21-28` - Engine mixes pure and impure:
  ```go
  type Engine struct {
      proj     events.Projection   // Pure
      variance VarianceModel       // Pure
      evtGen   *EventGenerator     // Impure (has *rand.Rand)
      policies PolicyEngine        // Pure
      store    events.Store        // Impure (I/O)
  }
  ```
  Well-documented but could be structured to separate pure/impure more explicitly.

#### Recommendations

| Recommendation | Effort | Impact |
|----------------|--------|--------|
| ~~**Move all Simulation mutating methods to Engine**~~ | ~~Medium (1 day)~~ | ✅ DONE (2026-01-22) |
| ~~Make Simulation a pure Data type with only query methods~~ | ~~Medium (1 day)~~ | ✅ DONE (2026-01-22) |
| Document Engine as "orchestrator of pure + impure" explicitly | Quick win (<1hr) | +2 points |
| Consider splitting Engine into PureEngine + ImpureShell | Larger refactor | Architectural clarity |

---

### 4. Go Development Guide

**Score: 93/100**

#### Strengths

- **Complete value semantics assessment** in `docs/value-semantics-assessment.md`:
  - 23 pointer receivers converted to value receivers
  - Eliminated all nil panic surfaces
  - Explicit cost-benefit analysis with line counts
- **Value receivers for immutable types** (grep shows 50+ value receivers):
  - `Projection.Apply()`, `Projection.State()`, `Projection.Version()`
  - `State.WithVisible()`, `State.WithSeen()`
  - All event types: `SimulationCreated.WithTrace()`, etc.
- **Pointer receivers only where required**:
  - `*Engine` - mutates projection field
  - `*MemoryStore` - has sync.RWMutex
  - `*DedupMiddleware` - has sync.Mutex
  - `*SimRegistry` - has sync.RWMutex
- **Boundary defense complete** (Phase 3 fixes):
  - `LimitBody` middleware: 1MB limit
  - `RequireJSON` middleware: Content-Type validation
  - `isValidSeed`: rejects negative seeds
  - `DedupMiddleware`: idempotency with mutex protection
- **fluentfp usage** throughout:
  ```go
  slice.From(s.Developers).KeepIf(Developer.IsIdle)
  slice.From(f.History).ToFloat64(FeverSnapshot.GetPercentUsed)
  slice.From(sim.CompletedTickets).KeepIf(completedAfterCutoff).Len()
  ```
- **option.Basic** for optional values instead of nil pointers:
  ```go
  CurrentSprintOption option.Basic[Sprint]
  ```

#### Compliance Issues

**Resolved** (2026-01-22):
- ~~`model/simulation.go:58-72` - Pointer receivers on value type~~ → **FIXED**: Methods removed. Simulation now only has value receivers for query methods. Mutation happens via Engine.

**Minor** (-2 points):
- `api/handlers.go:329-363` - `runComparison` function does too much:
  - Creates simulation
  - Adds developers
  - Adds tickets
  - Runs sprints
  - Could be decomposed into smaller pure functions

#### Recommendations

| Recommendation | Effort | Impact |
|----------------|--------|--------|
| ~~**Move Simulation mutating methods to Engine**~~ | ~~Medium (1 day)~~ | ✅ DONE (2026-01-22) |
| Decompose `runComparison` into smaller functions | Medium (1 day) | +2 points |
| Add `//nolint:receiver` comments to document pointer receiver choices | Quick win (<1hr) | Clarity |

**Note:** ~~The Simulation fix is shared with FP Guide - one refactor, two compliance improvements.~~ Completed 2026-01-22.

---

## Priority Matrix

| Finding | Severity | Effort | Priority |
|---------|----------|--------|----------|
| ~~**Simulation mutating methods (FP + Go)**~~ | ~~Critical~~ | ~~Medium~~ | ✅ DONE (2026-01-22) |
| ~~Missing edge case tests (seed=0, boundaries)~~ | ~~Minor~~ | ~~Quick~~ | ✅ DONE (2026-01-23) |
| ~~Test names describe methods not behaviors~~ | ~~Minor~~ | ~~Quick~~ | ✅ DONE (2026-01-23) |
| `runComparison` function too large | Minor | Medium | 🔵 Backlog |
| Read model extraction from handlers | Minor | Medium | 🔵 Backlog |

**Key Insight:** ~~The Simulation structural issue is the only Critical finding.~~ **Resolved 2026-01-22:** Simulation methods moved to Engine, yielding +8 (FP) + +5 (Go) = +13 points.

---

## Summary by Guide Principle

### Khorikov: 4 Pillars Assessment

| Pillar | Score | Evidence |
|--------|-------|----------|
| Resistance to Refactoring | 8/8 | Tests verify behaviors via ranges, not implementation |
| Protection Against Regressions | 6/6 | Full edge case coverage: seed=0, boundaries, concurrent append, large replay |
| Fast Feedback | 5/5 | All tests < 1s, no flaky tests detected |
| Maintainability | 3/3 | Table-driven pattern; behavior-focused test names |
| Mock Usage | 3/3 | No internal mocks, only HTTP boundary |

### Event Sourcing: CQRS Checklist

| Criterion | Status | Evidence |
|-----------|--------|----------|
| Event Immutability | ✅ | Value types, documented in types.go |
| Command/Query Separation | ✅ | Engine emits, Projection.State() reads |
| Idempotent Handlers | ✅ | EventID dedup in Projection.Apply() |
| Optimistic Concurrency | ✅ | Version check in Store.Append() |
| Projection Correctness | ✅ | State derivable from events, tested |

### FP: ACD Classification

| Category | Examples | Compliance |
|----------|----------|------------|
| Actions | handlers.go, Engine.emit() | ✅ Clear I/O boundary |
| Calculations | variance.Calculate(), LinksFor() | ✅ Pure, documented |
| Data | Event types, Lesson, State, Simulation | ✅ All Data types are inert |

### Go: Value Semantics Audit

| Type | Receiver | Justified |
|------|----------|-----------|
| Projection | Value | ✅ Immutable, returns new |
| Events | Value | ✅ Immutable facts |
| Engine | Pointer | ✅ Mutates proj field |
| MemoryStore | Pointer | ✅ Has sync.RWMutex |
| Simulation | Value | ✅ Query methods only, mutation via Engine |

---

## Appendix: Files Reviewed

### Test Files (33)
```
internal/api/body_limit_test.go
internal/api/content_type_test.go
internal/api/decode_test.go
internal/api/dedup_test.go
internal/api/hypermedia_test.go
internal/api/registry_test.go
internal/api/resources_test.go
internal/api/seed_validation_test.go
internal/api/tutorial_walkthrough_test.go
internal/engine/engine_test.go
internal/engine/events_test.go
internal/engine/fluentfp_bench_test.go
internal/engine/policies_test.go
internal/engine/variance_test.go
internal/events/projection_test.go
internal/events/store_test.go
internal/events/types_test.go
internal/events/upcasting_test.go
internal/export/export_test.go
internal/export/writers_test.go
internal/lessons/lessons_test.go
internal/metrics/dora_test.go
internal/metrics/fever_test.go
internal/metrics/tracker_test.go
internal/model/developer_test.go
internal/model/incident_test.go
internal/model/sprint_test.go
internal/model/ticket_test.go
internal/persistence/persistence_test.go
internal/persistence/schema_test.go
internal/registry/dedup_test.go
internal/registry/registry_test.go
internal/tui/app_test.go
```

### Key Source Files
```
internal/api/handlers.go (569 lines)
internal/engine/engine.go (463 lines)
internal/events/projection.go (310 lines)
internal/events/types.go (800+ lines)
internal/model/simulation.go (125 lines)
internal/engine/variance.go (pure calculations)
internal/lessons/lessons.go (ACD documented)
docs/value-semantics-assessment.md (design rationale)
```

---

*Generated: 2026-01-21*
