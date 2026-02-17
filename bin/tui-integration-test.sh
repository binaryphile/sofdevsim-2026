#!/bin/bash
# TUI integration test — exercises the full happy-path user session.
# Usage: bin/tui-integration-test.sh [seed]
#
# Walks through: planning → navigation → assignment → sprint → completion
#                → metrics → comparison → lessons
#
# This complements the smoke test (launch verification) and Go unit tests
# (domain logic). It exercises the actual binary end-to-end via tmux.
#
# Requires: tmux, go (binary at /tmp/sofdevsim)

set -euo pipefail

SESSION="sofdevsim-integration"
OUTDIR="/tmp/tui-integration"
SEED="${1:-42}"
TIMEOUT=12
SPRINT_TIMEOUT=20
PASS=0
FAIL=0
TOTAL=0

rm -rf "$OUTDIR"
mkdir -p "$OUTDIR"
tmux kill-session -t "$SESSION" 2>/dev/null || true

# --- Helpers ---

SNAPSHOT=""

pane_text() {
    tmux capture-pane -t "$SESSION" -p 2>/dev/null
}

wait_for() {
    local text="$1"
    local timeout="${2:-$TIMEOUT}"
    local i=0 max=$((timeout * 5))
    while true; do
        SNAPSHOT=$(pane_text)
        if echo "$SNAPSHOT" | grep -qF "$text"; then return 0; fi
        sleep 0.2
        i=$((i + 1))
        if [ "$i" -ge "$max" ]; then return 1; fi
    done
}

assert() {
    local label="$1" text="$2"
    TOTAL=$((TOTAL + 1))
    if echo "$SNAPSHOT" | grep -qF "$text"; then
        PASS=$((PASS + 1))
        echo "  PASS: $label"
    else
        FAIL=$((FAIL + 1))
        echo "  FAIL: $label — expected '$text'"
        echo "$SNAPSHOT" > "$OUTDIR/FAIL-$(printf '%02d' $TOTAL)-${label// /-}.txt"
    fi
}

send_key() {
    tmux send-keys -t "$SESSION" "$1"
    sleep 0.3
}

capture() {
    echo "$SNAPSHOT" > "$OUTDIR/$1.txt"
}

# --- Launch ---

echo "=== TUI Integration Test (seed=$SEED) ==="
echo ""

tmux new-session -d -s "$SESSION" -x 160 -y 45 "/tmp/sofdevsim -seed $SEED"

# ============================================================
# 01-planning: Initial render
# ============================================================
echo "[01-planning]"
if ! wait_for "Backlog (" "$TIMEOUT"; then
    echo "  TIMEOUT waiting for initial render"
    TOTAL=$((TOTAL + 1)); FAIL=$((FAIL + 1))
    capture "TIMEOUT-01-planning"
else
    # Opening animation blocks input (~4s: 6 devs × 600ms stagger + 500ms walk).
    # "Backlog (" appears immediately but keys are swallowed until animation ends.
    sleep 5
    SNAPSHOT=$(pane_text)
    assert "header seed"         "Seed $SEED"
    assert "header paused"       "PAUSED"
    assert "header backlog"      "Backlog: 12"
    assert "header day"          "Day 0"
    assert "header policy"       "Policy: None"
    assert "backlog title"       "Backlog (12 tickets)"
    assert "help assign"         "a assign"
    assert "help sprint"         "s start sprint"
fi
capture "01-planning"

# ============================================================
# 02-navigation: Tab through all views
# ============================================================
echo "[02-navigation]"

send_key "Tab"
if ! wait_for "No active sprint"; then
    echo "  TIMEOUT waiting for Execution view"
    TOTAL=$((TOTAL + 1)); FAIL=$((FAIL + 1))
else
    assert "execution anchor" "No active sprint"
fi
capture "02a-execution"

send_key "Tab"
if ! wait_for "DORA Metrics"; then
    echo "  TIMEOUT waiting for Metrics view"
    TOTAL=$((TOTAL + 1)); FAIL=$((FAIL + 1))
else
    assert "metrics title"       "DORA Metrics"
    assert "metrics empty"       "No completed tickets yet"
fi
capture "02b-metrics"

send_key "Tab"
if ! wait_for "Policy Comparison"; then
    echo "  TIMEOUT waiting for Comparison view"
    TOTAL=$((TOTAL + 1)); FAIL=$((FAIL + 1))
else
    assert "comparison title"    "Policy Comparison"
    assert "comparison prompt"   "Press 'c'"
fi
capture "02c-comparison"

send_key "Tab"
if ! wait_for "Backlog (12 tickets)"; then
    echo "  TIMEOUT waiting for Planning return"
    TOTAL=$((TOTAL + 1)); FAIL=$((FAIL + 1))
else
    assert "planning returned"   "Backlog (12 tickets)"
fi
capture "02d-planning-return"

# ============================================================
# 03-assignment: Assign first ticket
# ============================================================
echo "[03-assignment]"

send_key "a"
if ! wait_for "Backlog (11 tickets)"; then
    echo "  TIMEOUT waiting for assignment"
    TOTAL=$((TOTAL + 1)); FAIL=$((FAIL + 1))
    capture "TIMEOUT-03-assignment"
else
    assert "backlog shrunk title"  "Backlog (11 tickets)"
    assert "backlog shrunk header" "Backlog: 11"
fi
capture "03-assignment"

# ============================================================
# 04-sprint: Start sprint
# ============================================================
echo "[04-sprint]"

send_key "s"
if ! wait_for "Sprint 1" "$TIMEOUT"; then
    echo "  TIMEOUT waiting for sprint start"
    TOTAL=$((TOTAL + 1)); FAIL=$((FAIL + 1))
    capture "TIMEOUT-04-sprint"
else
    assert "sprint number"  "Sprint 1"
    assert "header running" "RUNNING"
fi
capture "04-sprint-start"

# ============================================================
# 05-completion: Speed up and wait for sprint to end
# ============================================================
echo "[05-completion]"

send_key "+"
send_key "+"
send_key "+"
send_key "+"

if ! wait_for "PAUSED" "$SPRINT_TIMEOUT"; then
    echo "  TIMEOUT waiting for sprint completion"
    TOTAL=$((TOTAL + 1)); FAIL=$((FAIL + 1))
    capture "TIMEOUT-05-completion"
else
    assert "sprint ended"        "PAUSED"
    assert "sprint complete msg" "Sprint complete"
fi
capture "05-completion"

# ============================================================
# 06-metrics: Verify DORA metrics populated
# ============================================================
echo "[06-metrics]"

send_key "Tab"
if ! wait_for "DORA Metrics"; then
    echo "  TIMEOUT waiting for Metrics view"
    TOTAL=$((TOTAL + 1)); FAIL=$((FAIL + 1))
    capture "TIMEOUT-06-metrics"
else
    assert "metrics title"        "DORA Metrics"
    assert "metrics lead time"    "Lead Time"
    assert "sprint ran"           "Day 10"
fi
capture "06-metrics"

# ============================================================
# 07-comparison: Run policy comparison
# ============================================================
echo "[07-comparison]"

send_key "Tab"
if ! wait_for "Press 'c'"; then
    echo "  TIMEOUT waiting for Comparison view"
    TOTAL=$((TOTAL + 1)); FAIL=$((FAIL + 1))
    capture "TIMEOUT-07-comparison-pre"
else
    assert "comparison pre-run" "Press 'c'"
fi

send_key "c"
if ! wait_for "Policy Comparison Results" "$TIMEOUT"; then
    echo "  TIMEOUT waiting for comparison results"
    TOTAL=$((TOTAL + 1)); FAIL=$((FAIL + 1))
    capture "TIMEOUT-07-comparison"
else
    assert "results title"     "Policy Comparison Results"
    assert "dora policy"       "DORA-Strict"
    assert "tameflow policy"   "TameFlow-Cognitive"
fi
capture "07-comparison"

# ============================================================
# 08-lessons: Toggle lesson panel
# ============================================================
echo "[08-lessons]"

send_key "h"
if ! wait_for "Lessons enabled"; then
    echo "  TIMEOUT waiting for lessons panel"
    TOTAL=$((TOTAL + 1)); FAIL=$((FAIL + 1))
    capture "TIMEOUT-08-lessons-on"
else
    assert "lessons enabled"    "Lessons enabled"
    assert "lessons progress"   "Progress:"
fi
capture "08a-lessons-on"

send_key "h"
if ! wait_for "Lessons hidden"; then
    echo "  TIMEOUT waiting for lessons hide"
    TOTAL=$((TOTAL + 1)); FAIL=$((FAIL + 1))
    capture "TIMEOUT-08-lessons-off"
else
    assert "lessons hidden"     "Lessons hidden"
fi
capture "08b-lessons-off"

# --- Quit and Report ---

send_key "q"
sleep 0.5

echo ""
echo "==============================="
echo "Results: $PASS/$TOTAL passed, $FAIL failed"
echo "==============================="
if [ "$FAIL" -gt 0 ]; then
    echo ""
    echo "Failure captures:"
    ls "$OUTDIR"/FAIL-* "$OUTDIR"/TIMEOUT-* 2>/dev/null || true
fi
echo "All captures: $OUTDIR/"

tmux kill-session -t "$SESSION" 2>/dev/null || true

exit "$FAIL"
