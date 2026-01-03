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
| **Green** | < 50% | On track |
| **Yellow** | 50-80% | At risk |
| **Red** | > 80% | Over budget |

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
