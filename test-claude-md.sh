#!/bin/bash
# Behavioral compliance tests: does CLAUDE.md produce FluentFP without being asked?
# Uses neutral "Write Go code" prompts (not "Write FluentFP code").
# Per integration test memory: claude -p --model sonnet, N=3 quick screening.

set -e

CLAUDE_MD_NEW="/home/ted/projects/sofdevsim-2026/CLAUDE.md"
CLAUDE_MD_OLD="/tmp/claude-md-old-baseline.md"
RESULTS="/tmp/claude-md-results.txt"
N=3

# Extract baseline from git (before our edit)
git -C /home/ted/projects/sofdevsim-2026 show HEAD:CLAUDE.md > "$CLAUDE_MD_OLD"

run_test() {
    local claude_md="$1"
    local prompt="$2"

    local test_dir="/tmp/test-claude-$$-$RANDOM"
    mkdir -p "$test_dir"
    cp "$claude_md" "$test_dir/CLAUDE.md"

    cd "$test_dir"
    local output
    output=$(claude -p --model sonnet --max-turns 1 "$prompt" 2>/dev/null || true)
    cd /home/ted/projects/sofdevsim-2026
    rm -rf "$test_dir"
    echo "$output"
}

score() {
    local output="$1"
    if echo "$output" | grep -q 'slice\.From\|slice\.MapTo\|slice\.FindAs'; then
        echo "FLUENTFP"
    elif echo "$output" | grep -q 'for.*range\|for.*:='; then
        echo "LOOP"
    else
        echo "UNCLEAR"
    fi
}

# Simple prompts: one-liner extractions (baseline)
SIMPLE_PROMPTS=(
    "Write Go code to extract all Name fields from a []Developer slice. Assume Developer has a GetName() method. Just the code, no explanation."
    "Write Go code to find the first ticket with UnderstandingLevel == LowUnderstanding in a []model.Ticket slice. Just the code, no explanation."
    "Write Go code to count how many items in a []Ticket slice have Status == \"done\". Just the code, no explanation."
)

# Hard prompts: FluentFP embedded in larger function (real failure mode)
HARD_PROMPTS=(
    "Write a Go function SummaryReport(tickets []Ticket) string that returns a report. It needs to: get a count of tickets where Priority == \"high\", get the average EstimatedDays across all tickets (Ticket has GetEstimatedDays() float64), and format them into a string. Just the code, no explanation."
    "Write a Go function FindIdleDevelopers(devs []Developer, tickets []Ticket) []Developer that returns developers whose ID does not appear in any ticket's AssignedTo field. Developer has GetID() string. Just the code, no explanation."
    "Write a Go function HasOverdueTicket(tickets []Ticket) bool that returns true if any ticket has DaysRemaining < 0. Just the code, no explanation."
)

echo "CLAUDE.md FluentFP Compliance Test - $(date)" | tee "$RESULTS"
echo "N=$N per variant, neutral prompts (no FluentFP mentioned)" | tee -a "$RESULTS"
echo "" | tee -a "$RESULTS"

run_suite() {
    local suite_name="$1"
    shift
    local prompts=("$@")

    echo "--- $suite_name ---" | tee -a "$RESULTS"
    for label_md in "OLD:$CLAUDE_MD_OLD" "NEW:$CLAUDE_MD_NEW"; do
        label="${label_md%%:*}"
        md="${label_md#*:}"
        fluentfp_count=0
        loop_count=0
        total=0

        echo "=== $label CLAUDE.md ===" | tee -a "$RESULTS"
        for prompt in "${prompts[@]}"; do
            for i in $(seq 1 $N); do
                total=$((total + 1))
                output=$(run_test "$md" "$prompt")
                result=$(score "$output")
                echo "  [$label] Trial $i: $result" | tee -a "$RESULTS"
                if [ "$result" = "FLUENTFP" ]; then
                    fluentfp_count=$((fluentfp_count + 1))
                elif [ "$result" = "LOOP" ]; then
                    loop_count=$((loop_count + 1))
                fi
            done
        done
        echo "  $label total: $fluentfp_count/$total FluentFP, $loop_count/$total Loop" | tee -a "$RESULTS"
        echo "" | tee -a "$RESULTS"
    done
}

run_suite "SIMPLE (one-liner extractions)" "${SIMPLE_PROMPTS[@]}"
run_suite "HARD (FluentFP in larger functions)" "${HARD_PROMPTS[@]}"

echo "Results saved to: $RESULTS"
