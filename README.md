# Software Development Simulation

Simulate software delivery to test DORA vs TameFlow sizing strategies.

## Why This Matters

Teams argue endlessly about sizing:
- *"Just break it down into smaller pieces"*
- *"We need to understand it better first"*

**DORA research** says batch size matters: tickets taking longer than one week correlate with worse delivery outcomes.

**TameFlow** argues that cognitive load (understanding level) is the real discriminant: uncertain work causes variance regardless of size.

This simulation lets you test both hypotheses with data.

## The Experiment

| Policy | Rule | Theory |
|--------|------|--------|
| **DORA-Strict** | Decompose tickets > 5 days | Time-based ceiling reduces batch size |
| **TameFlow-Cognitive** | Decompose tickets with Low understanding | Reducing uncertainty improves predictability |
| **Hybrid** | Both conditions | Belt and suspenders |
| **None** | No decomposition | Baseline |

Run the same scenario under each policy. Compare DORA metrics. See which approach wins.

## Features

- **8-phase workflow** (Research → Sizing → Planning → Implement → Verify → CI/CD → Review → Done)
- **4 sizing policies** with automatic decomposition
- **Variance model** (understanding level → outcome predictability)
- **DORA metrics dashboard** with ntcharts sparklines
- **Fever chart** (buffer consumption: Green/Yellow/Red)
- **A/B policy comparison** with identical seeds for reproducibility
- **Batch experiments** — declarative JSON config + N-seed sweeps + R/Python-ready per-run CSV bundles (see [Batch Experiments](#batch-experiments) below)

## How It Works

The simulation's core insight: **understanding level determines predictability**.

| Understanding | Variance | What Happens |
|---------------|----------|--------------|
| **High** | ±5% | Ticket completes close to estimate |
| **Medium** | ±20% | Some surprises, moderate slippage |
| **Low** | ±50% | Frequent surprises, major slippage possible |

This is why TameFlow-Cognitive often beats DORA-Strict: decomposing by *uncertainty* (Low understanding) reduces variance more than decomposing by *size* (>5 days).

**Example:** A 3-day ticket with Low understanding has more variance (1.5-4.5 days) than a 6-day ticket with High understanding (5.7-6.3 days). DORA-Strict would decompose the 6-day ticket but leave the risky 3-day ticket alone.

## Screenshot

```
┌──────────────────────────────────────────────────────────────────────────────┐
│  Planning   Execution   Metrics   Comparison      Policy: DORA-Strict │ Day 5│
├──────────────────────────────────────────────────────────────────────────────┤
│ Backlog (8 tickets)                              │ Team                      │
│ ────────────────────────────────────────────────│                           │
│ ID       Title                        Est  Understanding  Phase             │ Alice (v:1.0) [busy] → TKT-001 │
│ ▶TKT-002 Implement auth flow         4.5d Medium         Backlog           │ Bob (v:0.8)   [idle]           │
│  TKT-003 Add logging middleware      2.0d High           Backlog           │ Carol (v:1.2) [busy] → TKT-004 │
│  TKT-005 Refactor database layer     8.2d Low            Backlog           │                                │
│  TKT-006 Write API documentation     1.5d High           Backlog           │                                │
│  TKT-007 Fix memory leak             3.0d Medium         Backlog           │                                │
└──────────────────────────────────────────────────────────────────────────────┘

┌─ Execution View ─────────────────────────────────────────────────────────────┐
│ Sprint 1                                                                     │
│ Day 5/10  [████████░░░░░░░░░░░░] 50%                                        │
│                                                                              │
│ Active Work                                                                  │
│ Alice    → TKT-001   [████████████░░░░░░░░] 60% (Implement)                 │
│ Bob      [idle]                                                              │
│ Carol    → TKT-004   [██████░░░░░░░░░░░░░░] 30% (Verify)                    │
│                                                                              │
│ ┌─ Fever Chart ──────────────┐  ┌─ Events ─────────────────────────────────┐│
│ │ Buffer: [████░░░░░░] 38%   │  │ Day 5: TKT-001 entered Implement phase  ││
│ │ 0.8 / 2.0 days remaining   │  │ Day 4: Bug found in TKT-004 (+0.5d)     ││
│ │ Status: GREEN              │  │ Day 3: TKT-003 completed                ││
│ └────────────────────────────┘  │ Day 2: Sprint 1 started                 ││
│                                 └──────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────────────────┘

┌─ Metrics View ───────────────────────────────────────────────────────────────┐
│ DORA Metrics                                                                 │
│                                                                              │
│   Lead Time       Deploy Freq      MTTR            Change Fail Rate         │
│   2.3 days        0.43/day         1.2 days        8.3%                     │
│   ▂▃▄▃▂▃▄▅▆▅▄▃   ▁▂▂▃▃▄▄▅▅▆▆▇   ▇▆▅▄▃▃▂▂▁▁▁   ▅▄▄▃▃▂▂▂▁▁▁               │
│   ↓ lower better  ↑ higher better  ↓ lower better  ↓ lower better           │
└──────────────────────────────────────────────────────────────────────────────┘

┌─ Comparison View ────────────────────────────────────────────────────────────┐
│ Policy Comparison Results                                                    │
│ Seed: 1704067200  |  Sprints: 3  |  Same backlog & team                     │
│                                                                              │
│ Metric               DORA-Strict    TameFlow-Cognitive    Winner            │
│ ─────────────────────────────────────────────────────────────────           │
│ Lead Time                   3.2d                  2.8d    TameFlow ✓        │
│ Deploy Frequency         0.38/d                0.42/d    TameFlow ✓        │
│ MTTR                       1.5d                  1.1d    TameFlow ✓        │
│ Change Fail Rate          12.5%                  8.2%    TameFlow ✓        │
│                                                                              │
│ WINNER: TameFlow-Cognitive (4-0 on DORA metrics)                            │
│                                                                              │
│ Experiment suggests: COGNITIVE LOAD decomposition led to better outcomes.   │
│ • Lead time improved by 0.4 days (13% faster)                               │
│ • Change fail rate reduced by 4.3% points                                   │
│ • Understanding level is a stronger discriminant than time estimate         │
└──────────────────────────────────────────────────────────────────────────────┘

Keybindings: [a]ssign [d]ecompose [p]olicy [s]tart sprint | Tab:view Space:pause +/-:speed [c]ompare [e]xport [q]uit
```

## Quick Start

### With Nix

```bash
nix develop
go run cmd/sofdevsim/main.go
```

### Without Nix

```bash
go mod download
go run cmd/sofdevsim/main.go
```

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--seed` | 0 | Random seed for reproducibility (0 = use current time) |
| `--api-port` | 8080 | HTTP API port |
| `--mix` | `healthy` | Backlog mix profile (see "Backlog Mix Profiles" below) |
| `--phase-wip` | none | Per-phase WIP caps as `phase=cap,...` (see "Per-Phase WIP Caps" below) |
| `--phase-wip-profile` | `uncapped` | Bundled WIP profile shortcut (`uncapped` or `balanced`) |
| `--release-mode` | `push` | Release controller mode (`push` or `demand`; see "Demand-Driven Release" below) |

**Example:** Run with fixed seed for reproducible results:
```bash
go run cmd/sofdevsim/main.go --seed 42
```

The above flags belong to the interactive `sofdevsim` TUI binary. For batch / headless experiments, see [Batch Experiments](#batch-experiments) below — the `sofdevsim-batch` binary takes `-config <path>` and `-out <dir>` instead.

## Backlog Mix Profiles

The `--mix` flag selects a named backlog generation profile. Mix profiles control how the simulation's initial backlog is composed across ticket types (UC37: heterogeneous ticket types). Each ticket type has a different phase-effort distribution, so changing the mix can shift the system's bottleneck phase.

| Profile | Ticket types | Use for |
|---|---|---|
| `healthy` (default) | all Feature | Today's baseline — homogeneous backlog, regression-safe default |
| `overloaded` | all Feature | Existing: high WIP stress, all-Feature |
| `uncertain` | all Feature | Existing: high Low-understanding variance, all-Feature |
| `mixed` | all Feature | Existing: varied understanding/estimate/priority, all-Feature (name predates UC37) |
| `uc37-default` | 60% Feature / 25% Bug / 10% Spike / 5% Migration | Demonstrates a moderately mixed backlog with Implement-dominant aggregate |
| `bug-heavy` | 30% F / 50% Bug / 5% Spike / 10% Migration / 5% Infra | Bug-fix-heavy quarter |
| `migration-quarter` | 30% F / 15% B / 5% Spike / 45% Migration / 5% Infra | Migration-heavy quarter (Verify-shifting) |
| `infra-push` | 35% F / 15% B / 5% Spike / 10% Migration / 35% Infra | Infra/platform-heavy quarter (CI/CD-shifting) |
| `research-shop` | 5% F / 5% B / 85% Spike / 0% Migration / 5% Infra | Heavy-research-mode shop (Research-dominant aggregate; useful contrast to uc37-default) |

**Example:** Run a comparison between contrasting mixes:
```bash
go run cmd/sofdevsim/main.go --seed 42 --mix uc37-default
go run cmd/sofdevsim/main.go --seed 42 --mix research-shop
```

Unknown mix names are rejected at startup with a diagnostic listing registered scenarios. Mix profile selection is also available via the REST API: `POST /simulations` accepts an optional `scenarioName` field (default `healthy`).

See `docs/design.md` §"Heterogeneous Ticket Types (UC37)" for per-type phase-effort distributions and the design rationale.

## Per-Phase WIP Caps

The `--phase-wip` flag (or `--phase-wip-profile` shortcut) caps how many concurrent tickets each phase may host. Cap-blocked queues surface as head-of-line blocking in the new Phase Queues panel (UC38: per-phase WIP caps).

| Profile | Research | Sizing | Planning | Implement | Verify | CI/CD | Review | Use case |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| `uncapped` (default) | nil | nil | nil | nil | nil | **2 (via CICDSlots)** | nil | Most-regression-safe; nil = unlimited; CI/CD effective cap of 2 reflects the existing `CICDSlots` declaration now wired into the cap path |
| `balanced` | nil | nil | nil | 4 | 2 | 1 | 2 | Demonstrates head-of-line blocking under heterogeneous mix; CI/CD=1 (explicit, overrides CICDSlots fallback) |

**Example:** Run with explicit per-phase caps:
```bash
go run cmd/sofdevsim/main.go --seed 42 --mix uc37-default \
  --phase-wip Implement=4,Verify=2,CICD=1,Review=2
```

The flag's phase-name parser accepts the canonical phase names (`Research`, `Sizing`, `Planning`, `Implement`, `Verify`, `CI/CD`, `Review`) AND the slash-free `CICD` alias for `CI/CD` (parser-friendly); matching is case-insensitive. Invalid configs are rejected at simulation startup with a typed diagnostic identifying the failing phase, the cap value, and the rule that was violated. The four diagnostic categories are:

| Sentinel | Trigger |
|---|---|
| `ErrCapZero` | Any cap = 0 — would deadlock that phase |
| `ErrCapNegative` | Any cap < 0 — semantic error |
| `ErrCapBelowMentorMin` | `Implement` cap < 2 — mentor-pair minimum |
| `ErrCapConflict` | Per-phase cap exceeds the aggregate rope-style WIP ceiling on the Implement→Review span |

Per-phase caps are also available via the REST API: `POST /simulations` accepts an optional `phaseWIPConfig` object field (e.g., `{"Implement": 4, "Verify": 2}`). Validation errors return HTTP 422 (Unprocessable Entity) with sentinel-differentiated diagnostics.

**Behavior-change note**: pre-UC38 the `CICDSlots` field was declared but never enforced. Post-UC38, `PhaseWIPCap(CICD)` falls back to `CICDSlots` (default 2), so CI/CD throughput is now bound to ≤ 2 concurrent tickets in the absence of an explicit override. For typical mixes (CI/CD phase effort ≈ 5%) this is invisible; for CI/CD-bound pathological runs it surfaces as observable head-of-line blocking on the CI/CD queue.

See `docs/design.md` §"Per-Phase WIP Caps (UC38)" for the schema, dual-checkpoint enforcement, and direct-CICDSlots-reader migration notes.

## Demand-Driven Release

The `--release-mode` flag selects between two release controllers (UC39: demand-driven release):

| Mode | Behavior |
|---|---|
| `push` (default) | Current commit-then-flow behavior — `StartSprint` commits to capacity; assignment + phase-advance flow as today. Zero-value default for regression-safety |
| `demand` | `StartSprint` skips bulk-commit; release controller drips one ticket per tick when downstream headroom permits. Warm-up phase forces push behavior until the TOC analyzer locks a constraint with at least medium confidence |

**Example:** Run in demand mode for the same backlog as a push baseline:
```bash
go run cmd/sofdevsim/main.go --seed 42 --mix uc37-default --release-mode demand
go run cmd/sofdevsim/main.go --seed 42 --mix uc37-default --release-mode push
```

Aggregate WIP under demand mode should be materially lower (UC39 §Postconditions/Success); verifiable from the `avg_wip` column in `sprints.csv` exports.

**Mode indicator**: the TUI header displays one of 4 states:
- `Mode: push` — push mode (zero-value default)
- `Mode: demand (warming)` — warmup running (waiting for analyzer lock)
- `Mode: demand (push fallback)` — warmup-timeout fired (sim falls back to push behavior; terminal for the run)
- `Mode: demand` — post-warmup-exit (release controller is dripping)

**Headroom formula**: `floor((1 - Buffer.Penetration) × MaxBacklogDrip)`. With MaxBacklogDrip default 1, fully-green admits 1 ticket/tick; fully-red (Penetration=1.0) throttles to 0.

**Warmup-timeout**: hard-coded to N=5 sprints. If the TOC analyzer can't lock a constraint within 5 sprints (e.g., on a degenerate mix), the `WarmupTimedOut` event fires, `WarmupFailed` becomes true, and the sim falls back to effective push behavior for the rest of its life. The TUI Mode indicator reflects this as `demand (push fallback)`.

Validation errors map to:

| Sentinel | Trigger |
|---|---|
| `ErrInvalidReleaseMode` | Unknown mode name (e.g., `--release-mode garbage`); HTTP 422 from REST |
| `ErrAnalyzerNotReady` | Defensive guard — controller called before analyzer has any ticks |
| `ErrWarmupTimeout` | Returned (non-fatal) by the release controller alongside the `WarmupTimedOut` event |

Demand mode is also available via the REST API: `POST /simulations` accepts an optional `releaseMode` field. Invalid mode values return HTTP 422.

See `docs/design.md` §"Demand-Driven Release (UC39)" for the state machine, ShouldAdmit pseudocode, and warmup-exit-vs-timeout race resolution.

## Investment Moves

Between sprints, the operator spends a finite **Budget** on capacity-changing investments targeted at the analyzer-identified constraint (UC40: closes the 5FS EXPLOIT/ELEVATE game loop). The investment window opens automatically when a sprint ends; press 's' to start the next sprint (which closes the window).

**Starting budget**: 10. **Per-option costs**:

| Option | Cost | Effect |
|---|---:|---|
| `hire` (Hire developer) | 5 | Adds a new developer (auto-generated ID; default velocity 1.0) |
| `cicd-slot` (Buy CI/CD slot) | 3 | Increments CI/CD pipeline-slot count (works with UC38's PhaseWIPCap fallback) |
| `review-tool` (Upgrade Review tooling) | 2 | Multiplies Review-phase velocity by 1.2 (stacks across investments) |
| `verify-paydown` (Pay down Verify tech debt) | 2 | Multiplies Verify-phase variance by 0.8 (stacks; lower = less variance) |

**Budget: $N** is always visible in the TUI header alongside Mode and Policy.

**TUI**: When the investment window is open, the Execution view body renders numbered options `[1]Hire($5) [2]CICDSlot($3) [3]ReviewTool($2) [4]VerifyPaydown($2)` (grayed when unaffordable). Press 1–4 to spend the corresponding option.

**REST**: `POST /simulations/{id}/investments` with body `{"option": "hire|cicd-slot|review-tool|verify-paydown"}`. Returns updated sim state on success.

Validation errors map to:

| Sentinel | Trigger |
|---|---|
| `ErrInsufficientBudget` | Cost > remaining budget; HTTP 422 |
| `ErrInvalidInvestment` | Unknown option name; HTTP 422 |
| `ErrInvestmentWindowClosed` | Spend called mid-sprint OR before first sprint; HTTP 409 |

**Atomicity**: each investment is a single `InvestmentApplied` event — budget debit + capacity change applied atomically in the projection handler. CSV `sprints.csv` exports `budget_remaining` + `investment_applied` columns per sprint.

See `docs/design.md` §"Investment Moves (UC40)" for the state machine, options table, and 5FS-loop wiring.

## HTTP API

The TUI and HTTP API share the same simulation state. Control the TUI's simulation programmatically via REST.

Responses are `application/json` (post-#18915 fluentfp/web migration; previously `application/hal+json`). The HAL body shape with `_links` and `_embedded` is preserved — only the wire-level Content-Type header changed.


```bash
# List simulations (find the TUI's simulation ID)
curl http://localhost:8080/simulations

# Assign a ticket
curl -X POST http://localhost:8080/simulations/{id}/assignments \
  -H "Content-Type: application/json" \
  -d '{"ticketId": "TKT-001"}'

# Start sprint
curl -X POST http://localhost:8080/simulations/{id}/sprints

# Advance one tick
curl -X POST http://localhost:8080/simulations/{id}/tick

# Run policy comparison
curl -X POST http://localhost:8080/comparisons \
  -H "Content-Type: application/json" \
  -d '{"seed": 42, "sprints": 3}'
```

The API follows HATEOAS - responses include `_links` showing available actions based on current state.

See [docs/design.md](docs/design.md) for full API documentation.

### Same-process co-tenancy (TUI + REST)

When the TUI and REST API run in the same process (default mode), the TUI auto-tick yields to REST writers per UC10's single-writer principle. **Observable state**: when a REST writer has been observed during the session, the TUI header renders a `[READ-ONLY]` badge and keeps rendering it until process restart. The badge is the load-bearing operational signal:

- **Badge visible** → auto-tick is suppressed; TUI is consuming the event stream as a projection. Safe to issue REST writes; the TUI will reflect changes.
- **Badge absent** → no REST writer has been observed in this session. TUI auto-tick still active.

**Limitations:**

- The badge-latched state is **one-way** — once `[READ-ONLY]` appears, the TUI remains read-only until process restart (deliberate per UC10's conservative read of "TUI becomes a read-only projection"). Restart the TUI process to recover the writer role.

**Complete safety contract** (closed via #18913): When the `[READ-ONLY]` badge is visible, ALL TUI mutation keypresses (`s`/start-sprint, `a`/assign, `d`/decompose, `p`/policy, `1`-`4`/invest) are gated — they record a status message + `*Attempted{Failed{Conflict}}` event but do NOT write to the engine. The badge is now a complete safety signal. SpendInvestment (`1`-`4`) uses status-message-only (no event); the other 4 actions record events in `app.uiProjection`.

**Resize artifacts**: If the TUI's rendering appears corrupted after an interactive `tmux split-window` mid-session (partial redraw, blank rows), press **Ctrl+L** to force a full redraw (UC34 workaround). The root cause is environmental (tmux + alt-screen + SIGWINCH interaction); scripted `tmux resize-window` does NOT reproduce the symptom.

## Batch Experiments

Run N independent simulations unattended from a declarative JSON config, producing per-run CSV bundles + a `runs.csv` index + an `experiment.json` provenance file suitable for downstream R/Python analysis. The `sofdevsim-batch` binary is headless (no TUI, no REST server) and intended for scripted parameter sweeps and future LLM-driven AB-comparison harnesses.

```bash
# Build
go build ./cmd/sofdevsim-batch

# Run an example experiment (3 seeds × healthy scenario × 3 sprints each)
./sofdevsim-batch -config examples/batch/minimal.json -out ./out

# Output structure:
#   out/
#     experiment.json        (config + git SHA + tool version + timestamp + schema_version)
#     runs.csv               (one row per attempted run)
#     run-0-seed-42/sofdevsim-export-<timestamp>/{metadata,tickets,sprints,incidents,metrics}.csv
#     run-1-seed-99/sofdevsim-export-<timestamp>/...
#     run-2-seed-123/sofdevsim-export-<timestamp>/...
```

See [docs/use-cases.md §UC41](docs/use-cases.md) for the behavioral contract (Cockburn shape) and [docs/design.md §Batch CLI (UC41)](docs/design.md) for the config schema, run-loop topology, per-CSV determinism contract, and per-run registry isolation rationale. Aggregation primitives (mean / stddev across runs) are deferred to a follow-up cycle; cycle 1 ships data-emission only.

## Tutorial

For a hands-on walkthrough with checkpoints, see [docs/tutorial.md](docs/tutorial.md).

## Usage

### Views

Press **Tab** to switch between views:

| View | Shows |
|------|-------|
| **Planning** | Backlog, team status, ticket assignment |
| **Execution** | Active work, fever chart, event log |
| **Metrics** | DORA dashboard, sparklines, completed tickets |
| **Comparison** | A/B policy test results |

### Keybindings

| Key | Action | Available In |
|-----|--------|--------------|
| **Tab** | Switch view | All |
| **Space** | Pause/resume simulation | Execution |
| **+/-** | Adjust simulation speed | Execution |
| **p** | Cycle sizing policy | Planning |
| **s** | Start sprint | Planning |
| **a** | Assign selected ticket to developer | Planning |
| **d** | Decompose selected ticket | Planning |
| **c** | Run policy comparison | All |
| **e** | Export data to CSV | All (after sprints complete) |
| **j/k** or **↑/↓** | Navigate backlog | Planning |
| **Ctrl+L** | Force full screen redraw (UC34 workaround) | All |
| **q** | Quit | All |

### Typical Workflow

1. Launch the simulation (starts in Planning view)
2. Press **a** to assign tickets to developers
3. Press **s** to start a sprint
4. Watch the simulation run in Execution view
5. Press **Tab** to check Metrics view
6. Press **c** to run a DORA vs TameFlow comparison

## Understanding the Results

### DORA Metrics

| Metric | Meaning | Better |
|--------|---------|--------|
| **Lead Time** | Time from start to deploy | Lower |
| **Deploy Frequency** | Deploys per day | Higher |
| **MTTR** | Mean time to recover from incidents | Lower |
| **Change Fail Rate** | Incidents per deploy | Lower |

### Fever Chart

| Color | Buffer Used | Meaning |
|-------|-------------|---------|
| **Green** | < 33% | On track |
| **Yellow** | 33-66% | At risk |
| **Red** | > 66% | Over budget |

### Comparison Mode

When you press **c**, the simulation:
1. Generates a fresh backlog with a random seed
2. Runs 3 sprints with DORA-Strict policy
3. Runs 3 sprints with TameFlow-Cognitive policy (same seed)
4. Compares the four DORA metrics
5. Declares a winner and explains why

### Data Export

Press **e** to export simulation data for external analysis. Creates a timestamped directory with CSV files:

```
sofdevsim-export-20260103-143052/
├── metadata.csv      # Seed, policy, timestamp
├── tickets.csv       # Per-ticket variance vs theoretical bounds
├── sprints.csv       # Buffer consumption, WIP (TOC)
├── incidents.csv     # Per-incident MTTR detail
├── metrics.csv       # DORA metrics summary
└── comparison.csv    # Policy comparison (if run)
```

**Sample tickets.csv row:**
```csv
ticket_id,understanding,estimated_days,actual_days,variance_ratio,expected_var_min,expected_var_max,within_expected,...
TKT-001,Medium,4.5,5.2,1.16,0.80,1.20,true,...
```

The `within_expected` column shows whether actual variance fell within theoretical bounds—making hypothesis validation as simple as `=COUNTIF(within_expected, "true")`.

**Use cases:**
- **Validate the variance model**: Check if actual variance falls within theoretical bounds (High ±5%, Medium ±20%, Low ±50%)
- **Test the sizing hypothesis**: Run multiple comparisons, export each, analyze statistically
- **Teach TOC principles**: Project the CSV in a workshop to show buffer consumption patterns

## Architecture

```
cmd/sofdevsim/main.go    # Entry point
internal/
  tui/                   # Bubbletea views
  engine/                # Simulation logic
  metrics/               # DORA calculations
  model/                 # Domain types
```

See [docs/design.md](docs/design.md) for detailed architecture and algorithms.

See [docs/use-cases.md](docs/use-cases.md) for user scenarios.

## License

MIT License

Copyright (c) 2026

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
