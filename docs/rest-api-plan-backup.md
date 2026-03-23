# Plan: REST API for Programmatic Simulation Testing

## Objective

Add an HTTP API to sofdevsim that enables Claude (and human developers) to programmatically test simulation behavior without manual TUI interaction.

---

## Use Case: UC9 - Programmatic Simulation Testing

**Primary Actor:** Automated Test Agent (Claude)

**Secondary Actor:** Human Developer (curl/Postman)

**Goal in Context:** Execute simulation scenarios and verify outcomes programmatically, enabling automated verification of simulation behavior.

**Scope:** Software Development Simulation (sofdevsim)

**Level:** User Goal (Blue)

**Stakeholder Interests:**
| Stakeholder | Interest |
|-------------|----------|
| Developer (Claude) | Verify simulation fixes without manual TUI interaction |
| Human Developer | Debug and explore simulation state via HTTP |
| Researcher | Run batch experiments via scripting |

### System-in-Use Story

> Claude, verifying a fix to sprint-end behavior, sends a POST to `/simulations` to create a new simulation with seed 42. Claude then POSTs to `/simulations/{id}/sprints` to start a sprint, followed by repeated POSTs to `/simulations/{id}/tick` until CurrentTick reaches the sprint's EndDay. After each tick, Claude GETs `/simulations/{id}` to inspect state. When the sprint ends, Claude verifies that `CurrentSprintOption` is cleared and the simulation is paused. Claude likes this API because each response includes links to available actions, so there's no need to hardcode URL patterns—just follow the hypermedia.

### Main Success Scenario

1. Actor creates a new simulation (specifying policy, seed, team)
2. System returns simulation resource with links to available actions
3. Actor starts a sprint via the provided link
4. System returns updated state with tick/assign/decompose links
5. Actor advances simulation via tick link
6. System returns events and updated state with context-appropriate links
7. Actor inspects metrics via provided link
8. System returns DORA metrics and fever chart data
9. Actor verifies expected outcomes

### Extensions

- 2a. *Invalid configuration:* System returns 400 with problem details
- 5a. *Sprint ends:* System clears sprint, response links change (no tick, only start-sprint)
- 5b. *Ticket completes:* Event included in response, metrics updated
- 7a. *No completed tickets:* Metrics show zero values

---

## API Style Analysis for Testing Purpose

### Option A: True REST (HATEOAS)

**What it means:** Server responses include hypermedia links that tell the client what actions are available. Client discovers API by following links, not by hardcoding URLs.

**Example response:**
```json
{
  "simulation": {
    "id": "sim-42",
    "currentTick": 5,
    "sprintActive": true
  },
  "_links": {
    "self": "/simulations/sim-42",
    "tick": "/simulations/sim-42/tick",
    "assign": "/simulations/sim-42/tickets/{ticketId}/assign",
    "metrics": "/simulations/sim-42/metrics"
  }
}
```

**Pros for testing:**
- Self-documenting: Claude can discover available actions from responses
- State-driven: Links change based on state (no tick link when paused)
- Correct behavior verified by link presence/absence
- Future-proof: API can evolve without breaking clients

**Cons for testing:**
- More complex to implement (hypermedia format: HAL, Siren, or custom)
- Responses are larger (include links)
- Claude must parse links instead of hardcoding (minor)

### Option B: Resource-Oriented HTTP (Pragmatic)

**What it means:** Clean HTTP semantics (proper verbs, status codes, resource URIs) but no hypermedia links. Client knows URL patterns from documentation.

**Example response:**
```json
{
  "simulation": {
    "id": "sim-42",
    "currentTick": 5,
    "sprintActive": true
  }
}
```

**Pros for testing:**
- Simpler to implement
- Smaller responses
- Well-understood patterns (OpenAPI spec)
- Claude can hardcode URL patterns easily

**Cons for testing:**
- No self-documentation in responses
- Client must know URL structure out-of-band
- State changes don't affect available URLs (must check manually)
- Not truly RESTful (per Fielding)

### Option C: Minimal RPC-over-HTTP

**What it means:** POST to command endpoints, GET to query endpoints. URLs are verbs, not nouns.

**Example:**
```
POST /start-sprint {"simulationId": "sim-42"}
POST /tick {"simulationId": "sim-42"}
GET /get-metrics?simulationId=sim-42
```

**Pros for testing:**
- Fastest to implement
- Dead simple to call from bash/curl
- Minimal ceremony

**Cons for testing:**
- Not RESTful at all (violates uniform interface)
- No resource identity (URLs are verbs)
- Harder to cache, layer, or evolve
- Doesn't leverage HTTP semantics

---

## Recommendation

**Option A (True REST with HATEOAS)** is recommended for these reasons:

1. **Self-verifying tests:** When sprint ends, the `tick` link disappears from response. Claude can verify correct behavior by checking link presence—no need to inspect internal state.

2. **Discoverable:** Claude starts at entry point, follows links. If we add new features, Claude can discover them without code changes.

3. **Educational value:** This simulation teaches software development practices. A truly RESTful API demonstrates proper architecture.

4. **Fielding's intent:** "Software design on the scale of decades." The extra upfront work pays off in evolvability.

**Implementation approach:** Use HAL+JSON format.

### Hypermedia Format Comparison

| Format | Links | Actions | Complexity | Go Support |
|--------|-------|---------|------------|------------|
| **HAL+JSON** | Yes | No (verbs implied) | Low | Easy to hand-roll |
| **Siren** | Yes | Yes (explicit) | Medium | No mainstream lib |
| **JSON:API** | Yes | No | High (spec is large) | Libraries exist |
| **Custom `_links`** | Yes | Optional | Low | Trivial |

**Choice: HAL+JSON** - Simple, well-documented, sufficient for our needs. Actions are implied by link relations (e.g., `tick` link means POST to tick).

---

## Architecture

Per user requirement: **Both TUI and API run simultaneously.**

### Value-Semantic Design (No Mutex)

The UC9 story reveals the key insight: API creates **independent simulation instances**. TUI has its own simulation; API manages a collection of separate ones. No sharing = no mutex.

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
│  │ Simulation  │          │ map[id]SimInst  │   │
│  └─────────────┘          └─────────────────┘   │
│                                    │            │
│                           ┌────────┴────────┐   │
│                           ▼                 ▼   │
│                    ┌───────────┐     ┌───────────┐
│                    │ SimInst 1 │     │ SimInst 2 │
│                    │ (seed 42) │     │ (seed 99) │
│                    └───────────┘     └───────────┘
└─────────────────────────────────────────────────┘
```

### Why No Mutex?

1. **Each API simulation is independent** - POST `/simulations` creates a new instance
2. **TUI doesn't share state with API** - They operate on different simulations
3. **HTTP is request-per-goroutine** - Within a handler, we have exclusive access
4. **No concurrent access to same simulation** = No mutex needed

### Domain Layer (Pure, Unit Testable)

```go
// Engine.Tick is pure: takes state, returns new state + events
func (e Engine) Tick(sim model.Simulation) (model.Simulation, []model.Event) {
    // No mutation, returns new state
}

// LinksFor is pure: state → links (unit testable!)
func LinksFor(state SimulationState) map[string]string {
    links := map[string]string{
        "self": "/simulations/" + state.ID,
    }

    // sprintIsActive checks if the sprint option has a value.
    sprintIsActive := func(s SimulationState) bool {
        _, ok := s.CurrentSprintOption.Get()
        return ok
    }

    if sprintIsActive(state) {
        links["tick"] = "/simulations/" + state.ID + "/tick"
    } else {
        links["start-sprint"] = "/simulations/" + state.ID + "/sprints"
    }
    return links
}
```

### Controller Layer (Thin, Integration Tested)

```go
// SimRegistry manages independent simulation instances
type SimRegistry struct {
    instances map[string]*SimInstance
}

// SimInstance owns its simulation - value semantics internally
type SimInstance struct {
    id      string
    sim     model.Simulation  // Value, not pointer
    engine  engine.Engine
    tracker metrics.Tracker
}

// HandleTick is a thin controller - wires domain pieces together
func (r *SimRegistry) HandleTick(w http.ResponseWriter, req *http.Request) {
    id := chi.URLParam(req, "id")
    inst := r.instances[id]

    // Domain call (pure)
    newSim, events := inst.engine.Tick(inst.sim)
    inst.sim = newSim

    // Domain call (pure)
    state := ToState(newSim)
    state.Links = LinksFor(state)

    json.NewEncoder(w).Encode(state)
}
```

### Startup Sequence

1. Start HTTP server on `:8080` in goroutine (API creates its own simulations)
2. Run TUI on main goroutine with its own simulation (Bubbletea requirement)
3. TUI and API are independent - no shared state

---

## Test Strategy

Following Khorikov quadrants from CLAUDE.md:

### Khorikov Quadrant Classification

| Component | Quadrant | Complexity | Collaborators | Strategy |
|-----------|----------|------------|---------------|----------|
| `LinksFor(state)` | Domain | Medium | Few (state only) | Unit test heavily |
| `Engine.Tick()` | Domain | High | Few (sim only) | Unit test heavily (existing) |
| `ToState()` | Trivial | Low | Few | Don't test |
| `resources.go` | Trivial | Low | Few | Don't test |
| HTTP handlers | Controller | Low | Many | ONE integration test |
| `SimRegistry` | Controller | Low | Many | Covered by integration |

### Domain/Algorithms (Unit Test Heavily)

| Component | Tests |
|-----------|-------|
| `hypermedia.LinksFor()` | State→links rules: sprint active = tick link, sprint ended = start-sprint link |
| `engine.Tick()` | Already tested - phase transitions, variance, events |

### Controllers (ONE Integration Test per Workflow)

| Workflow | Test |
|----------|------|
| Full lifecycle | Create → Start → Tick until sprint ends → Verify links change |

**Note:** "Sprint end clears links" is verified BY the lifecycle test (link presence = correct behavior). Not a separate test.

### Example Tests

**Domain test (unit):**
```go
func TestLinksFor_SprintActive_HasTickLink(t *testing.T) {
    state := SimulationState{
        ID:                  "sim-42",
        CurrentSprintOption: option.Of(Sprint{EndDay: 10}),
    }

    links := LinksFor(state)

    if links["tick"] == "" {
        t.Error("active sprint should have tick link")
    }
    if links["start-sprint"] != "" {
        t.Error("active sprint should NOT have start-sprint link")
    }
}

func TestLinksFor_NoSprint_HasStartSprintLink(t *testing.T) {
    state := SimulationState{
        ID:                  "sim-42",
        CurrentSprintOption: option.Option[Sprint]{}, // Empty = no sprint
    }

    links := LinksFor(state)

    if links["tick"] != "" {
        t.Error("no sprint should NOT have tick link")
    }
    if links["start-sprint"] == "" {
        t.Error("no sprint should have start-sprint link")
    }
}
```

**Controller test (integration - ONE test):**
```go
func TestAPI_SprintLifecycle(t *testing.T) {
    registry := NewSimRegistry()
    srv := httptest.NewServer(NewRouter(registry))
    defer srv.Close()

    // Create simulation - POST to entry point
    resp := httpPost(srv.URL+"/simulations", `{"seed": 42}`)
    links := parseHAL(resp).Links

    // Start sprint - follow link
    resp = httpPost(srv.URL + links["start-sprint"])
    links = parseHAL(resp).Links

    // Tick until sprint ends (link disappears)
    for links["tick"] != "" {
        resp = httpPost(srv.URL + links["tick"])
        links = parseHAL(resp).Links
    }

    // HATEOAS verification: link presence = correct behavior
    if links["tick"] != "" {
        t.Error("tick link should be absent after sprint ends")
    }
    if links["start-sprint"] == "" {
        t.Error("start-sprint link should be present after sprint ends")
    }
}
```

### What NOT to Test

- `ToState()` - trivial conversion
- `resources.go` - trivial JSON serialization
- HTTP routing - trust `net/http`
- Map access in `SimRegistry` - trivial

---

## Files to Create/Modify

| File | Khorikov Quadrant | Purpose |
|------|-------------------|---------|
| `internal/api/hypermedia.go` | Domain | `LinksFor()` pure function (unit tested) |
| `internal/api/hypermedia_test.go` | - | Unit tests for link generation |
| `internal/api/registry.go` | Controller | `SimRegistry` manages simulation instances |
| `internal/api/handlers.go` | Controller | HTTP handlers (thin, wire domain calls) |
| `internal/api/server.go` | Controller | HTTP server setup, routing |
| `internal/api/resources.go` | Trivial | `SimulationState`, `ToState()` - not tested |
| `internal/api/api_test.go` | - | ONE integration test via httptest |
| `cmd/sofdevsim/main.go` | - | Start API server alongside TUI |
| `docs/use-cases.md` | - | Add UC9 |

---

## option.Option JSON Serialization

`option.Option[T]` needs custom JSON marshaling for HAL responses:

```go
// SimulationState uses option.Option for nullable sprint
type SimulationState struct {
    ID                  string              `json:"id"`
    CurrentTick         int                 `json:"currentTick"`
    CurrentSprintOption option.Option[Sprint] `json:"-"` // Custom handling

    // Computed for JSON
    SprintActive bool    `json:"sprintActive"`
    Sprint       *Sprint `json:"sprint,omitempty"` // nil if not active
}

// ToState converts model to JSON-friendly representation
func ToState(sim model.Simulation) SimulationState {
    state := SimulationState{
        ID:          sim.ID,
        CurrentTick: sim.CurrentTick,
    }

    if sprint, ok := sim.CurrentSprintOption.Get(); ok {
        state.SprintActive = true
        state.Sprint = &sprint
    }

    return state
}
```

---

## Implementation Phases

1. **Phase 1: Domain** - `LinksFor()` with unit tests (TDD)
2. **Phase 2: Resources** - `SimulationState`, `ToState()`
3. **Phase 3: Controller** - `SimRegistry`, handlers, routing
4. **Phase 4: Integration** - ONE lifecycle test
5. **Phase 5: Wire up** - Start API in main.go

---

## Sources

- [HATEOAS - Wikipedia](https://en.wikipedia.org/wiki/HATEOAS)
- [REST Architectural Constraints](https://restfulapi.net/rest-architectural-constraints/)
- [htmx HATEOAS essay](https://htmx.org/essays/hateoas/)
