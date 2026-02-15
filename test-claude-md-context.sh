#!/bin/bash
# Context-loaded behavioral compliance test for FluentFP in CLAUDE.md.
# Tests whether CLAUDE.md produces FluentFP compliance under cognitive load.
# Three depth levels: baseline (prompt only), files (source files), session (simulated conversation).
# Compliance = binaryphile/fluentfp import + API usage + no unjustified loops.

set -e

PROJECT_DIR="/home/ted/projects/sofdevsim-2026"
CLAUDE_MD_NEW="$PROJECT_DIR/CLAUDE.md"
CLAUDE_MD_OLD="/tmp/claude-md-old-baseline.md"
RESULTS="/tmp/claude-md-context-results.txt"
OUTPUT_DIR="/tmp/claude-md-context-outputs"
N_BASELINE=3
N_MAIN=5

# Extract old CLAUDE.md from before the FluentFP rewrite
git -C "$PROJECT_DIR" show 3e7e89e^:CLAUDE.md > "$CLAUDE_MD_OLD"

# Create output directory for raw captures
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# =======================================================================
# SCORING -- compliance = correct import + API usage + no unjustified loops
# =======================================================================

score() {
    local output="$1"
    local size=${#output}

    if [ "$size" -lt 10 ]; then
        echo "EMPTY"
        return
    fi

    local has_import=false
    local has_wrong_import=false
    local has_api=false

    echo "$output" | grep -q 'binaryphile/fluentfp' && has_import=true
    echo "$output" | grep -qE 'samber/lo|go-functional|rprtr258|pkg/fluentfp' && has_wrong_import=true
    echo "$output" | grep -qE 'slice\.From|\.KeepIf|\.ToString|\.ToFloat64|\.FindAs|slice\.MapTo' && has_api=true

    # Count unjustified loops (for-loops without // justified: annotation)
    local unjustified_loops
    unjustified_loops=$(echo "$output" | grep -E 'for .*(range|:=)' | grep -cv '// justified' || true)

    if $has_import && $has_api && [ "$unjustified_loops" -eq 0 ]; then
        echo "COMPLIANT"
    elif $has_import && $has_api && [ "$unjustified_loops" -gt 0 ]; then
        echo "MIXED(import+${unjustified_loops}unjustified)"
    elif $has_wrong_import; then
        echo "WRONG_IMPORT"
    elif ! $has_import && $has_api; then
        echo "NO_IMPORT"
    elif [ "$unjustified_loops" -gt 0 ]; then
        echo "LOOPS_ONLY"
    else
        echo "OTHER"
    fi
}

# Binary: COMPLIANT or not. No partial credit.
classify() {
    local result="$1"
    if [ "$result" = "COMPLIANT" ]; then echo "COMPLIANT"
    else echo "NON_COMPLIANT"
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
# PROMPTS -- require complete Go files with imports
# =======================================================================

PROMPTS=(
    "Write a complete Go file with package declaration and all necessary imports. Define a SprintSummary struct with JSON tags. Write a function SprintSummary(sim model.Simulation, sprintID string) (Summary, error) that finds the sprint by ID (return error if not found), filters ActiveTickets to those in this sprint, counts complete vs in-progress, calculates average EstimatedDays for remaining tickets, and returns a populated Summary struct."
    "Write a complete Go file with package declaration and all necessary imports. Define a SprintReport struct with JSON tags. Write a function GenerateSprintReport(sim model.Simulation, sprintID string) (SprintReport, error) that finds the sprint by ID (return error if not found), filters tickets by status (complete, in-progress, blocked), extracts developer names from assigned tickets, computes average lead time for completed tickets, and returns a populated SprintReport struct."
    "Write a complete Go file with package declaration and all necessary imports. Define a TeamMetrics struct with JSON tags. Write a function ComputeTeamMetrics(tickets []model.Ticket, devs []model.Developer) TeamMetrics that counts active tickets, counts completed tickets, extracts unique assignee IDs, calculates average estimated vs actual days ratio, and populates a TeamMetrics struct."
    "Write a complete Go file with package declaration and all necessary imports. Define a TicketHealthReport struct with JSON tags. Write a function TicketHealth(tickets []model.Ticket) TicketHealthReport that counts total tickets, filters to overdue tickets (where ActualDays exceeds EstimatedDays), extracts IDs of overdue tickets, calculates the average overrun ratio (ActualDays/EstimatedDays) for overdue tickets, and counts tickets that have no estimate (EstimatedDays == 0)."
    "Write a complete Go file with package declaration and all necessary imports. Define an ActiveWorkSummary struct with JSON tags. Write a function ActiveWork(tickets []model.Ticket, devs []model.Developer) ActiveWorkSummary that counts active tickets, counts idle developers, extracts names of idle developers, calculates total remaining effort across active tickets (sum of EstimatedDays minus ActualDays for each), and calculates average EstimatedDays across active tickets."
)
# =======================================================================
# TEST RUNNER
# =======================================================================

run_test() {
    local claude_md="$1"
    local full_prompt="$2"
    local output_file="$3"

    local test_dir="/tmp/test-claude-ctx-$$-$RANDOM"
    mkdir -p "$test_dir"
    cp "$claude_md" "$test_dir/CLAUDE.md"

    cd "$test_dir"
    local result
    result=$(claude -p --model sonnet --max-turns 1 --output-format json \
        "$full_prompt" 2>"${output_file%.txt}.err" || true)
    cd "$PROJECT_DIR"
    rm -rf "$test_dir"

    local output
    output=$(echo "$result" | jq -r '.result // ""' 2>/dev/null || echo "$result")

    # Retry once on empty output
    if [ ${#output} -lt 10 ]; then
        echo "  RETRY: empty output, waiting 5s..." >&2
        sleep 5
        local test_dir2="/tmp/test-claude-ctx-$$-$RANDOM"
        mkdir -p "$test_dir2"
        cp "$claude_md" "$test_dir2/CLAUDE.md"
        cd "$test_dir2"
        result=$(claude -p --model sonnet --max-turns 1 --output-format json \
            "$full_prompt" 2>"${output_file%.txt}.retry.err" || true)
        cd "$PROJECT_DIR"
        rm -rf "$test_dir2"
        output=$(echo "$result" | jq -r '.result // ""' 2>/dev/null || echo "$result")
    fi

    echo "$output" > "$output_file"
    echo "$output"
}
# =======================================================================
# SUITE RUNNER
# =======================================================================

run_depth_suite() {
    local depth="$1"
    local n="$2"
    local variants="OLD:$CLAUDE_MD_OLD NEW:$CLAUDE_MD_NEW"

    echo "--- DEPTH: $depth (N=$n) ---" | tee -a "$RESULTS"

    # Build context layers
    local files_ctx=""
    local conv_ctx=""

    if [ "$depth" != "baseline" ]; then
        files_ctx=$(build_files_context)
    fi

    if [ "$depth" = "session" ]; then
        conv_ctx="$CONVERSATION"
    fi

    local num_prompts=${#PROMPTS[@]}

    for label_md in $variants; do
        label="${label_md%%:*}"
        md="${label_md#*:}"
        local compliant_count=0
        local total=0

        echo "=== $label CLAUDE.md ($depth) ===" | tee -a "$RESULTS"
        for pidx in $(seq 0 $((num_prompts - 1))); do
            local prompt="${PROMPTS[$pidx]}"
            for i in $(seq 1 $n); do
                total=$((total + 1))
                local output_file="$OUTPUT_DIR/${depth}-${label}-p${pidx}-t${i}.txt"

                # Build full prompt
                local full_prompt="${files_ctx}${conv_ctx}"$'\n\n'"${prompt}"

                output=$(run_test "$md" "$full_prompt" "$output_file")
                result=$(score "$output")
                class=$(classify "$result")

                echo "  [$label/$depth] p${pidx} t${i}: $result" | tee -a "$RESULTS"

                if [ "$class" = "COMPLIANT" ]; then
                    compliant_count=$((compliant_count + 1))
                fi

                # Inter-trial sleep to avoid rate limiting
                sleep 2
            done
        done
        local rate=0
        if [ "$total" -gt 0 ]; then
            rate=$((compliant_count * 100 / total))
        fi
        echo "  $label/$depth: ${compliant_count}/${total} compliant (${rate}%)" | tee -a "$RESULTS"
        echo "" | tee -a "$RESULTS"
    done
}
# =======================================================================
# MAIN
# =======================================================================

echo "CLAUDE.md Context-Loaded FluentFP Compliance Test - $(date)" | tee "$RESULTS"
echo "Compliance = binaryphile/fluentfp import + API usage + no unjustified loops" | tee -a "$RESULTS"
echo "Baseline N=$N_BASELINE, Files/Session N=$N_MAIN" | tee -a "$RESULTS"
echo "Depths: baseline (prompt only), files (source code), session (conversation)" | tee -a "$RESULTS"
echo "" | tee -a "$RESULTS"

run_depth_suite "baseline" "$N_BASELINE"
run_depth_suite "files" "$N_MAIN"
run_depth_suite "session" "$N_MAIN"

echo "" | tee -a "$RESULTS"
echo "=== SUMMARY ===" | tee -a "$RESULTS"
echo "Results saved to: $RESULTS" | tee -a "$RESULTS"
echo "Raw outputs in: $OUTPUT_DIR" | tee -a "$RESULTS"

# Summary: count by category per depth
echo "" | tee -a "$RESULTS"
for depth in baseline files session; do
    echo "-- $depth --" | tee -a "$RESULTS"
    for cat in COMPLIANT MIXED WRONG_IMPORT NO_IMPORT LOOPS_ONLY EMPTY OTHER; do
        count=$(grep "/$depth]" "$RESULTS" | grep -c "$cat" || true)
        if [ "$count" -gt 0 ]; then
            echo "  $cat: $count" | tee -a "$RESULTS"
        fi
    done
done
# =======================================================================
# DISCRIMINATION CHECK -- Fisher's exact test on session depth
# =======================================================================

echo "" | tee -a "$RESULTS"
echo "=== DISCRIMINATION CHECK (session depth) ===" | tee -a "$RESULTS"

old_compliant=$(grep "/session]" "$RESULTS" | grep "OLD" | grep -c 'COMPLIANT' || true)
old_total=$(grep "/session]" "$RESULTS" | grep -c "OLD" || true)

new_compliant=$(grep "/session]" "$RESULTS" | grep "NEW" | grep -c 'COMPLIANT' || true)
new_total=$(grep "/session]" "$RESULTS" | grep -c "NEW" || true)

echo "  OLD: ${old_compliant}/${old_total} compliant" | tee -a "$RESULTS"
echo "  NEW: ${new_compliant}/${new_total} compliant" | tee -a "$RESULTS"

if [ "$old_total" -gt 0 ] && [ "$new_total" -gt 0 ]; then
    old_rate=$((old_compliant * 100 / old_total))
    new_rate=$((new_compliant * 100 / new_total))
    delta=$((new_rate - old_rate))
    echo "  OLD rate: ${old_rate}%  NEW rate: ${new_rate}%  Delta: ${delta}pp" | tee -a "$RESULTS"

    # Fisher's exact test (pure Python, no scipy needed)
    p_value=$(python3 -c "
from math import comb
def fisher_p(a, b, c, d):
    n = a+b+c+d; r1=a+b; r2=c+d; c1=a+c
    if n == 0: return 1.0
    cutoff = comb(r1,a)*comb(r2,c)/comb(n,c1)
    return sum(comb(r1,i)*comb(r2,c1-i)/comb(n,c1)
               for i in range(min(r1,c1)+1)
               if comb(r1,i)*comb(r2,c1-i)/comb(n,c1) <= cutoff+1e-10)
print(f'{fisher_p($old_compliant, $old_total - $old_compliant, $new_compliant, $new_total - $new_compliant):.4f}')
" 2>/dev/null || echo "N/A")

    echo "  Fisher's exact p-value: $p_value" | tee -a "$RESULTS"

    if [ "$p_value" != "N/A" ]; then
        if python3 -c "exit(0 if float('$p_value') < 0.05 else 1)" 2>/dev/null; then
            echo "  VERDICT: DISCRIMINATES (p=$p_value < 0.05)" | tee -a "$RESULTS"
        elif python3 -c "exit(0 if float('$p_value') < 0.20 else 1)" 2>/dev/null; then
            echo "  VERDICT: TRENDING (p=$p_value < 0.20)" | tee -a "$RESULTS"
        else
            echo "  VERDICT: NO SIGNAL (p=$p_value >= 0.20)" | tee -a "$RESULTS"
        fi
    fi
else
    echo "  VERDICT: INSUFFICIENT DATA" | tee -a "$RESULTS"
fi
