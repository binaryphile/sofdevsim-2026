@/home/ted/projects/share/tandem-protocol/tandem-protocol.md

# Software Development Simulation

## Development Environment

- **Language**: Go
- **Package Management**: Nix flakes for ongoing development
- **Ephemeral Tools**: nix-shell for one-off tool needs

## Code Style: FluentFP

Use `github.com/binaryphile/fluentfp` for fluent, functional patterns where they afford concise but clear code.

### Slice Operations
```go
import "github.com/binaryphile/fluentfp/slice"

// Instead of manual loops for filtering/mapping:
activeTickets := slice.From(tickets).KeepIf(func(t Ticket) bool {
    return t.Phase != PhaseDone
})

// Chain operations fluently:
ticketIDs := slice.From(tickets).ToString(func(t Ticket) string {
    return t.ID
})

// Filter and count:
completedCount := slice.From(tickets).KeepIf(func(t Ticket) bool {
    return t.CompletedTick >= startDay
}).Len()
```

### Option Type (for nullable values)
```go
import "github.com/binaryphile/fluentfp/option"

// Wrap optional values:
assignee := option.Of(ticket.AssignedTo)
devName := assignee.Or("unassigned")

// Comma-ok unwrapping:
if dev, ok := assignee.Get(); ok {
    // process assigned developer
}
```

### Must (panic on error, for init/setup)
```go
import "github.com/binaryphile/fluentfp/must"

// For initialization where errors are fatal:
config := must.Get(loadConfig())
atoi := must.Of(strconv.Atoi)
```

### Ternary Expressions
```go
import "github.com/binaryphile/fluentfp/ternary"

status := ternary.If[string](ticket.Phase == PhaseDone).Then("complete").Else("in progress")
```

### When NOT to Use FluentFP
- Simple single-pass loops that are already clear
- Performance-critical hot paths (stick to plain loops)
- When it would obscure rather than clarify intent

## Testing: Red/Green TDD + Khorikov Principles

Reference: Khorikov, Vladimir. "Unit Testing: Principles, Practices, and Patterns." Manning, 2020.
Summary available at: `/home/ted/projects/urma-obsidian/sources/tier2-silver/practitioner-blogs/khorikov-unit-testing-olano-summary.md`

### TDD Cycle (MANDATORY)

**CRITICAL**: Never implement before writing a failing test. If caught implementing first, STOP and revert.

1. **Red**: Write a failing test first - NO EXCEPTIONS
2. **Green**: Write minimal code to make it pass
3. **Refactor**: Clean up while keeping tests green
4. **Prune**: After refactoring, evaluate if test should be kept (see quadrants below)

### Khorikov's Four Quadrants

Categorize code BEFORE deciding what to test:

| Quadrant | Complexity | Collaborators | Test Strategy |
|----------|------------|---------------|---------------|
| **Domain/Algorithms** | High | Few | Unit test heavily (edge cases) |
| **Controllers** | Low | Many | ONE integration test per happy path |
| **Trivial** | Low | Few | **Don't test at all** |
| **Overcomplicated** | High | Many | Refactor first, then test |

### Domain/Algorithms: Unit Test Heavily
- Pure functions with business logic (e.g., `GetVarianceBounds()`, `IsWithinExpected()`)
- Domain models with calculations (e.g., `Sprint.AvgWIP()`)
- Test all edge cases with table-driven tests

### Controllers: ONE Integration Test
- **One happy path** per major workflow - not multiple!
- Plus edge cases that **cannot** be covered by unit tests
- Example: `Export()` gets ONE test that verifies files created, not separate tests for each file

**Anti-pattern**: Writing TestWriteMetrics, TestWriteTickets, TestWriteSprints separately - these are all ONE controller operation.

### Trivial Code: Don't Test
Examples of trivial code to **skip**:
- `ExportResult.Summary()` - just string formatting
- Simple getters/setters
- Constructors with no logic
- Code that just delegates

### What Makes a Test Bad (Delete It)
- Tests implementation details (e.g., checking for specific string ",5," in CSV)
- Breaks on every refactor but functionality still works
- Redundant with other tests (multiple controller tests for same operation)
- Tests trivial code that can't meaningfully fail

### Test Behavior, Not Implementation
- Test **observable outcomes**: "file exists", "has 5 lines", "returns error"
- Don't test **how**: checking specific string formats, column order, internal state
- Black-box by default: verify outputs, not steps
- Mocks only for **external boundaries** (filesystem, HTTP), never internal collaborators

### Coverage: Diagnostic Only

**How we track:**
1. Run `go test -cover ./...` at end of each phase
2. Update baseline in this file
3. Investigate any package that drops significantly or falls below 60%
4. Note: Low coverage is fine IF the code is trivial (see quadrants)

```bash
go test -cover ./...
```

**Current baseline (2026-01-03):**
| Package | Coverage | Notes |
|---------|----------|-------|
| engine | 79.1% | Domain + controller logic |
| export | 69.8% | Controller with domain helpers |
| metrics | 60.8% | Domain calculations |
| model | 28.4% | Mostly data structures (trivial) - acceptable |
| tui | 0.0% | UI controller - test via manual/integration |

Per Khorikov: Coverage is a "good negative indicator, bad positive one."
- **Below 60%**: Investigate - unless code is trivial/controller
- **High coverage**: Means nothing about quality
- **Don't target a number**: Creates perverse incentive for useless tests
- **Drops matter more than absolutes**: If a package drops 20%, investigate

### Go Test Style
- Prefer **table-driven tests** for domain algorithms
- Use descriptive test names that explain the behavior being tested
- Group related assertions in subtests with `t.Run()`

```go
func TestFoo(t *testing.T) {
    tests := []struct {
        name     string
        input    int
        expected int
    }{
        {"zero input", 0, 0},
        {"positive input", 5, 10},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Foo(tt.input)
            if got != tt.expected {
                t.Errorf("Foo(%d) = %d, want %d", tt.input, got, tt.expected)
            }
        })
    }
}
```

Run tests: `go test ./...`
Run with coverage: `go test -cover ./...`

## Development Process

Our development workflow follows this sequence:

1. **Use Cases** (`/use-case-skill`): Define use cases as the origin of development
2. **Design Documents**: Create design documents from use cases
3. **Implementation Plan**: Plan the implementation with Tandem Protocol contract

Each phase produces artifacts that inform the next, ensuring traceability from user needs to code.

## Branching Strategy: Trunk-Based Development

We use trunk-based development (TBD) to minimize merge complexity and enable continuous integration.

### Core Principles

- **Single trunk**: `main` is the only long-lived branch
- **Short-lived feature branches**: If used, live < 1-2 days max
- **Small, frequent commits**: Commit directly to main when safe
- **Feature flags over branches**: Hide incomplete work behind flags, not branches
- **Always releasable**: Main should always be in a deployable state

### When to Use a Branch

| Situation | Approach |
|-----------|----------|
| Small change (< 1 day) | Commit directly to main |
| Larger feature (1-2 days) | Short-lived branch, merge quickly |
| Experimental/risky | Feature flag on main |
| Multi-day work | Break into smaller pieces that can merge daily |

### Practices

- **No long-lived feature branches**: They create merge hell
- **No release branches**: Tag releases on main instead
- **Continuous integration**: All commits trigger CI on main
- **Code review**: Use small PRs or pair programming
- **Revert over rollback**: If main breaks, revert the commit

### Feature Flags

For incomplete features that span multiple commits:
```go
if config.EnableDataExport {
    // new feature code
}
```

This lets us merge to main continuously without exposing unfinished work.

## Data Output Requirements

The simulation must produce sufficient data output to:
- Compare actual runs against theoretical predictions
- Validate variance models and incident rates
- Enable statistical analysis across multiple seeds
- Support hypothesis testing (DORA vs TameFlow)

## Project Overview

A software development simulation with two evolutionary paths:
1. Full video game
2. LLM-based laboratory for automated software development experimentation
