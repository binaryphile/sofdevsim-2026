# Tickets

## Open

---

### TKT-002: Auto-pause when sprint ends

**Type:** Feature | **Priority:** P2 | **Effort:** M | **Status:** Open

Simulation keeps ticking after sprint ends, reaching Day 400+ with idle developers. Should auto-pause or prompt user.

**Acceptance Criteria:**
- Simulation pauses when `CurrentTick >= CurrentSprint.EndDay`
- Status message: "Sprint complete - press 's' for next sprint"

**Location:** `internal/tui/app.go` (tickMsg handler, around line 115)

---

## Closed

### TKT-001: Add backlog count to header

**Type:** Feature | **Priority:** P2 | **Effort:** S | **Status:** Fixed

Added `Backlog: N` to header bar showing count of tickets in backlog. Updates in real-time.

**Closed:** 2026-01-15

---

### TKT-004: Comparison shows winner when both metrics are 0.0

**Type:** Bug | **Priority:** P2 | **Effort:** S | **Status:** Auto-resolved

Comparison view showed "TameFlow ✓" for Lead Time when both policies had 0.0d. Investigation confirmed the comparison logic was correct (`winnerStr` returns "TIE" when neither policy wins). The root cause was TKT-003 - both values were 0.0 due to the wall-clock bug.

**Resolution:** Auto-resolved by TKT-003 fix. Now that Lead Time uses tick-based calculation, comparisons have real values.

**Closed:** 2026-01-15

---

### TKT-003: DORA metrics show 0.0 for Lead Time and MTTR

**Type:** Bug | **Priority:** P1 | **Effort:** M | **Status:** Fixed

Lead Time used wall-clock `time.Now()` which was identical for start/complete in fast simulations. Fixed to use tick-based calculation (`CompletedTick - StartedTick`).

**Closed:** 2026-01-15
