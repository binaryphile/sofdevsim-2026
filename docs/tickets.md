# Tickets

## Open

### TKT-001: Add backlog count to header

**Type:** Feature | **Priority:** P2 | **Effort:** S | **Status:** Open

Users have to manually count tickets in Planning view. Add `Backlog: N` to the header bar next to `Done: N`.

**Acceptance Criteria:**
- Header shows `Backlog: N` where N = len(sim.Backlog)
- Updates in real-time as tickets are assigned/decomposed

**Location:** `internal/tui/app.go:417` (headerView function)

---

### TKT-002: Auto-pause when sprint ends

**Type:** Feature | **Priority:** P2 | **Effort:** M | **Status:** Open

Simulation keeps ticking after sprint ends, reaching Day 400+ with idle developers. Should auto-pause or prompt user.

**Acceptance Criteria:**
- Simulation pauses when `CurrentTick >= CurrentSprint.EndDay`
- Status message: "Sprint complete - press 's' for next sprint"

**Location:** `internal/tui/app.go` (tickMsg handler, around line 115)

---

### TKT-004: Comparison shows winner when both metrics are 0.0

**Type:** Bug | **Priority:** P2 | **Effort:** S | **Status:** Open

Comparison view shows "TameFlow ✓" for Lead Time when both policies have 0.0d.

**Steps to Reproduce:**
1. Press 'c' to run comparison
2. Observe Lead Time row: `0.0d | 0.0d | TameFlow ✓`

**Expected:** Should show "TIE" when values are equal

**Blocked By:** TKT-003 (now fixed) - verify if this auto-resolved.

**Location:** `internal/tui/comparison.go`, `internal/metrics/comparison.go:52-56`

---

## Closed

### TKT-003: DORA metrics show 0.0 for Lead Time and MTTR

**Type:** Bug | **Priority:** P1 | **Effort:** M | **Status:** Fixed

Lead Time used wall-clock `time.Now()` which was identical for start/complete in fast simulations. Fixed to use tick-based calculation (`CompletedTick - StartedTick`).

**Closed:** 2026-01-15
