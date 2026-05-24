# Roadmap

**Primary Direction:** LLM Laboratory for software development experimentation

## Phase 1: Statistical Simulation Foundation

**Goal:** Robust statistical engine for hypothesis testing

### Core Capabilities (done)
- [x] 8-phase ticket workflow
- [x] Variance model tied to understanding level
- [x] 4 sizing policies (None, DORA-Strict, TameFlow-Cognitive, Hybrid)
- [x] DORA metrics tracking
- [x] TameFlow buffer/fever tracking
- [x] Policy comparison (A/B testing)
- [x] CSV export with theoretical bounds
- [x] Seed-based reproducibility

### Remaining Work (Priority Order)
1. [x] **Persistence** - Save/load simulation state for research workflows
2. [ ] **Batch mode** - Headless runs for Monte Carlo analysis
3. [ ] **Multi-seed runs** - Automated N-seed comparison with aggregate statistics
4. [ ] **Parameter configuration** - Externalize variance bounds, phase distribution, incident rates (TOML or JSON)
5. [ ] **Statistical summary** - Mean, stddev, confidence intervals across runs

---

## Phase 2: LLM Laboratory Infrastructure

**Goal:** Platform for running controlled LLM experiments

### Capabilities Needed
- [ ] **Programmatic API** - Library interface, not just TUI
- [ ] **Experiment definition** - Declarative config for experiment parameters
- [ ] **Results aggregation** - Combine results across experiment runs
- [ ] **Calibration workflow** - Import observed data, fit parameters, export refined model

### Integration Points
- [ ] CLI for scripted experiments
- [ ] JSON/YAML experiment configs
- [ ] Output formats for R/Python analysis

---

## Phase 3: LLM Team Validation

**Goal:** Use LLM "team" to validate simulation assumptions

### Capabilities Needed
- [ ] **Role definitions** - Developer, reviewer, PM perspectives
- [ ] **Work session protocol** - Structured approach to LLM working tickets
- [ ] **Measurement capture** - Extract phase timing, variance sources, decision points
- [ ] **Comparison tooling** - Predicted vs actual analysis

### Research Questions
- Does phase distribution match real work?
- What actually causes variance?
- Does decomposition improve understanding in practice?

---

## Phase 4: Real Team Calibration (Stretch)

**Goal:** Calibrate to actual human team data

### Data Sources
- JIRA ticket history
- Git commit/PR data
- Incident tracking

### Capabilities Needed
- [ ] **Data import** - JIRA/git ETL pipeline
- [ ] **Parameter fitting** - Derive simulation parameters from historical data
- [ ] **Validation metrics** - How well does calibrated model predict?

---

## Phase 5: Factorio-Inspired Dynamics Program

**Goal:** Convert sofdevsim from a TOC *observatory* into a TOC *playable system* by adding production-modeling dynamics inspired by Factorio's recipe/buffer/capacity-invest game loop. Each enhancement makes a specific Five Focusing Step (UC22) tangibly actionable rather than purely lesson-based.

**Scope discipline:** dynamics — recipes (typed ticket flows), buffers (per-phase WIP caps), drum-buffer-rope (demand-driven release), capacity investment (between-sprint moves). **Spatial layout, belt routing, multi-team handoff topology, and other game-mechanics richness are explicitly out of scope** — see Not Planned below.

### Parent Epic
- Task **#15441** (`factorio-dynamics-epic`) — organizing cycle; ships docs + child task IDs only.

### Children (default sequence; reorderable per child's 1a evidence via `/epic-reorder`)

1. **#15442 — UC37 Heterogeneous Ticket Types** (`factorio-c1-types`) — `TicketType` enum + per-type phase distributions + mix-profile backlog generator. *Sequencing rationale:* nothing else has bite if backlog is uniform; types make the constraint move with the mix.
2. **#15443 — UC38 Per-Phase WIP Caps** (`factorio-c2-caps`) — explicit `PhaseWIPConfig` + wires the long-declared `CICDSlots`; reuses the `RopeConfig`/`DownstreamWIP()` pattern. *Sequencing rationale:* caps only pedagogically interesting under differentiated load (UC37).
3. **#15445 — UC39 Demand-Driven Release** (`factorio-c3-demand`) — pull mode gated by constraint-buffer penetration; rope becomes end-to-end visible. *Sequencing rationale:* the rope needs anchors (UC38's per-phase caps).
4. **#15446 — UC40 Investment Moves at Sprint Boundary** (`factorio-c4-invest`) — finite budget the operator spends on capacity changes targeted at the identified constraint; closes the 5FS EXPLOIT/ELEVATE loop. *Sequencing rationale:* investment is meaningful only when the substrate is legible (UC37 + UC38 + UC39).

Each child is its own `/begin <task-id>` Tandem cycle with full plan/contract/attestation/commit chain. Parent epic ships no production code; pedagogical UC22 forward-references are wired in the parent commit so each child's docs-first sequence already has a target to satisfy.

---

## Decisions Made

| Question | Decision |
|----------|----------|
| TUI future | Keep for interactive exploration |
| Visualization | TUI sparklines + Grafana for deeper analysis |
| Config format | TOML or JSON (no YAML) |
| Phase 1 priority | Persistence first, then batch mode |

## Open Questions

1. **Collaboration** - Single-user research tool or multi-user platform?
2. **Documentation** - Academic paper potential? Teaching materials?

---

## Not Planned

- Video game evolution as game-mechanics richness (deprioritized). Note: Factorio-inspired **dynamics** (queueing/TOC mechanics — recipes, buffers, drum-buffer-rope, capacity investment) are IN scope as Phase 5 above; the rejection here is specifically of **spatial layout, belt routing, topology mutation (alternate phase paths), multi-team handoff geometry, and per-developer-desk modeling**. The office animation provides enough texture; further spatial complexity dilutes TOC pedagogy.
- Multi-team simulation
- Real CI/CD integration
- Web/GUI interface (TUI + Grafana covers visualization needs)
