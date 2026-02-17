#!/bin/bash
# TUI smoke test — verifies binary launches and renders.
# Usage: bin/tui-smoke-test.sh [seed]
#
# This is a smoke test, not an integration test. It confirms the Bubble Tea
# runtime correctly wires key delivery and rendering. All application behavior
# (view navigation, keyboard input, sprint lifecycle) is tested as Q1 domain
# logic via Go unit tests on app.Update() and app.View().

set -euo pipefail

SESSION="sofdevsim-smoke"
OUTDIR="/tmp/tui-smoke"
SEED="${1:-42}"
TIMEOUT=12
PASS=0
FAIL=0
TOTAL=0

rm -rf "$OUTDIR"
mkdir -p "$OUTDIR"
tmux kill-session -t "$SESSION" 2>/dev/null || true

# --- Helpers ---

pane_text() {
    tmux capture-pane -t "$SESSION" -p 2>/dev/null
}

wait_for() {
    local text="$1"
    local timeout="${2:-$TIMEOUT}"
    local i=0 max=$((timeout * 5))
    while ! pane_text | grep -qF "$text"; do
        sleep 0.2
        i=$((i + 1))
        if [ "$i" -ge "$max" ]; then return 1; fi
    done
}

assert() {
    local label="$1" text="$2"
    TOTAL=$((TOTAL + 1))
    if pane_text | grep -qF "$text"; then
        PASS=$((PASS + 1))
        echo "  PASS: $label"
    else
        FAIL=$((FAIL + 1))
        echo "  FAIL: $label — expected '$text'"
        pane_text > "$OUTDIR/FAIL-$(printf '%02d' $TOTAL)-${label// /-}.txt"
    fi
}

# --- Launch ---

echo "=== TUI Smoke Test (seed=$SEED) ==="
echo ""

tmux new-session -d -s "$SESSION" -x 160 -y 45 "/tmp/sofdevsim -seed $SEED"

# Wait for initial render (animation completes, Planning view appears)
echo "[01-planning]"
if ! wait_for "Backlog (" "$TIMEOUT"; then
    echo "  TIMEOUT waiting for initial render"
    TOTAL=$((TOTAL + 1)); FAIL=$((FAIL + 1))
    pane_text > "$OUTDIR/TIMEOUT-planning.txt"
else
    assert "header" "Seed $SEED"
    assert "backlog" "Backlog ("
    assert "help bar" "assign"
    assert "status" "PAUSED"
fi
pane_text > "$OUTDIR/01-planning.txt"

# Quit
tmux send-keys -t "$SESSION" "q"
sleep 0.5

# --- Report ---

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
