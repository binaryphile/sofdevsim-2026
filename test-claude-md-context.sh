#!/bin/bash
# Context-loaded behavioral compliance test for FluentFP in CLAUDE.md.
# Tests whether CLAUDE.md produces FluentFP under cognitive load (implementation mode).
# Three depth levels: baseline (prompt only), files (source files), session (simulated conversation).
# Per integration test infrastructure: claude -p --model sonnet, N=3 quick screening.

set -e

PROJECT_DIR="/home/ted/projects/sofdevsim-2026"
CLAUDE_MD_NEW="$PROJECT_DIR/CLAUDE.md"
CLAUDE_MD_OLD="/tmp/claude-md-old-baseline.md"
RESULTS="/tmp/claude-md-context-results.txt"
OUTPUT_DIR="/tmp/claude-md-context-outputs"
N=3

# Extract old CLAUDE.md from before the FluentFP rewrite
git -C "$PROJECT_DIR" show 3e7e89e^:CLAUDE.md > "$CLAUDE_MD_OLD"

# Create output directory for raw captures
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# =======================================================================
# SCORING (enhanced with per-operation counting)
# =======================================================================

score() {
    local output="$1"
    local fp_count=0
    local loop_count=0

    # Count FluentFP operations (grep -o counts each match, wc -l totals)
    fp_count=$(echo "$output" | grep -oE 'slice\.From|\.KeepIf|\.ToString|\.ToFloat64|\.FindAs|slice\.MapTo' | wc -l)

    # Count loop operations
    loop_count=$(echo "$output" | grep -oE 'for .*(range|:=)' | wc -l)

    if [ "$fp_count" -gt 0 ] && [ "$loop_count" -eq 0 ]; then
        echo "FLUENTFP(${fp_count}fp)"
    elif [ "$fp_count" -eq 0 ] && [ "$loop_count" -gt 0 ]; then
        echo "LOOP(${loop_count}loop)"
    elif [ "$fp_count" -gt 0 ] && [ "$loop_count" -gt 0 ]; then
        echo "MIXED(${fp_count}fp/${loop_count}loop)"
    else
        echo "UNCLEAR"
    fi
}

classify() {
    local result="$1"
    if echo "$result" | grep -q '^FLUENTFP'; then echo "FLUENTFP"
    elif echo "$result" | grep -q '^LOOP'; then echo "LOOP"
    elif echo "$result" | grep -q '^MIXED'; then echo "MIXED"
    else echo "UNCLEAR"
    fi
}

# =======================================================================
# CONTEXT BUILDERS
# =======================================================================

build_files_context() {
    local context=""
    context+="<system-reminder>"
    context+=$'\nYou are implementing a feature in sofdevsim-2026. Here are the files you have been reading:\n'

    for filepath in \
        "$PROJECT_DIR/internal/model/developer.go" \
        "$PROJECT_DIR/internal/model/ticket.go" \
        "$PROJECT_DIR/internal/model/simulation.go" \
        "$PROJECT_DIR/internal/engine/engine.go" \
        "$PROJECT_DIR/internal/api/handlers.go"; do
        local relpath="${filepath#$PROJECT_DIR/}"
        context+=$'\nFile: '"$relpath"$'\n'
        context+="$(cat "$filepath")"$'\n'
    done

    context+="</system-reminder>"
    printf '%s' "$context"
}

# Simulated 7-turn conversation to create prior-reasoning tracking load.
CONVERSATION='The following is the conversation so far:

User: "I need to add sprint health metrics to the API. Can you read the relevant files first?"

Assistant: "I have read the relevant files. Here is what I found:

1. engine.go uses an immutable pattern -- Tick() returns a new Engine. It processes tickets through workflow phases and uses FluentFP for some operations (like TryDecompose) but plain loops for complex state mutation in Tick().

2. handlers.go follows a resource-mapping pattern with ToResource functions and JSON response helpers. Handlers get the simulation state and map it to API resources.

3. model types use value semantics -- Ticket has IsActive(), IsComplete(), GetEstimatedDays(). Developer has IsIdle(), GetID(), GetName().

I will add a new handler with resource mapping."

User: "Should this go in handlers.go or a separate file?"

Assistant: "handlers.go -- it follows the existing pattern. I will add a SprintSummary resource type and a handler that aggregates ticket data from the simulation state."

User: "What about error handling for missing sprints?"

Assistant: "I will return 404 with the standard error response pattern from the existing handlers. Let me also handle the case where no tickets exist for the sprint."

User: "proceed"'

# =======================================================================
# PROMPTS (multi-concern, FluentFP is one piece)
# =======================================================================

PROMPTS=(
    "Write a Go function SprintSummary(sim model.Simulation, sprintID string) (Summary, error) that finds the sprint by ID (return error if not found), filters ActiveTickets to those assigned to this sprint, counts how many are complete vs in-progress, calculates average EstimatedDays for remaining tickets, and returns a populated Summary struct. Just the code, no explanation."
    "Write a Go function GenerateSprintReport(sim model.Simulation, sprintID string) (string, error) that finds the sprint by ID, filters tickets by status, extracts developer names from assigned tickets, computes average lead time, formats into a multi-section report string. Return error if sprint not found. Just the code, no explanation."
    "Write a Go function ComputeTeamMetrics(tickets []model.Ticket, devs []model.Developer) TeamMetrics that counts tickets per status, extracts unique assignee IDs, calculates average estimated vs actual days ratio, and populates a TeamMetrics struct. Just the code, no explanation."
)

# =======================================================================
# TEST RUNNER
# ======================================================================

run_test() {
    local claude_md="$1"
    local full_prompt="$2"
    local output_file="$3"

    local test_dir="/tmp/test-claude-ctx-$$-$RANDOM"
    mkdir -p "$test_dir"
    cp "$claude_md" "$test_dir/CLAUDE.md"

    cd "$test_dir"
    local result
    result=$(claude -p --model sonnet --max-turns 1 --output-format json "$full_prompt" 2>/dev/null || true)
    cd "$PROJECT_DIR"
    rm -rf "$test_dir"

    # Extract result text from JSON output
    local output
    output=$(echo "$result" | jq -r '.result // empty' 2>/dev/null || echo "$result")

    # Save raw output
    echo "$output" > "$output_file"

    echo "$output"
}

# =======================================================================
# SUITE RUNNER
# ======================================================================

run_depth_suite() {
    local depth="$1"
    local variants=""

    # baseline only runs NEW CLAUDE.md
    if [ "$depth" = "baseline" ]; then
        variants="NEW:$CLAUDE_MD_NEW"
    else
        variants="OLD:$CLAUDE_MD_OLD NEW:$CLAUDE_MD_NEW"
    fi

    echo "--- DEPTH: $depth ---" | tee -a "$RESULTS"

    # Build context layers
    local files_ctx=""
    local conv_ctx=""

    if [ "$depth" != "baseline" ]; then
        files_ctx=$(build_files_context)
    fi

    if [ "$depth" = "session" ]; then
        conv_ctx="$CONVERSATION"
    fi

    for label_md in $variants; do
        label="${label_md%%:*}"
        md="${label_md#*:}"
        fp_count=0
        loop_count=0
        mixed_count=0
        total=0

        echo "=== $label CLAUDE.md ($depth) ===" | tee -a "$RESULTS"
        for pidx in 0 1 2; do
            local prompt="${PROMPTS[$pidx]}"
            for i in $(seq 1 $N); do
                total=$((total + 1))
                local output_file="$OUTPUT_DIR/${depth}-${label}-p${pidx}-t${i}.txt"

                # Build full prompt
                local full_prompt="${files_ctx}${conv_ctx}"$'\n\n'"${prompt}"

                output=$(run_test "$md" "$full_prompt" "$output_file")
                result=$(score "$output")
                class=$(classify "$result")

                echo "  [$label/$depth] p${pidx} t${i}: $result" | tee -a "$RESULTS"

                case "$class" in
                    FLUENTFP) fp_count=$((fp_count + 1)) ;;
                    LOOP) loop_count=$((loop_count + 1)) ;;
                    MIXED) mixed_count=$((mixed_count + 1)) ;;
                esac
            done
        done
        echo "  $label/$depth: ${fp_count}/${total} FluentFP, ${loop_count}/${total} Loop, ${mixed_count}/${total} Mixed" | tee -a "$RESULTS"
        echo "" | tee -a "$RESULTS"
    done
}

# ======================================================================
# MAIN
# =======================================================================

echo "CLAUDE.md Context-Loaded FluentFP Compliance Test - $(date)" | tee "$RESULTS"
echo "N=$N per variant per depth, neutral multi-concern prompts" | tee -a "$RESULTS"
echo "Depths: baseline (prompt only), files (source code), session (conversation)" | tee -a "$RESULTS"
echo "" | tee -a "$RESULTS"

for depth in baseline files session; do
    run_depth_suite "$depth"
done

echo "" | tee -a "$RESULTS"
echo "=== SUMMARY ===" | tee -a "$RESULTS"
echo "Results saved to: $RESULTS" | tee -a "$RESULTS"
echo "Raw outputs in: $OUTPUT_DIR" | tee -a "$RESULTS"

# Summary: count results by depth
echo "" | tee -a "$RESULTS"
for depth in baseline files session; do
    echo "-- $depth --" | tee -a "$RESULTS"
    echo -n "  FLUENTFP: " | tee -a "$RESULTS"; (grep "/$depth]" "$RESULTS" | grep -c 'FLUENTFP' || echo 0) | tee -a "$RESULTS"
    echo -n "  LOOP: " | tee -a "$RESULTS"; (grep "/$depth]" "$RESULTS" | grep -c 'LOOP' || echo 0) | tee -a "$RESULTS"
    echo -n "  MIXED: " | tee -a "$RESULTS"; (grep "/$depth]" "$RESULTS" | grep -c 'MIXED' || echo 0) | tee -a "$RESULTS"
done

# Automated discrimination check
# Compare session-depth OLD vs NEW FluentFP rates (includes MIXED as partial credit)
echo "" | tee -a "$RESULTS"
echo "=== DISCRIMINATION CHECK ===" | tee -a "$RESULTS"

old_fp=$(grep "/session]" "$RESULTS" | grep "OLD" | grep -c 'FLUENTFP' || echo 0)
old_mixed=$(grep "/session]" "$RESULTS" | grep "OLD" | grep -c 'MIXED' || echo 0)
old_total=$(grep "/session]" "$RESULTS" | grep -c "OLD" || echo 0)

new_fp=$(grep "/session]" "$RESULTS" | grep "NEW" | grep -c 'FLUENTFP' || echo 0)
new_mixed=$(grep "/session]" "$RESULTS" | grep "NEW" | grep -c 'MIXED' || echo 0)
new_total=$(grep "/session]" "$RESULTS" | grep -c "NEW" || echo 0)

echo "  Session depth OLD: ${old_fp}/${old_total} pure FP, ${old_mixed}/${old_total} mixed" | tee -a "$RESULTS"
echo "  Session depth NEW: ${new_fp}/${new_total} pure FP, ${new_mixed}/${new_total} mixed" | tee -a "$RESULTS"

if [ "$old_total" -gt 0 ] && [ "$new_total" -gt 0 ]; then
    # Discrimination: NEW should have more FluentFP than OLD at session depth
    old_score=$((old_fp * 100 / old_total))
    new_score=$((new_fp * 100 / new_total))
    delta=$((new_score - old_score))
    echo "  OLD FluentFP rate: ${old_score}%  NEW FluentFP rate: ${new_score}%  Delta: ${delta}pp" | tee -a "$RESULTS"

    if [ "$delta" -gt 20 ]; then
        echo "  VERDICT: DISCRIMINATES (NEW > OLD by ${delta}pp, threshold 20pp)" | tee -a "$RESULTS"
    elif [ "$delta" -gt 0 ]; then
        echo "  VERDICT: WEAK SIGNAL (NEW > OLD by ${delta}pp, below 20pp threshold)" | tee -a "$RESULTS"
    else
        echo "  VERDICT: NO DISCRIMINATION (delta ${delta}pp). Simulated context does not reproduce the failure mode." | tee -a "$RESULTS"
        echo "  NEXT STEP: Test with real multi-turn agentic sessions." | tee -a "$RESULTS"
    fi
else
    echo "  VERDICT: INSUFFICIENT DATA" | tee -a "$RESULTS"
fi
