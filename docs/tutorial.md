# Software Development Simulation Tutorial

A hands-on walkthrough of the simulation's core features with verifiable checkpoints.

**Prerequisites:**
- Go 1.21+ installed (or Nix)
- Terminal with at least 80x24 characters

**Time:** ~50 minutes

## Section 1: Orientation (5 min)

### Launch the Simulation

Start with a fixed seed for reproducible results:

```bash
go run cmd/sofdevsim/main.go --seed 42
```

### Explore the Views

Press **Tab** to cycle through the four views:

1. **Planning** - Backlog and team management
2. **Execution** - Active sprint progress
3. **Metrics** - DORA dashboard
4. **Comparison** - Policy A/B testing

### CHECKPOINT 1: Initial State

Verify these exact values in the header and Planning view:

| Item | Expected Value |
|------|----------------|
| Seed | 42 |
| Day | 0 |
| Policy | DORA-Strict |
| Backlog count | 12 tickets |
| Developers | 3 (all idle) |

**Team:**
- Alice (velocity: 1.0) [idle]
- Bob (velocity: 0.8) [idle]
- Carol (velocity: 1.2) [idle]

**First 5 tickets (same order every run):**
1. TKT-001 - 6.5d Est, Low understanding
2. TKT-002 - 5.6d Est, Low understanding
3. TKT-003 - 2.2d Est, Medium understanding
4. TKT-004 - 2.0d Est, High understanding
5. TKT-005 - 5.6d Est, Low understanding

If your values differ, check that you launched with `--seed 42`.

## Section 2: Sprint Planning (5 min)

### Navigate and Assign Tickets

In the Planning view:

1. Use **j/k** (or arrow keys) to navigate the backlog
2. Press **a** to assign the selected ticket to the first idle developer
3. Repeat until all 3 developers have tickets

### CHECKPOINT 2: After Assignment (Exact Values)

| Item | Expected Value |
|------|----------------|
| Day | 0 |
| Alice | [busy] → TKT-001 |
| Bob | [busy] → TKT-002 |
| Carol | [busy] → TKT-003 |
| Backlog remaining | 9 tickets |
| Policy | DORA-Strict |

### Understanding Policies

Press **p** to cycle through sizing policies:
- **DORA-Strict** - Decompose tickets > 5 days
- **TameFlow-Cognitive** - Decompose Low-understanding tickets
- **Hybrid** - Both rules
- **None** - No automatic decomposition

Return to **DORA-Strict** for this tutorial.

## Section 3: Running a Sprint (10 min)

### Start the Sprint

Press **s** to start the sprint. The view switches to **Execution**.

### What You'll See

- **Sprint progress bar** - Days elapsed / total
- **Active work** - Each developer's current ticket and phase
- **Fever chart** - Buffer consumption (Green/Yellow/Red)
- **Events log** - Phase transitions, incidents, completions

### Speed Controls

- Press **+** to speed up (shorter tick interval)
- Press **-** to slow down (longer tick interval)
- Press **Space** to pause/resume

### CHECKPOINT 3: Mid-Sprint (Day 5) - Exact Values

Pause the simulation at Day 5 using **Space**. Verify:

| Item | Expected Value |
|------|----------------|
| Day | 5 |
| Sprint progress | 50% (Day 5 of 10) |
| Backlog | 9 tickets |
| Completed | 0 tickets |
| Fever | 23% buffer used (Green) |

**Active Work (exact):**
| Developer | Ticket | Progress | Phase |
|-----------|--------|----------|-------|
| Alice | TKT-001 | 55% | Implement |
| Bob | TKT-002 | 52% | Implement |
| Carol | TKT-003 | 95% | CI/CD |

Carol's ticket (TKT-003) is nearly complete and will finish before sprint end.

## Section 4: Understanding Measurements (5 min)

### View DORA Metrics

Press **Tab** until you reach the **Metrics** view.

### The Four DORA Metrics

| Metric | Meaning | Better Direction |
|--------|---------|------------------|
| **Lead Time** | Days from start to deploy | Lower |
| **Deploy Frequency** | Deploys per day | Higher |
| **MTTR** | Mean time to recover from incidents | Lower |
| **Change Fail Rate** | % of deploys causing incidents | Lower |

### Sparklines

Each metric shows a sparkline (mini trend chart). Early in the simulation, data is limited. After 2-3 sprints, trends become meaningful.

### CHECKPOINT 4: Metrics Understanding

Verify you can identify:
- [ ] All four DORA metrics are displayed
- [ ] Each shows a numeric value
- [ ] Each shows a sparkline (may be sparse early on)
- [ ] Direction indicators show which way is "better"

## Section 5: Sprint Completion (5 min)

### Let the Sprint Finish

Press **Space** to resume, then wait for Day 10 (sprint end).

### CHECKPOINT 5: Sprint Complete (Day 10) - Exact Values

Switch to **Metrics** view and verify:

| Item | Expected Value |
|------|----------------|
| Day | 10 |
| Backlog | 9 tickets |
| Completed | 1 ticket |
| Alice | [busy] → TKT-001 (still in progress) |
| Bob | [busy] → TKT-002 (still in progress) |
| Carol | [idle] |

**Completed Ticket:**
| ID | Est | Actual | Ratio | Understanding |
|----|-----|--------|-------|---------------|
| TKT-003 | 2.2d | 9.4d | 4.27 | Medium |

**DORA Metrics:**
| Metric | Value |
|--------|-------|
| Deploy Frequency | 0.14/day |

**Note on TKT-003:** The 4.27x variance ratio exceeds the expected Medium bounds (0.80-1.20). This can happen due to incidents or phase delays. The simulation models real-world unpredictability where even "understood" work sometimes surprises us.

## Section 6: Decomposition (5 min)

### Find a Decomposable Ticket

Press **Tab** to return to **Planning** view.

Look for a ticket with:
- Low understanding, OR
- Estimate > 5 days (under DORA-Strict policy)

### Decompose the Ticket

1. Navigate to select the ticket with **j/k**
2. Press **d** to decompose

### CHECKPOINT 6: Decomposition Result

After decomposition:
- Parent ticket disappears from backlog
- 2-4 child tickets appear (smaller, better understood)
- Child tickets have:
  - Higher understanding level than parent
  - Smaller estimates
  - IDs like "TKT-001-1", "TKT-001-2", etc.

**Why this matters:** Decomposition by understanding (TameFlow) often beats decomposition by size (DORA) because variance depends on understanding, not duration.

## Section 7: Variance Understanding (5 min)

### The Core Model

The simulation's key insight: **understanding level determines predictability**.

| Understanding | Variance Range | Example |
|---------------|----------------|---------|
| High | ±5% | 4.0d estimate → 3.8-4.2d actual |
| Medium | ±20% | 4.0d estimate → 3.2-4.8d actual |
| Low | ±50% | 4.0d estimate → 2.0-6.0d actual |

### CHECKPOINT 7: Variance Validation

Review completed tickets in the Metrics view. For each ticket:

1. Calculate: `variance_ratio = Actual / Estimated`
2. Check against expected bounds:
   - **High**: ratio between 0.95-1.05
   - **Medium**: ratio between 0.80-1.20
   - **Low**: ratio between 0.50-1.50

Most tickets should fall within their expected bounds. Outliers indicate incident impact or random edge cases.

### Why This Matters

A 3-day ticket with Low understanding (variance: 1.5-4.5 days) is riskier than a 6-day ticket with High understanding (variance: 5.7-6.3 days).

DORA-Strict would decompose the 6-day ticket but ignore the risky 3-day one. TameFlow-Cognitive catches both risks.

## Section 8: Multi-Sprint Analysis (10 min)

### Run Additional Sprints

1. Return to **Planning** view
2. Assign available tickets to idle developers
3. Press **s** to start the next sprint
4. Repeat for a total of 3 sprints

### CHECKPOINT 8: Trend Analysis

After 3 sprints, check the Metrics view:

| Item | What to Look For |
|------|------------------|
| Sparklines | Show trends over time |
| Lead time | Should stabilize or improve |
| Deploy frequency | Should be consistent |
| MTTR | Depends on incident occurrence |
| Change fail rate | Varies with incident rate |

### Run a Policy Comparison

Press **c** to run an automated comparison:
- Creates identical backlog (same seed)
- Runs 3 sprints with DORA-Strict
- Runs 3 sprints with TameFlow-Cognitive
- Reports which policy won on each DORA metric

### CHECKPOINT 8b: Comparison Results

The Comparison view shows:
- [ ] Seed used (for reproducibility)
- [ ] Results for each DORA metric
- [ ] Winner for each metric
- [ ] Overall winner and margin
- [ ] Brief explanation of results

## Summary

You've learned:

1. **Views** - Planning, Execution, Metrics, Comparison
2. **Controls** - Assign (a), Decompose (d), Policy (p), Start (s)
3. **DORA metrics** - Lead Time, Deploy Freq, MTTR, CFR
4. **Fever chart** - Buffer consumption tracking
5. **Variance model** - Understanding determines predictability
6. **Policy comparison** - DORA-Strict vs TameFlow-Cognitive

## Next Steps

- **Export data**: Press **e** to export CSV files for external analysis
- **Try different seeds**: Each seed produces different initial conditions
- **Experiment with policies**: Which policy wins most often?
- **Read the design doc**: See [design.md](design.md) for implementation details

## Quick Reference

| Key | Action |
|-----|--------|
| Tab | Switch view |
| Space | Pause/resume |
| +/- | Speed up/slow down |
| j/k | Navigate backlog |
| a | Assign ticket |
| d | Decompose ticket |
| p | Cycle policy |
| s | Start sprint |
| c | Run comparison |
| e | Export data |
| q | Quit |
