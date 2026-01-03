# Value Semantics Conversion Assessment

Assessment of converting from pointer-based mutation to value semantics with the `With*` pattern.

## Summary

| Metric | Before | After | Change |
|--------|--------|-------|--------|
| Pointer receivers in model | 23 | 0 | -23 |
| Nil panic surface | 23 methods | 0 methods | Eliminated |
| Method expression compatible | ~5 | ~28 | +23 |
| Code lines (internal/) | - | - | +22 net |
| Correctness verification | Trace pointers | Read linearly | Simplified |

## Detailed Metrics

### 1. Receiver Type Distribution

**Before:**
| File | Pointer Receivers |
|------|-------------------|
| developer.go | 3 |
| sprint.go | 10 |
| incident.go | 3 |
| ticket.go | 7 |
| **Total** | **23** |

**After:** 0 pointer receivers in model types.

All 23 methods converted to value receivers, eliminating nil receiver panic risk.

### 2. Method Definition Overhead

Each mutating method gains one line (`return d`/`return s`/etc.):

```go
// Before: 4 lines
func (d *Developer) Assign(ticketID string) {
    d.CurrentTicket = ticketID
    d.WIPCount++
}

// After: 5 lines (+1)
func (d Developer) WithTicket(ticketID string) Developer {
    d.CurrentTicket = ticketID
    d.WIPCount++
    return d
}
```

**Overhead:** +1 line per mutating method (8 mutating methods = +8 lines in definitions)

### 3. Call Site Verbosity

```go
// Before: 19 characters
dev.Assign("TKT-001")

// After: 32 characters (+13)
dev = dev.WithTicket("TKT-001")
```

**Overhead:** ~13 characters per mutation call site.

**Benefit:** Mutation is now visible at the call site without reading the method.

### 4. Net Code Change

```
14 files changed, 159 insertions(+), 137 deletions(-)
```

Net increase: **+22 lines** in internal/ (excluding documentation).

This includes:
- +8 lines for `return` statements in methods
- +14 lines for explicit write-back patterns in engine
- Offset by removing `moveToCompleted` helper (-10 lines)

### 5. FluentFP Compatibility

Methods usable as method expressions with `slice.From()`:

| Type | Before | After |
|------|--------|-------|
| Developer | 1 (IsIdle) | 4 |
| Incident | 1 (IsOpen) | 4 |
| Sprint | 0 | 10 |
| Ticket | ~3 | ~10 |

Example now possible:
```go
slice.From(developers).KeepIf(Developer.IsIdle)
```

### 6. Clarity Improvement: The Ticket Completion Flow

The pointer-based code required careful reasoning to verify correctness:

```go
// BEFORE: Pointer-based (correct but non-obvious)

// In Tick():
ticket := e.sim.FindTicketByID(dev.CurrentTicket)  // Returns *Ticket pointing into ActiveTickets[i]
// ... work done via pointer ...
if ticket.RemainingEffort <= 0 {
    events := e.advancePhase(ticket, dev)          // Pass pointer
}
// No write-back needed - modifications went through pointer

// In advancePhase():
func (e *Engine) advancePhase(ticket *model.Ticket, dev *model.Developer) []model.Event {
    ticket.CompletedAt = time.Now()                // Modifies ActiveTickets[i] via pointer
    ticket.CompletedTick = e.sim.CurrentTick       // Modifies ActiveTickets[i] via pointer
    dev.CompleteTicket(ticket.ActualDays)          // Modifies Developers[j] via pointer
    e.moveToCompleted(ticket.ID)                   // Find by ID, copy from slice
    // ...
}

// In moveToCompleted():
func (e *Engine) moveToCompleted(ticketID string) {
    for i, t := range e.sim.ActiveTickets {        // t is copied from slice
        if t.ID == ticketID {
            e.sim.CompletedTickets = append(e.sim.CompletedTickets, t)  // Append copy
            // ...
        }
    }
}
```

**Why this was fragile:**

To verify correctness, you must trace:
1. `FindTicketByID` returns `&ActiveTickets[i]` (pointer into slice)
2. Modifications through `ticket` pointer update `ActiveTickets[i]` in place
3. `moveToCompleted` iterates the *same* slice AFTER modifications
4. The `range` copy `t` therefore has the updated `CompletedAt` value
5. The copy appended to `CompletedTickets` is correct

This works, but the correctness depends on:
- Order of operations (modify THEN move)
- No intervening slice modifications
- Understanding Go's pointer semantics into slices

**Value semantics: Obviously correct**

```go
// AFTER: Value-based (obviously correct)

// In Tick():
ticket := e.sim.ActiveTickets[ticketIdx]           // Get copy
// ... work on copy ...
events, ticket, dev := e.advancePhase(ticket, dev) // Returns updated copies

if ticket.Phase == model.PhaseDone {
    e.sim.CompletedTickets = append(e.sim.CompletedTickets, ticket)  // Use THIS ticket
    e.sim.ActiveTickets = append(e.sim.ActiveTickets[:ticketIdx], e.sim.ActiveTickets[ticketIdx+1:]...)
    e.sim.Developers[i] = dev                      // Write back dev
    continue
}
e.sim.ActiveTickets[ticketIdx] = ticket            // Write back ticket
e.sim.Developers[i] = dev                          // Write back dev

// In advancePhase():
func (e *Engine) advancePhase(ticket model.Ticket, dev model.Developer) ([]model.Event, model.Ticket, model.Developer) {
    ticket.CompletedAt = time.Now()                // Modify local copy
    ticket.CompletedTick = e.sim.CurrentTick
    dev = dev.WithCompletedTicket(ticket.ActualDays)
    // ...
    return events, ticket, dev                     // Return updated copies
}
```

**Why this is obviously correct:**

The data flow is explicit:
1. Get a copy
2. Transform the copy
3. Return the transformed copy
4. Caller decides what to do with it

No pointer tracing required. The `ticket` that goes into `CompletedTickets` is visibly the same `ticket` that was just modified. A new reader can verify correctness in seconds.

**Side benefit:** The refactor eliminated `moveToCompleted` entirely (-10 lines). The caller now handles the move directly, which is simpler and removes the need to find-by-ID twice.

## Qualitative Assessment

### Benefits Realized

1. **Nil safety** - 23 potential nil panic sites eliminated
2. **Explicit mutation** - All state changes visible at call sites
3. **Method expressions** - ~23 more methods usable with FluentFP
4. **Local reasoning** - Verify correctness by reading linearly, no pointer tracing
5. **Robustness** - Less fragile to refactoring; data flow can't be broken by reordering

### Costs Incurred

1. **Verbosity** - +13 chars per mutation, +22 lines total
2. **Pattern shift** - Team must learn `x = x.WithFoo()` idiom
3. **Index management** - Must track indices for slice updates

### Verdict

The conversion added ~22 lines of code but:
- Eliminated an entire class of runtime errors (nil panics)
- Made correctness obvious instead of requiring careful analysis
- Enabled method expressions throughout the codebase
- Made mutation explicit at every call site

**Cost-benefit:** The verbosity cost (~1.6% more code) is outweighed by the safety and clarity gains.

**The key insight:** The original pointer code was *correct* but required 5-step reasoning to verify. The value code is *obviously* correct—data flows visibly from input to output. When code is obviously correct, it stays correct through refactoring.

## Files Changed

```
internal/model/developer.go    - 3 methods converted
internal/model/incident.go     - 3 methods converted
internal/model/sprint.go       - 10 methods converted
internal/model/ticket.go       - 7 methods converted
internal/model/simulation.go   - Find methods → index-based
internal/engine/engine.go      - Primary caller, simplified flow
internal/engine/events.go      - Incident resolution updated
internal/engine/variance.go    - Parameter change (value not pointer)
internal/metrics/fever.go      - Parameter change
internal/metrics/tracker.go    - Nil check added
internal/tui/execution.go      - Index-based lookup
*_test.go                      - Call sites updated
```
