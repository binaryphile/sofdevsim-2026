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

### TDD Cycle
1. **Red**: Write a failing test first
2. **Green**: Write minimal code to make it pass
3. **Refactor**: Clean up while keeping tests green
4. **Prune**: After refactoring, evaluate if test should be kept (see below)

### Khorikov's Test Value Framework

Categorize code before testing:

| Code Type | Complexity | Collaborators | Test Strategy |
|-----------|------------|---------------|---------------|
| **Domain/Algorithms** | High | Few | Unit test heavily |
| **Controllers** | Low | Many | Integration test only |
| **Trivial** | Low | Few | Don't test |
| **Overcomplicated** | High | Many | Refactor first, then test |

### What to Unit Test
- **Domain models** with business logic (e.g., `Ticket.CalculatePhaseEffort()`)
- **Algorithms** with significant complexity (e.g., variance calculation, decomposition)
- **Pure functions** with meaningful behavior

### What NOT to Unit Test (Prune After TDD)
- Trivial getters/setters and constructors
- Code that just delegates to collaborators
- Implementation details that don't represent business behavior
- Tests that break on every refactor (coupled to internals)

### Test Behavior, Not Implementation
- Test **units of behavior**, not units of code
- A "unit" may span multiple functions/structs if they form one behavior
- Black-box testing by default: verify outputs, not internal steps
- Mocks only for **external boundaries** (DB, HTTP, filesystem), never for internal collaborators

### Integration Tests
- One happy path per major workflow
- Edge cases that can't be covered by unit tests
- Test the full engine simulation loop

### Go Test Style
- Prefer **table-driven tests** for testing multiple cases
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

## Development Process

Our development workflow follows this sequence:

1. **Use Cases** (`/use-case-skill`): Define use cases as the origin of development
2. **Design Documents**: Create design documents from use cases
3. **Implementation Plan**: Plan the implementation with Tandem Protocol contract

Each phase produces artifacts that inform the next, ensuring traceability from user needs to code.

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
