# Testing Strategy

## Overview

This document describes the testing approach for sofdevsim-2026, following Khorikov's principles from "Unit Testing: Principles, Practices, and Patterns."

## Test Pyramid

| Layer | Scope | Test Count | Speed |
|-------|-------|------------|-------|
| **Unit** | Pure calculations, domain logic | Many | Fast (<1ms) |
| **Integration** | API endpoints, database, HTTP | Few | Medium (<100ms) |
| **Manual** | TUI interactions, visual verification | Rare | Slow |

## Khorikov Quadrant Classification

Before writing tests, classify code into quadrants:

| Quadrant | Complexity | Collaborators | Strategy |
|----------|------------|---------------|----------|
| **Domain/Algorithms** | High | Few | Unit test heavily |
| **Controllers** | Low | Many | ONE integration test per workflow |
| **Trivial** | Low | Few | Don't test |
| **Overcomplicated** | High | Many | Refactor first |

## Running Tests

```bash
# Full suite
go test ./...

# With coverage
go test -cover ./...

# Specific package
go test -v ./internal/lessons/...

# Run benchmarks
go test -bench=. -benchmem ./internal/engine/

# Race detection
go test -race ./...

# API stress tests only
go test -v -run TestAPI_Concurrent ./internal/api/...
```

## Manual Testing Protocol

### API Server Testing

```bash
# 1. Start the server
go run ./cmd/sofdevsim-server &
# Or: go build -o /tmp/sofdevsim-server ./cmd/sofdevsim-server && /tmp/sofdevsim-server

# 2. Create simulation (use unique seed to avoid conflicts)
SEED=$RANDOM
curl -X POST http://localhost:8080/simulations \
  -H "Content-Type: application/json" \
  -d "{\"policy\": \"tameflow-cognitive\", \"seed\": $SEED}"

# 3. Note the simulation ID from response
SIM_ID="sim-$SEED"

# 4. Start sprint
curl -X POST "http://localhost:8080/simulations/$SIM_ID/start-sprint" \
  -H "Content-Type: application/json"

# 5. Run ticks
curl -X POST "http://localhost:8080/simulations/$SIM_ID/tick" \
  -H "Content-Type: application/json"

# 6. Get state
curl "http://localhost:8080/simulations/$SIM_ID" | jq .

# 7. Test assignment
curl -X POST "http://localhost:8080/simulations/$SIM_ID/assign" \
  -H "Content-Type: application/json" \
  -d '{"ticketId": "TKT-001", "developerId": "dev-1"}'

# 8. Get lessons
curl "http://localhost:8080/simulations/$SIM_ID/lessons" | jq .

# 9. Kill server when done
pkill -f sofdevsim-server
```

### TUI Testing via UIProjection

The UIProjection event-sourced model enables programmatic testing of TUI behavior without visual rendering. See `TestApp_FullSessionWalkthrough` in `app_test.go`.

```go
// Example: verify key→event→state flow
app := NewAppWithSeed(42)
app.Update(tea.KeyMsg{Type: tea.KeyTab})
state := app.uiProjection.State()
// state.CurrentView == ViewExecution ✓
```

**Run walkthrough test:**
```bash
go test -v -run "TestApp_FullSessionWalkthrough" ./internal/tui/...
```

### TUI Visual Inspection (AI-Assisted Testing)

For AI assistants (Claude) to directly verify TUI rendering, use `TestView_Inspect` in `view_inspect_test.go`. This test outputs ANSI-stripped rendered views at key interaction points, enabling visual verification without running the actual TUI.

```bash
go test -v -run "TestView_Inspect" ./internal/tui/...
```

The test walks through:
1. Initial planning view
2. Tab navigation to execution
3. j/k ticket selection
4. h toggle lessons panel
5. s start sprint

Each step outputs the full rendered view with ANSI codes stripped, plus UIProjection state for verification. This enables:
- Verifying layout and content without running the TUI
- Catching view/projection sync bugs (if CurrentView doesn't match rendered view)
- Regression detection when modifying view code

**Example output:**
```
=== 5. After s (Sprint started - Execution view) ===
╭──────────────────────────────────────────────────────────────╮
│  Planning  Execution  Metrics  Comparison   Policy: None ... │
╰──────────────────────────────────────────────────────────────╯
...
=== UIProjection State ===
CurrentView: 1
SelectedTicket: ""
LessonVisible: true
ErrorMessage: ""
```

### TUI Golden File Testing

For CI regression detection, use `TestView_PlanningInitial` in `view_golden_test.go` with the teatest package:

```bash
# Run to verify (fails if output differs from golden file)
go test -v -run "TestView_PlanningInitial" ./internal/tui/...

# Update golden file after intentional changes
go test -v -run "TestView_PlanningInitial" ./internal/tui/... -update
```

Golden files are stored in `testdata/` and capture exact terminal output for baseline comparison.

### TUI Manual Testing (Visual Verification)

For visual/rendering issues not covered by UIProjection:

```bash
# Local engine mode (default)
go run ./cmd/sofdevsim

# Client mode (requires server running)
go run ./cmd/sofdevsim -client

# With specific seed (reproducible)
go run ./cmd/sofdevsim -seed 42
```

**Walkthrough Script (Engine Mode):**

```
1. START
   $ go run ./cmd/sofdevsim -seed 42
   ✓ Planning view shows backlog with 12 tickets
   ✓ 3 developers listed (Alice, Bob, Carol)
   ✓ Status bar shows key hints

2. NAVIGATION
   Press: Tab → Tab → Tab → Tab
   ✓ Cycles through: Planning → Execution → Metrics → Comparison → Planning

   Press: j j j k k
   ✓ Selection moves down 3, up 2 (highlights different ticket)

3. LESSONS PANEL
   Press: h
   ✓ Lesson panel appears on right (Orientation lesson)
   Press: h
   ✓ Lesson panel hides

4. ASSIGNMENT
   Press: j (select second ticket)
   Press: a
   ✓ Ticket assigned to first idle developer
   ✓ Developer shows as busy

   Press: a (try again with no idle dev after assigning all 3)
   ✓ Error shown: "no idle developer"

5. START SPRINT
   Press: s
   ✓ View switches to Execution
   ✓ Sprint timer starts, buffer shows
   ✓ Tickets show progress

   Press: s (try to start again)
   ✓ Error shown: "sprint already active"

6. EXECUTION CONTROLS
   Press: Space
   ✓ Simulation pauses
   Press: Space
   ✓ Simulation resumes

   Press: + + + (3 times)
   ✓ Speed increases (tick interval decreases)
   Press: - -
   ✓ Speed decreases

7. WAIT FOR SPRINT END
   (let it run or hold Space to pause/unpause through ticks)
   ✓ Sprint ends after 10 days
   ✓ Status shows "Sprint complete"
   ✓ Auto-pauses

8. METRICS VIEW
   Press: Tab (to Metrics)
   ✓ DORA metrics displayed
   ✓ Fever chart shows buffer consumption

9. COMPARISON MODE
   Press: Tab (to Comparison)
   Press: c
   ✓ Runs DORA vs TameFlow comparison
   ✓ Results show winner and metrics

10. SAVE/LOAD (Engine mode only)
    Press: Ctrl+s
    ✓ Status shows "Saved to saves/..."

    Press: Ctrl+o
    ✓ Status shows "Loaded..."
    ✓ State restored

11. EXPORT
    Press: e
    ✓ Status shows export path
    ✓ HTML file created in exports/

12. QUIT
    Press: q
    ✓ Clean exit
```

**Walkthrough Script (Client Mode):**

```
# Terminal 1: Start server
$ go run ./cmd/sofdevsim-server

# Terminal 2: Start TUI client
$ go run ./cmd/sofdevsim -client -seed 42

1. VERIFY CLIENT MODE
   ✓ Status bar shows "Client mode"
   ✓ Planning view loads from server

2. SPRINT START
   Press: s
   ✓ HTTP request sent, view switches to Execution

3. TICK (manual in client mode)
   Press: Space
   ✓ Single tick sent to server
   ✓ State updates from response

   Press: Space Space Space (rapid)
   ✓ Only one request at a time (in-flight blocking)

4. ASSIGNMENT
   Press: Tab (back to Planning if between sprints)
   Press: j, then a
   ✓ HTTP assign request sent
   ✓ State updates on success

5. ERROR HANDLING
   (Try invalid operation)
   ✓ Error message appears from HTTP response
   ✓ Error clears on next successful action or navigation
```

**Quick Smoke Test (30 seconds):**
```bash
go run ./cmd/sofdevsim -seed 42
# Press: h s (show lessons, start sprint)
# Watch buffer, press q when satisfied
```

### Expected Error Responses

| Scenario | Expected Response |
|----------|-------------------|
| Duplicate simulation ID | 409 Conflict |
| Simulation not found | 404 Not Found |
| Invalid ticket/dev ID | 404 Not Found |
| Missing Content-Type | 415 Unsupported Media Type |
| Developer already busy | 409 Conflict |

## TUI Testing Architecture

### Current Implementation

```
App (Bubble Tea Model)
├── State (SimulationState)
├── UIProjection (event-sourced UI state)
├── Update(msg) → App, Cmd
└── View() → string (with ANSI codes)
```

**Implemented approaches:**

| Approach | File | Purpose |
|----------|------|---------|
| UIProjection state testing | `app_test.go` | Key→event→state flow |
| Visual inspection | `view_inspect_test.go` | AI-assisted rendering verification |
| Golden file regression | `view_golden_test.go` | CI baseline comparison |

### Future: ViewModel Separation

For Phase 7 HTML export, consider extracting view models:

```go
// Layer 1: Pure data (testable)
type ExecutionViewModel struct {
    Day           int
    BufferPercent float64
    Tickets       []TicketViewModel
}

// Layer 2: Renderer interface
type Renderer interface {
    RenderExecution(vm ExecutionViewModel) string
}

// Layer 3: Implementations
type BubbleTeaRenderer struct{}  // TUI
type HTMLRenderer struct{}       // Export
```

**Benefits of ViewModel separation:**
- Test business logic without rendering
- Swap renderers (TUI, HTML, JSON)
- Cleaner HTML export implementation

## Coverage Baseline

Track coverage changes, not absolute numbers:

| Package | Coverage | Notes |
|---------|----------|-------|
| engine | ~80% | Domain + controller logic |
| export | ~70% | Controller with domain helpers |
| metrics | ~60% | Domain calculations |
| model | ~28% | Mostly data structures (trivial) |
| lessons | ~75% | Domain logic |
| tui | ~0% | UI layer - manual testing |
| api | ~65% | Integration tests |

## Benchmarking

Track benchmarks in `CLAUDE.md` Benchmarks section. Run after each phase:

```bash
go test -bench=. -benchmem ./internal/engine/
go test -bench=. -benchmem ./internal/lessons/
```

Key targets:
- Engine tick: <50ms
- Lesson selection: <1μs
- Pure calculations: 0 allocs/op
