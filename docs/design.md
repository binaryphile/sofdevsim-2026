# Design Document

## Overview

### What This Simulation Does

The Software Development Simulation models an 8-phase ticket workflow to test competing theories about optimal ticket sizing:

- **DORA Research** suggests that batch size matters: tickets taking longer than one week correlate with worse delivery outcomes
- **TameFlow** argues that cognitive load (understanding level) is the real discriminant: uncertain work causes variance regardless of size

This simulation lets you run controlled experiments to see which approach produces better DORA metrics.

### The Hypothesis

| Policy | Rule | Theory |
|--------|------|--------|
| DORA-Strict | Decompose tickets > 5 days | Time-based ceiling reduces batch size |
| TameFlow-Cognitive | Decompose tickets with Low understanding | Reducing uncertainty improves predictability |
| Hybrid | Both conditions | Belt and suspenders |
| None | No decomposition | Baseline for comparison |

### Why This Matters

Sizing policy affects:
- **Lead Time** - How long from start to deploy?
- **Quality** - How many incidents per deploy?
- **Predictability** - Can we trust our estimates?

---

## Domain Model

```mermaid
classDiagram
    class Simulation {
        +string ID
        +int CurrentTick
        +int SprintNumber
        +option.Basic~Sprint~ CurrentSprintOption
        +SizingPolicy SizingPolicy
        +int64 Seed
        +int SprintLength
        +float64 BufferPct
        +Developer[] Developers
        +Ticket[] Backlog
        +Ticket[] ActiveTickets
        +Ticket[] CompletedTickets
        +Incident[] OpenIncidents
        +Incident[] ResolvedIncidents
        +NewSimulation(policy, seed)$ *Simulation
        +FindActiveTicketIndex(id) int
        +FindBacklogTicketIndex(id) int
        +FindDeveloperIndex(id) int
        +IdleDevelopers() Developer[]
        +TotalOpenIncidents() int
        +TotalIncidents() int
        +TotalDeploys() int
    }

    class Ticket {
        +string ID
        +string Title
        +string ParentID
        +WorkflowPhase Phase
        +UnderstandingLevel UnderstandingLevel
        +float64 EstimatedDays
        +float64 ActualDays
        +map PhaseEffortSpent
        +int StartedTick
        +int CompletedTick
        +CalculatePhaseEffort(phase) float64
    }

    class Developer {
        +string ID
        +string Name
        +float64 Velocity
        +string CurrentTicket
        +IsIdle() bool
    }

    class Sprint {
        +int Number
        +int StartDay
        +int DurationDays
        +float64 BufferDays
        +float64 BufferUsed
        +ProgressPct(tick) float64
        +BufferPctUsed() float64
    }

    class Incident {
        +string ID
        +string TicketID
        +string Severity
        +time CreatedAt
        +time ResolvedAt
        +IsOpen() bool
    }

    Simulation "1" *-- "*" Developer
    Simulation "1" *-- "*" Ticket
    Simulation "1" *-- "0..1" Sprint
    Simulation "1" *-- "*" Incident
    Ticket "*" -- "0..1" Ticket : parent
```

> **Note:** Simulation is a pure Data type with query methods only. Mutation happens via Engine, which emits events that update state through Projection. Index-based lookups return -1 if not found.

### Workflow Phases

```mermaid
stateDiagram-v2
    [*] --> Research
    Research --> Sizing
    Sizing --> Planning
    Planning --> Implement
    Implement --> Verify
    Verify --> CI_CD
    CI_CD --> Review
    Review --> Done
    Done --> [*]
```

### Enumerations

**Understanding Levels:** Low | Medium | High

**Sizing Policies:** None | DORA-Strict | TameFlow-Cognitive | Hybrid

---

## Key Algorithms

### Variance Model (Core Hypothesis)

The variance model is the heart of the simulation. It maps understanding level to outcome predictability:

| Understanding | Multiplier Range | Meaning |
|---------------|------------------|---------|
| High | 0.95 - 1.05x | Predictable, minimal surprise |
| Medium | 0.80 - 1.20x | Some unknowns, moderate variance |
| Low | 0.50 - 1.50x | High uncertainty, frequent surprise |

**Implementation:** Each tick, actual effort = estimated effort × random multiplier from the range above.

### Phase Effort Distribution

Total ticket effort is distributed across phases:

| Phase | % of Total Effort |
|-------|-------------------|
| Research | 5% |
| Sizing | 2% |
| Planning | 3% |
| Implement | 55% |
| Verify | 20% |
| CI/CD | 5% |
| Review | 10% |
| Done | 0% |

### Decomposition Algorithm

When a ticket is decomposed:

1. **Children count:** 2-4 (weighted 40%/40%/20%)
2. **Children sum:** 90-110% of parent estimate (decomposition reveals scope)
3. **Each child:** Varies ±30% from base estimate
4. **Understanding improves:** 60% chance each child has better understanding than parent

### Incident Generation

Incidents are generated when tickets complete, based on understanding:

| Understanding | Base Fail Rate |
|---------------|----------------|
| High | 5% |
| Medium | 12% |
| Low | 25% |

**Large ticket multiplier:** Tickets > 5 days have 1.5x incident rate.

### DORA Metrics Calculation

| Metric | Formula | Better |
|--------|---------|--------|
| Lead Time | Average of (CompletedTick - StartedTick) | Lower |
| Deploy Frequency | Deploys in last 7 ticks ÷ 7 | Higher |
| MTTR | Average of (ResolvedAt - CreatedAt) for incidents | Lower |
| Change Fail Rate | Total incidents ÷ Total deploys | Lower |

---

## Architecture

```mermaid
flowchart TD
    subgraph cmd["cmd/sofdevsim/"]
        main["main.go<br/>(entry point)"]
    end

    subgraph tui["internal/tui/"]
        app["app.go - Bubbletea model, keybindings"]
        planning["planning.go - Backlog, developers"]
        execution["execution.go - Active work, fever chart"]
        metrics_view["metrics.go - DORA dashboard, sparklines"]
        comparison_view["comparison.go - A/B results"]
        styles["styles.go - Lipgloss styles"]
    end

    subgraph api["internal/api/"]
        server["server.go - HTTP router"]
        handlers["handlers.go - Request handlers"]
        middleware["middleware - LimitBody, RequireJSON, Dedup"]
        hypermedia["hypermedia.go - LinksFor()"]
        resources["resources.go - SimulationState"]
    end

    subgraph registry_pkg["internal/registry/"]
        registry["registry.go - SimRegistry + RWMutex"]
    end

    subgraph events_pkg["internal/events/"]
        evt_types["types.go - Event definitions"]
        evt_store["store.go - MemoryStore"]
        evt_projection["projection.go - State rebuild"]
        evt_upcasting["upcasting.go - Schema evolution"]
    end

    subgraph persistence["internal/persistence/"]
        schema["schema.go - SaveFile, SimulationState"]
        persist["persistence.go - Save/Load/ListSaves"]
        migrate["migrate.go - Version migrations"]
    end

    subgraph engine["internal/engine/"]
        engine_go["engine.go - Tick loop, transitions"]
        policies["policies.go - Decomposition"]
        variance["variance.go - Understanding→multiplier"]
        events["events.go - Bugs, incidents"]
        generator["generator.go - Ticket generation"]
    end

    subgraph metrics["internal/metrics/"]
        dora["dora.go - DORA calculations"]
        fever["fever.go - Buffer tracking"]
        comparison_logic["comparison.go - A/B logic"]
        tracker["tracker.go - History"]
    end

    subgraph model["internal/model/"]
        simulation["simulation.go"]
        ticket["ticket.go"]
        developer["developer.go"]
        sprint["sprint.go"]
        incident["incident.go"]
        enums["enums.go"]
    end

    main --> app
    main --> server
    app --> engine_go
    app --> dora
    app --> evt_store
    server --> middleware
    middleware --> handlers
    handlers --> registry
    registry --> engine_go
    registry --> dora
    registry --> evt_store
    engine_go --> simulation
    engine_go --> evt_projection
    dora --> simulation
```

### Package Dependencies

```mermaid
graph LR
    tui --> engine
    tui --> metrics
    tui --> persistence
    tui --> events
    tui --> registry
    api --> engine
    api --> metrics
    api --> events
    api --> registry
    engine --> model
    engine --> events
    metrics --> model
    persistence --> model
    persistence --> metrics
    registry --> events
    events --> model
```

**Dependency Rule:** Packages only depend downward. Model has no dependencies. Events is a central hub connecting TUI, API, Engine, and Registry.

### TUI Header Bar

```
[Planning] [Execution] [Metrics] [Comparison]  Policy: DORA-Strict | RUNNING | Day 42 | Backlog: 5 | Done: 12 | Seed 1234567890
```

| Element | Description |
|---------|-------------|
| View tabs | Current view highlighted |
| Policy | Active sizing policy |
| Status | RUNNING or PAUSED |
| Day | Current simulation tick |
| Backlog | Count of tickets awaiting assignment |
| Done | Count of completed tickets |
| Seed | RNG seed for reproducibility |

### Lessons Panel

Contextual teaching that adapts to current view and simulation state. Press 'h' to toggle.

**Architecture:**
```
┌────────────────────────────────┬──────────────────┐
│                                │ 💡 Lesson Title  │
│   View Content (2/3 width)     │                  │
│                                │ Content...       │
│                                │                  │
│                                │ • Tip 1          │
│                                │ • Tip 2          │
│                                │                  │
│                                │ Progress: 3/8    │
└────────────────────────────────┴──────────────────┘
```

**State (value semantics):**

| Field | Type | Purpose |
|-------|------|---------|
| Visible | bool | Toggle with 'h' key |
| SeenMap | map[LessonID]bool | Progress tracking (which lessons viewed) |
| Current | LessonID | Currently displayed lesson |

**Lesson Selection (pure function):**

`lessons.Select(view, state, hasActiveSprint, hasComparisonResult) → Lesson`

| View | Condition | Lesson |
|------|-----------|--------|
| (any) | First enable | Orientation (simulation intro) |
| Planning | — | Understanding levels (±5%, ±20%, ±50%) |
| Execution | Sprint active | Fever chart (buffer consumption) |
| Execution | Sprint ended | Phase progress (ticket phases) |
| Metrics | — | DORA metrics (4 metrics + direction) |
| Comparison | Has results | Policy comparison (DORA vs TameFlow) |
| Comparison | No results | Comparison intro (how to run) |

**8 Teaching Concepts:**

1. **Orientation** — Simulation intro, understanding→variance insight
2. **Understanding** — Understanding levels and their variance bounds
3. **Fever Chart** — Buffer consumption and traffic-light zones
4. **Phase Progress** — 8-phase ticket workflow
5. **DORA Metrics** — Four DevOps Research metrics
6. **Policy Comparison** — DORA-Strict vs TameFlow-Cognitive
7. **Variance Expected** — Per-ticket variance prediction
8. **Variance Analysis** — Post-sprint actual vs estimated

**API Endpoint:**

`GET /simulations/{id}/lessons` returns current lesson for external UI consumers (UC10 compatible).

```json
{
  "currentLesson": {
    "id": "orientation",
    "title": "Welcome to the Simulation",
    "content": "...",
    "tips": ["Tab switches views", "Space pauses/resumes"]
  },
  "progress": "0/8 concepts",
  "_links": {
    "self": "/simulations/sim-1/lessons",
    "simulation": "/simulations/sim-1"
  }
}
```

**Package Structure:**

- `internal/lessons/` — Shared types and Select() logic (avoids import cycle)
- `internal/tui/lessons.go` — Re-exports + lessonsPanel() rendering
- `internal/api/handlers.go` — HandleGetLessons endpoint

---

## Data Flow

### Tick Loop

```mermaid
flowchart TD
    A[Emit Ticked event] --> B[For each Developer with ticket]
    B --> C[Calculate effort with variance]
    C --> D[Emit WorkProgressed event]
    D --> E{Phase complete?}
    E -->|No| B
    E -->|Yes| F{Last phase?}
    F -->|No| G[Emit TicketPhaseChanged]
    G --> B
    F -->|Yes| H[Emit TicketCompleted]
    H --> I[Generate random events<br/>bugs, scope creep]
    I --> J[Check incident generation]
    J --> K[Emit BufferConsumed]
    K --> L[Track WIP]
    L --> M{Sprint ended?}
    M -->|Yes| N[Emit SprintEnded]
    M -->|No| O[Done]
```

> **Note:** All state changes happen through events. The Projection applies each event to rebuild simulation state.

### Phase Transition Logic

```mermaid
flowchart LR
    A[phaseEffort = EstimatedDays × distribution × variance] --> B{Spent >= Effort?}
    B -->|Yes| C[phase++]
    B -->|No| D[Continue work]
    C --> E{phase == Done?}
    E -->|Yes| F[Complete ticket]
    E -->|No| G[Start next phase]
```

### Comparison Mode

1. Generate backlog with seed N
2. Clone simulation state
3. Run Simulation A with DORA-Strict for 3 sprints
4. Run Simulation B with TameFlow-Cognitive for 3 sprints (same seed)
5. Compare final DORA metrics
6. Declare winner based on metric wins (4 metrics, majority wins)

**Auto-decomposition:** Before each sprint's ticket assignment (in `autoAssignForComparison`), the comparison auto-decomposes all backlog tickets that match the policy criteria. This happens per-sprint so children created by decomposition can be decomposed in subsequent sprints if they also match. This ensures policies produce different outcomes:
- DORA-Strict: Decomposes tickets > 5 days
- TameFlow-Cognitive: Decomposes tickets with Low understanding

---

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Tick = 1 day | Simplifies mental model; matches sprint planning |
| 8 phases | Based on Unified Workflow Rubric from industry research |
| Variance by understanding | Core hypothesis: uncertainty causes unpredictability |
| Seed-based RNG | Enables reproducible experiments |
| Gob-based persistence | Versioned binary saves for research workflows (see CLAUDE.md) |
| Bubbletea TUI | Elm architecture, well-maintained, ntcharts compatible |

---

## Data Export

### Purpose

Enable external validation of simulation hypotheses and teaching of TOC/DORA principles. The export provides raw data for:

| Goal | How Export Supports It |
|------|------------------------|
| **Teaching TOC** | sprints.csv: buffer_pct, fever_status, max_wip, avg_wip |
| **DORA integration** | metrics.csv: all 4 metrics; incidents.csv: MTTR detail |
| **Unified Ticket Workflow Rubric validation** | tickets.csv: 8 phase timing columns enable testing effort distribution |
| **Sizing hypothesis** | comparison.csv + tickets.csv: variance by understanding, policy comparison |

### Output Structure

```
sofdevsim-export-20260103-143052/
├── metadata.csv      # Seed, policy, export timestamp, phase distribution
├── tickets.csv       # Per-ticket data with theoretical validation + phase timing
├── sprints.csv       # Per-sprint buffer/flow/WIP data (TOC concepts)
├── incidents.csv     # Per-incident MTTR detail
├── metrics.csv       # DORA metrics summary
└── comparison.csv    # Policy A vs B results (if comparison run)
```

### CSV Schemas

```csv
# metadata.csv - Reproducibility and context
seed,policy,sprints_run,export_timestamp,simulation_version,phase_effort_distribution

# tickets.csv - Core hypothesis validation + 8-phase effort distribution
ticket_id,title,understanding,estimated_days,actual_days,variance_ratio,expected_var_min,expected_var_max,within_expected,policy,sprint_number,started_tick,completed_tick,lead_time_days,phase_research_days,phase_sizing_days,phase_planning_days,phase_implement_days,phase_verify_days,phase_cicd_days,phase_review_days,phase_done_days

# sprints.csv - TOC concepts (buffer, flow, WIP)
sprint_number,duration_days,buffer_days,buffer_used,buffer_pct,fever_status,tickets_started,tickets_completed,incidents_generated,max_wip,avg_wip

# incidents.csv - MTTR detail
incident_id,ticket_id,severity,created_tick,resolved_tick,mttr_days,sprint_number

# metrics.csv - DORA integration
policy,lead_time_avg,lead_time_stddev,deploy_frequency,mttr_avg,change_fail_rate,total_tickets,total_incidents

# comparison.csv - Sizing hypothesis test
seed,sprints_run,metric,dora_strict_value,tameflow_value,winner,difference,difference_pct
```

### Theoretical Bounds

For hypothesis validation, tickets.csv includes expected variance bounds:

| Understanding | expected_var_min | expected_var_max |
|---------------|------------------|------------------|
| High | 0.95 | 1.05 |
| Medium | 0.80 | 1.20 |
| Low | 0.50 | 1.50 |

The `within_expected` column is `true` if `expected_var_min <= variance_ratio <= expected_var_max`.

### Phase Effort Distribution

Stored in metadata.csv as JSON for Unified Ticket Workflow Rubric validation:

```json
{"research":0.10,"sizing":0.05,"planning":0.10,"implement":0.40,"verify":0.15,"cicd":0.05,"review":0.10,"done":0.05}
```

Compare actual `phase_*_days` columns against `estimated_days × distribution` to validate the 8-phase model.

### Export Algorithm

```mermaid
flowchart TD
    A[User presses 'e'] --> B{Completed tickets?}
    B -->|No| C[Show 'Nothing to export']
    B -->|Yes| D[Create directory<br/>sofdevsim-export-YYYYMMDD-HHMMSS]
    D --> E[Write metadata.csv]
    E --> F[Write tickets.csv<br/>with theoretical bounds]
    F --> G[Write sprints.csv<br/>with WIP metrics]
    G --> H[Write incidents.csv]
    H --> I[Write metrics.csv]
    I --> J{Comparison run?}
    J -->|Yes| K[Write comparison.csv]
    J -->|No| L[Skip comparison.csv]
    K --> M[Show confirmation<br/>path + row counts]
    L --> M
```

---

## Persistence

Enables pause/resume for long-running experiments. Full state is captured including metrics history.

### Architecture

```mermaid
flowchart LR
    A[TUI: Ctrl+s] --> B[persistence.Save]
    B --> C[gob.Encode]
    C --> D[saves/*.sds]

    E[TUI: Ctrl+o] --> F[persistence.Load]
    F --> G[gob.Decode]
    G --> H{Version check}
    H -->|Current| I[Restore state]
    H -->|Old| J[Migrate chain]
    J --> I
    H -->|Future| K[Error: upgrade required]
```

### Design Decisions

| Decision | Rationale |
|----------|-----------|
| Gob format | Go-native, efficient binary, handles all model types |
| Schema versioning | Forward compatibility for research data |
| Auto-migration | Seamless upgrades without user intervention |
| Most-recent load | Simple UX for common case (Ctrl+o loads latest) |

For API details and keybindings, see CLAUDE.md § Persistence.

---

## HTTP API

Enables programmatic simulation testing without TUI interaction. Supports UC9 (Test Simulation Behavior Programmatically).

### Design: HATEOAS

The API follows REST with hypermedia (HATEOAS). Each response includes `_links` that tell the client what actions are available based on current state.

**Why HATEOAS for testing:**

| Benefit | How It Helps Testing |
|---------|---------------------|
| Self-verifying | Link presence/absence proves state correctness |
| Discoverable | Agent follows links, no hardcoded URLs |
| State-driven | Links change when state changes (sprint ends → tick link disappears) |

### Endpoints

| Method | Path | Purpose | Links Returned |
|--------|------|---------|----------------|
| GET | `/` | Entry point | `simulations`, `comparisons` |
| GET | `/simulations` | List active simulations | `self`, per-simulation links |
| POST | `/simulations` | Create simulation | `self`, `start-sprint` |
| GET | `/simulations/{id}` | Get simulation state | `self`, `tick` or `start-sprint` |
| POST | `/simulations/{id}/sprints` | Start sprint | `self`, `tick` |
| POST | `/simulations/{id}/tick` | Advance one tick | `self`, `tick` or `start-sprint` |
| POST | `/simulations/{id}/assignments` | Assign ticket to developer | `self`, `tick` |
| POST | `/simulations/{id}/decompose` | Decompose ticket into children | `self` |
| GET | `/simulations/{id}/lessons` | Get current lesson for teaching | `self`, `simulation` |
| POST | `/comparisons` | Run policy comparison | `self` |

### Example Response (HAL+JSON style)

```json
{
  "id": "sim-42",
  "currentTick": 5,
  "sprintActive": true,
  "backlogCount": 8,
  "sprint": {
    "number": 1,
    "startDay": 1,
    "durationDays": 10,
    "bufferPctUsed": 0.23
  },
  "_links": {
    "self": "/simulations/sim-42",
    "tick": "/simulations/sim-42/tick",
    "assign": "/simulations/sim-42/assignments"
  }
}
```

**Link transitions:**
- Sprint ends → `tick` disappears, `start-sprint` appears
- Backlog has tickets → `assign` appears (regardless of sprint state, for sprint planning)
- Backlog empty → `assign` disappears (nothing to assign)

### Assignment Request

```json
POST /simulations/{id}/assignments

// Explicit assignment
{ "ticketId": "TKT-001", "developerId": "dev-1" }

// Auto-assign to first idle developer
{ "ticketId": "TKT-001" }
```

**Success:** Returns updated simulation state (same format as GET).

**Errors:**
- 400: Ticket not in backlog
- 400: Developer not found
- 400: Developer is busy
- 400: No idle developers (auto-assign only)

### Comparison Request

```json
POST /comparisons

{
  "seed": 12345,
  "sprints": 3
}
```

**Fields:**
- `seed`: Random seed for reproducibility (optional, defaults to current time)
- `sprints`: Sprints per policy (optional, defaults to 3)

**Success:** Returns comparison result with full DORA metrics for each policy.

**Note:** Blocking, synchronous operation. Runs both policy simulations to completion before returning.

**Errors:**
- 400: Invalid sprints count
- 500: Simulation error

### Comparison Response

```json
{
  "seed": 12345,
  "sprints": 3,
  "policyA": {
    "name": "dora-strict",
    "ticketsComplete": 15,
    "incidentCount": 2,
    "metrics": {
      "leadTimeAvgDays": 4.2,
      "deployFrequency": 1.8,
      "mttrAvgDays": 0.5,
      "changeFailRatePct": 13.3
    }
  },
  "policyB": {
    "name": "tameflow-cognitive",
    "ticketsComplete": 12,
    "incidentCount": 1,
    "metrics": {
      "leadTimeAvgDays": 5.1,
      "deployFrequency": 1.4,
      "mttrAvgDays": 0.3,
      "changeFailRatePct": 8.3
    }
  },
  "winners": {
    "leadTime": "dora-strict",
    "deployFrequency": "dora-strict",
    "mttr": "tameflow-cognitive",
    "changeFailRate": "tameflow-cognitive",
    "overall": "tie"
  },
  "winsA": 2,
  "winsB": 2,
  "_links": {
    "self": "/comparisons"
  }
}
```

**Note:** Response mirrors `metrics.ComparisonResult` struct. See `internal/metrics/comparison.go:8-26`.

### Architecture: Registry with Mutex Protection

```
┌─────────────────────────────────────────────────┐
│                   main.go                       │
├─────────────────────────────────────────────────┤
│  ┌─────────────┐          ┌─────────────────┐   │
│  │   TUI       │          │    HTTP API     │   │
│  │ (Bubbletea) │          │   (net/http)    │   │
│  └──────┬──────┘          └────────┬────────┘   │
│         │                          │            │
│         ▼                          ▼            │
│  ┌─────────────┐          ┌─────────────────┐   │
│  │ TUI's own   │          │  SimRegistry    │   │
│  │ Simulation  │          │ RWMutex + map   │   │
│  └─────────────┘          └─────────────────┘   │
│                                    │            │
│                           ┌────────┴────────┐   │
│                           ▼                 ▼   │
│                    ┌───────────┐     ┌───────────┐
│                    │ SimInst 1 │     │ SimInst 2 │
│                    │ (seed 42) │     │ (seed 99) │
│                    │ ⚠ SHARED  │     │ ⚠ SHARED  │
│                    └───────────┘     └───────────┘
└─────────────────────────────────────────────────┘
```

**Concurrency model:**

1. SimRegistry uses `sync.RWMutex` to protect the shared instances map
2. **LIMITATION:** Individual `SimInstance` structs ARE shared across concurrent requests
3. Read locks for lookups (`GetInstance`), write locks for registration
4. Engine mutations are NOT serialized - concurrent tick/assign calls can race

**ES Design Sections** (per `urma-obsidian/guides/cqrs-event-sourcing-guide.md`):
- Concurrency Model (§11) - immutable engine pattern
- Idempotency (§15) - command ID tracking
- Snapshots (§7) - periodic state capture
- Projections (§8) - pre-computed read models
- Implementation Sequence - dependency order

### Concurrency Model (ES Guide §11)

> Reference: `urma-obsidian/guides/cqrs-event-sourcing-guide.md` §11

**Problem:** Engine.proj field races between emit() and Sim(). The current
pointer receiver pattern `e.proj = e.proj.Apply(evt)` mutates shared state.
Concurrent Tick() calls race on the proj field. (See `internal/api/stress_test.go`
for reproduction.)

**Rejected approaches:**

| Approach | Why Rejected |
|----------|--------------|
| RWMutex on SimInstance | Works but violates FP Guide §7 - still mutable state, just protected |
| Atomic pointer | Lock-free but same FP violation - Engine field still mutable |

**Design:** Immutable Engine pattern. Methods use value receivers and return
new Engine. Callers must capture the return value.

```go
// Value receiver - returns new Engine instead of mutating
func (e Engine) Tick() (Engine, []model.Event) {
    // ... emit events to store ...
    newProj := e.store.Replay().Apply(newEvents...)
    return Engine{store: e.store, proj: newProj}, modelEvents
}

// Caller must capture return
eng, events = eng.Tick()
```

**Why this works:**
- No shared mutable state = no races (FP Guide §7)
- Value semantics throughout (Go Dev Guide §3)
- Event store remains source of truth (ES Guide)

**Trade-offs:**
- More allocations (new Engine per operation)
- Verbose call sites (`eng = eng.Method()` vs `eng.Method()`)
- Acceptable for reference implementation prioritizing correctness

**Verified by:** `go test -race ./internal/api/ -run Concurrent` (must pass)

**Regression gate:** `internal/api/stress_test.go` - currently fails, becomes passing test.

**Cleanup:** Remove `⚠ SHARED` annotation from architecture diagram (line 750) after implementation.

**Size: M** (~4-8 hours due to caller updates)

---

### Idempotency (ES Guide §15)

**Current state:** No duplicate detection. Same command executed twice produces
duplicate events.

**Design:** Commands carry a unique ID. The event store tracks processed command IDs
and rejects duplicates:

```go
type CommandID string

func (s *Store) Append(ctx context.Context, streamID string,
    expectedVersion int64, commandID CommandID, events []Event) error {
    if s.commandProcessed(commandID) {
        return nil  // Idempotent: already processed
    }
    // ... append events ...
    s.markCommandProcessed(commandID)
}
```

**Command ID generation:** API handlers generate UUIDs for each command. If the
client provides `X-Idempotency-Key` header, that value is used instead (enables
client-controlled retries).

**Storage:** Processed command IDs are stored in a map within the event store:

```go
type Store struct {
    events     map[string][]Event      // streamID → events
    versions   map[string]int64        // streamID → current version
    processed  map[CommandID]struct{}  // deduplication set
}
```

**Persistence path:** When adding persistent storage, command IDs are stored in
the same transaction as events (outbox pattern per ES Guide §10). The dedup
check becomes a database query: `SELECT 1 FROM events WHERE command_id = ?`.

**Verified by:** Test that submitting identical Tick twice produces one event

**Size: M** (~half day)

---

### Snapshots (ES Guide §7)

**Current state:** Every load replays all events from the beginning. Long-running
simulations get progressively slower.

**Design:** The event store captures periodic snapshots to bound replay cost:

```go
type Snapshot struct {
    StreamID  string
    Version   int64   // Event count at snapshot time
    State     []byte  // Serialized simulation state
    CreatedAt time.Time
}

func (s *Store) Load(ctx context.Context, streamID string) (*Simulation, error) {
    snapshot := s.loadLatestSnapshot(streamID)
    events := s.loadEventsFrom(streamID, snapshot.Version)
    return rebuildFrom(snapshot.State, events)
}
```

Snapshots are created every 100 events (configurable). Load time is
O(events since snapshot), not O(total events).

**Retention:** Keep only the latest snapshot per stream. Old snapshots are
deleted when a new one is created. Events before the snapshot are retained
(required for audit trail per ES Guide §23).

**Verified by:** Benchmark showing 1000-event simulation loads in <10ms

**Size: M** (~half day)

---

### Projections (ES Guide §8)

**Current state:** Every query rebuilds state from events. No pre-computed views.

**Design:** Pre-computed read models for dashboard queries:

| Projection | Updated On | Query |
|------------|------------|-------|
| SprintSummary | TickCompleted, TicketMoved | Tickets by status, WIP count |
| DeveloperLoad | TicketAssigned, TicketCompleted | Per-developer ticket counts |
| FlowMetrics | SprintCompleted | Throughput, cycle time, lead time |

```go
type SprintSummaryProjection struct {
    TicketsByStatus map[string]int
    WIPCount        int
    LastUpdated     time.Time
}

func (p *SprintSummaryProjection) Apply(event Event) {
    switch e := event.(type) {
    case TicketMoved:
        p.TicketsByStatus[e.FromStatus]--
        p.TicketsByStatus[e.ToStatus]++
    // ...
    }
}
```

Projections update synchronously on event append (acceptable for single-node).
Query time is O(1).

**Subscription:** Projections register with the event store and receive events
via callback on Append:

```go
type Store struct {
    // ...
    projections []Projection
}

func (s *Store) Append(...) error {
    // ... append events ...
    for _, p := range s.projections {
        for _, e := range events {
            p.Apply(e)
        }
    }
}
```

**Storage:** Projections live in memory alongside the event store.

**Persistence path:** When adding persistent storage, projection state is
serialized to disk after each update. On startup, load from disk instead of
replaying events. This trades disk writes for faster restarts.

**Rebuild:** On startup (or if projection state is corrupted/missing), replay
events from the beginning. With simulation snapshots available, replay starts
from the snapshot version instead. Natural idempotency (see below) means no
deduplication tracking is needed during rebuild.

**Idempotency:** Projection handlers are naturally idempotent—they compute
derived state from events. Re-applying the same event produces the same result.
The `Apply` method uses assignment, not accumulation, so duplicate delivery is
safe.

**Verified by:** Dashboard queries return in <1ms regardless of event count

**Size: L** (~1+ day)

---

### Implementation Sequence

| Phase | ES Guide | Size | Dependency |
|-------|----------|------|------------|
| Concurrency | §11 | S | None (do first) |
| Idempotency | §15 | M | Concurrency |
| Snapshots | §7 | M | None |
| Projections | §8 | L | None |

Concurrency and Idempotency are correctness requirements.
Snapshots and Projections are performance optimizations.

### SimRegistry

```go
// SimRegistry manages independent simulation instances
// Pointer receiver required: contains sync.RWMutex (must not be copied)
type SimRegistry struct {
    mu        sync.RWMutex
    instances map[string]SimInstance
    store     events.Store
}

// SimInstance holds simulation state
type SimInstance struct {
    Sim     *model.Simulation  // Pointer for registry storage
    Engine  *engine.Engine
    Tracker metrics.Tracker
}
```

> **Note:** `Engine.Sim()` returns a value copy of current state for safe read access.

### Startup Sequence

1. Create SimRegistry (empty, API creates simulations on demand)
2. Start HTTP server on configurable port in goroutine
3. Run TUI on main goroutine (Bubbletea requirement)
4. Process exit terminates both (no graceful shutdown yet)

### Hypermedia Logic (Pure, Unit Testable)

```go
// LinksFor is pure: state → links (unit testable)
func LinksFor(state SimulationState) map[string]string {
    links := map[string]string{
        "self": "/simulations/" + state.ID,
    }

    // Assign link available whenever backlog has tickets (UC11: sprint planning)
    // Not gated on sprint state - allows planning before sprint starts
    if state.BacklogCount > 0 {
        links["assign"] = "/simulations/" + state.ID + "/assignments"
    }

    if state.SprintActive {
        links["tick"] = "/simulations/" + state.ID + "/tick"
    } else {
        links["start-sprint"] = "/simulations/" + state.ID + "/sprints"
    }
    return links
}
```

This pure function enables unit testing of link logic without HTTP. Key insight: `assign` is not nested under `SprintActive` because UC11 requires sprint planning before the sprint starts.

### Response Building (Query Phase)

Handlers separate command execution from query response using a dedicated helper:

```go
// respondWithSimulation writes the HAL response for a simulation instance.
// Query: builds read model from instance state.
// Per CQRS Guide §Query Side: queries should be clearly separated from commands.
func respondWithSimulation(w http.ResponseWriter, inst registry.SimInstance, status int) {
    state := ToState(inst.Engine.Sim(), inst.Tracker)
    response := HALResponse{State: state, Links: LinksFor(state)}
    writeJSON(w, status, response)
}
```

**Pattern:** Commands mutate state via Engine, then delegate to this helper for the query phase. Per CQRS Guide §Query Side, queries should be clearly separated from commands. The helper builds a read model (ES Guide §8) then performs I/O - an acceptable boundary layer per FP Guide.

**Components:**

| Function | Classification | Purpose |
|----------|---------------|---------|
| `ToState()` | Calculation (pure) | Builds read model from simulation + tracker |
| `LinksFor()` | Calculation (pure) | Computes HATEOAS links from state |
| `writeJSON()` | Action | HTTP I/O |
| `respondWithSimulation()` | Action | Orchestrates calculations → I/O |

**Call sites:** HandleCreateSimulation (164), HandleGetSimulation (178), HandleStartSprint (207), HandleTick (244), HandleAssignTicket (294), HandleSetPolicy (508).

**Exception:** HandleDecompose (529) uses `DecomposeResponse` at line 564 (includes decomposition-specific fields).

### Test Strategy (Khorikov Quadrants)

| Component | Quadrant | Complexity | Collaborators | Strategy |
|-----------|----------|------------|---------------|----------|
| `LinksFor()` | Domain | Medium | Few (state only) | Unit test heavily |
| `ToState()` | Trivial | Low | Few | Don't test |
| `resources.go` | Trivial | Low | Few | Don't test |
| HTTP handlers | Controller | Low | Many | ONE integration test |
| `SimRegistry` | Controller | Low | Many | Covered by integration |

**Domain (unit test):** `LinksFor()` - test all state→link rules (sprint active = tick link, sprint ended = start-sprint link)

**Controller (ONE integration test):** Full lifecycle test - create simulation, start sprint, tick until sprint ends, verify links change. HATEOAS link presence = correct behavior.

---

## Event Sourcing Architecture

### Overview

The simulation uses event sourcing to enable shared access between TUI and API. Instead of mutating state directly, the engine emits events. State is derived by replaying events through a projection.

```
Commands (Tick, Assign, Decompose)
    │
    ▼
┌─────────────────────────────────────────┐
│           Event Store                    │
│  (append-only log per simulation)        │
│                                          │
│  sim-1: [Created, SprintStarted, Tick,  │
│          Assigned, Tick, Completed...]   │
└─────────────────────────────────────────┘
    │
    ├──→ TUI (subscribes, projects state)
    └──→ API (subscribes, returns state)
```

### Why Event Sourcing?

| Benefit | How It Helps |
|---------|--------------|
| **Shared state** | TUI and API see same simulation via same event stream |
| **Audit trail** | Every action recorded; replay for debugging |
| **Decoupling** | No `p.Send()` coupling between engine and TUI |
| **Replay** | Recreate any historical state by replaying events |

Per Martin Fowler: "CQRS is suited to complex domains" - simulation qualifies.

### Event Types

```go
// Event is the base interface for all simulation events
type Event interface {
    SimulationID() string
    Timestamp() time.Time
    EventType() string
}

// Simulation lifecycle
type SimulationCreated struct {
    ID     string
    Seed   int64
    Policy model.SizingPolicy
}

// Sprint lifecycle
type SprintStarted struct {
    SprintNumber int
    StartDay     int
    DurationDays int
    BufferDays   float64
}

type SprintEnded struct {
    SprintNumber     int
    EndDay           int
    TicketsCompleted int
}

// Tick events
type Ticked struct {
    Day int
}

// Ticket events
type TicketAssigned struct {
    TicketID    string
    DeveloperID string
}

type TicketPhaseChanged struct {
    TicketID  string
    FromPhase model.WorkflowPhase
    ToPhase   model.WorkflowPhase
}

type TicketCompleted struct {
    TicketID   string
    ActualDays float64
}

type TicketDecomposed struct {
    ParentID  string
    ChildIDs  []string
}

// Incident events
type IncidentCreated struct {
    IncidentID string
    TicketID   string
    Severity   string
}

type IncidentResolved struct {
    IncidentID string
}
```

### EventStore Interface

```go
// EventStore provides append-only storage and subscription
type EventStore interface {
    // Append adds events to a simulation's stream
    Append(simID string, events ...Event) error

    // Replay returns all events for a simulation in order
    Replay(simID string) ([]Event, error)

    // Subscribe returns a channel that receives new events
    Subscribe(simID string) <-chan Event

    // Unsubscribe stops receiving events
    Unsubscribe(simID string, ch <-chan Event)
}
```

### Projection

The projection rebuilds simulation state from events:

```go
// Projection applies events to build current state
type Projection struct {
    sim     *model.Simulation
    tracker *metrics.Tracker
}

// Apply processes a single event, updating internal state
func (p *Projection) Apply(event Event) {
    switch e := event.(type) {
    case *SimulationCreated:
        p.sim = model.NewSimulation(e.Seed, e.Policy)
    case *SprintStarted:
        p.sim.CurrentSprint = &model.Sprint{
            Number:       e.SprintNumber,
            StartDay:     e.StartDay,
            DurationDays: e.DurationDays,
            BufferDays:   e.BufferDays,
        }
    case *Ticked:
        p.sim.CurrentTick = e.Day
    case *TicketAssigned:
        // Update ticket and developer state
    case *TicketCompleted:
        // Move ticket, update metrics
    // ... other event types
    }
}

// State returns the current projected state
func (p *Projection) State() *model.Simulation {
    return p.sim
}
```

### Data Flow with Event Sourcing

```mermaid
flowchart TD
    subgraph Commands
        A[TUI: Press space] --> B[Tick Command]
        C[API: POST /tick] --> B
        D[TUI: Press 'a'] --> E[Assign Command]
    end

    subgraph Engine
        B --> F[Engine.Tick]
        E --> G[Engine.Assign]
        F --> H[Generate Events]
        G --> H
    end

    subgraph EventStore
        H --> I[Append Events]
        I --> J[(Event Log)]
    end

    subgraph Subscribers
        J --> K[TUI Subscription]
        J --> L[API Subscription]
        K --> M[TUI Projection]
        L --> N[API Projection]
        M --> O[Update Display]
        N --> P[Return State]
    end
```

### TUI Integration

The TUI subscribes to events and uses `p.Send()` to inject them into Bubbletea's update loop:

```go
func (a *App) subscribeToEvents() {
    ch := a.store.Subscribe(a.simID)
    go func() {
        for event := range ch {
            a.program.Send(eventMsg{event})
        }
    }()
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch m := msg.(type) {
    case eventMsg:
        a.projection.Apply(m.event)
        // React to significant events
        if m.event.EventType() == "SprintStarted" {
            a.currentView = ExecutionView
        }
        return a, nil
    // ... other message handling
    }
}
```

### API Integration

The API rebuilds state from events on each request:

```go
func (h *Handler) GetSimulation(w http.ResponseWriter, r *http.Request) {
    events, _ := h.store.Replay(simID)
    projection := NewProjection()
    for _, e := range events {
        projection.Apply(e)
    }
    state := projection.State()
    // Return state as JSON with HATEOAS links
}
```

### Package Structure

```
internal/
├── events/
│   ├── types.go      # Event type definitions
│   ├── store.go      # EventStore interface + in-memory impl
│   └── projection.go # State projection from events
├── engine/
│   └── engine.go     # Modified to emit events
├── tui/
│   └── app.go        # Subscribe and project
└── api/
    └── handlers.go   # Replay and project
```

### Migration Strategy

1. Define event types (no breaking changes)
2. Implement EventStore with in-memory storage
3. Implement Projection
4. Modify engine to emit events (alongside existing mutations)
5. Wire TUI to subscribe
6. Wire API to replay
7. Remove direct state mutations (events become source of truth)
