#!/bin/bash
# Behavioral compliance tests for CLAUDE.md
# Creates isolated project folders for each test

set -e

CLAUDE_MD_NEW="/home/ted/projects/sofdevsim-2026/CLAUDE.md"
CLAUDE_MD_OLD="/home/ted/projects/sofdevsim-2026/CLAUDE.md.bak"
RESULTS="/tmp/claude-md-results.txt"

run_test() {
    local name="$1"
    local claude_md="$2"
    local prompt="$3"
    local max_turns="${4:-2}"

    local test_dir="/tmp/test-claude-$$"
    rm -rf "$test_dir"
    mkdir -p "$test_dir"
    cp "$claude_md" "$test_dir/CLAUDE.md"

    cd "$test_dir"
    claude -p --model sonnet --max-turns "$max_turns" "$prompt" 2>/dev/null || true
    rm -rf "$test_dir"
}

echo "CLAUDE.md Behavioral Tests - $(date)" | tee "$RESULTS"
echo "" | tee -a "$RESULTS"

# Test 1: Chain Formatting
echo "=== Test 1: Chain Formatting ===" | tee -a "$RESULTS"
echo "OLD:" | tee -a "$RESULTS"
run_test "chain-old" "$CLAUDE_MD_OLD" "Write FluentFP code to filter tickets where CompletedTick > 100 and count them. Just the code." | tee -a "$RESULTS"
echo "" | tee -a "$RESULTS"
echo "NEW:" | tee -a "$RESULTS"
run_test "chain-new" "$CLAUDE_MD_NEW" "Write FluentFP code to filter tickets where CompletedTick > 100 and count them. Just the code." | tee -a "$RESULTS"
echo "" | tee -a "$RESULTS"

# Test 2: Method Expressions
echo "=== Test 2: Method Expressions ===" | tee -a "$RESULTS"
echo "OLD:" | tee -a "$RESULTS"
run_test "method-old" "$CLAUDE_MD_OLD" "Write FluentFP to extract all Name fields from a []Developer slice. Assume Developer has GetName() method. Just the code." | tee -a "$RESULTS"
echo "" | tee -a "$RESULTS"
echo "NEW:" | tee -a "$RESULTS"
run_test "method-new" "$CLAUDE_MD_NEW" "Write FluentFP to extract all Name fields from a []Developer slice. Assume Developer has GetName() method. Just the code." | tee -a "$RESULTS"
echo "" | tee -a "$RESULTS"

# Test 3: Value Receivers
echo "=== Test 3: Value Receivers ===" | tee -a "$RESULTS"
echo "OLD:" | tee -a "$RESULTS"
run_test "value-old" "$CLAUDE_MD_OLD" "Write a Go method IsEmpty() bool for type Buffer struct { Size int }. Just the method signature and body." 3 | tee -a "$RESULTS"
echo "" | tee -a "$RESULTS"
echo "NEW:" | tee -a "$RESULTS"
run_test "value-new" "$CLAUDE_MD_NEW" "Write a Go method IsEmpty() bool for type Buffer struct { Size int }. Just the method signature and body." 3 | tee -a "$RESULTS"
echo "" | tee -a "$RESULTS"

# Test 4: TDD
echo "=== Test 4: TDD ===" | tee -a "$RESULTS"
echo "OLD:" | tee -a "$RESULTS"
run_test "tdd-old" "$CLAUDE_MD_OLD" "I need AvgTicketSize([]Ticket) float64. What's your first step? One sentence answer." 3 | tee -a "$RESULTS"
echo "" | tee -a "$RESULTS"
echo "NEW:" | tee -a "$RESULTS"
run_test "tdd-new" "$CLAUDE_MD_NEW" "I need AvgTicketSize([]Ticket) float64. What's your first step? One sentence answer." 3 | tee -a "$RESULTS"
echo "" | tee -a "$RESULTS"

echo "=== DONE ===" | tee -a "$RESULTS"
echo "Results saved to: $RESULTS"
