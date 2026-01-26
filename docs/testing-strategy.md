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

### TUI Manual Testing

```bash
# Local engine mode (default)
go run ./cmd/sofdevsim

# Client mode (requires server running)
go run ./cmd/sofdevsim -client

# With specific seed (reproducible)
go run ./cmd/sofdevsim -seed 42

# Force local mode (for save/load, export)
go run ./cmd/sofdevsim -local
```

**TUI Test Checklist:**
- [ ] Views switch correctly (1=Planning, 2=Execution, 3=Metrics, 4=Comparison)
- [ ] Sprint starts with 's' key
- [ ] Tick advances with space
- [ ] Speed changes with +/- keys
- [ ] Pause/resume works with 'p'
- [ ] Lessons panel toggles with 'l'
- [ ] Save/load works with Ctrl+s/Ctrl+o
- [ ] Assignment works by selecting ticket and developer
- [ ] Comparison mode completes successfully

### Expected Error Responses

| Scenario | Expected Response |
|----------|-------------------|
| Duplicate simulation ID | 409 Conflict |
| Simulation not found | 404 Not Found |
| Invalid ticket/dev ID | 404 Not Found |
| Missing Content-Type | 415 Unsupported Media Type |
| Developer already busy | 409 Conflict |

## TUI Integration Testing (Future)

### Current Architecture

```
App (Bubble Tea Model)
├── State (SimulationState)
├── Update(msg) → App, Cmd
└── View() → string (with ANSI codes)
```

### Testing Options

**Option A: Direct View() Testing**
```go
func TestExecutionView(t *testing.T) {
    app := NewApp(...)
    app.state = SimulationState{...}
    output := app.View()
    // Strip ANSI and assert
}
```

**Option B: ViewModel Separation (Recommended for Future)**
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
type BubbleTeaRenderer struct{}  // Production
type PlainTextRenderer struct{}  // Testing
type HTMLRenderer struct{}       // Export
```

**Benefits of ViewModel separation:**
- Test business logic without rendering
- Swap renderers (TUI, plain text, HTML, JSON)
- Supports Phase 5 HTML export cleanly

**Option C: teatest Package**
```go
import "github.com/charmbracelet/x/exp/teatest"

func TestApp_SprintFlow(t *testing.T) {
    tm := teatest.NewTestModel(t, NewApp(...))
    tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
    teatest.RequireOutput(t, tm.FinalOutput(t), ...)
}
```

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
