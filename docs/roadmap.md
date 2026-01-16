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
1. [ ] **Persistence** - Save/load simulation state for research workflows
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

- Video game evolution (deprioritized)
- Multi-team simulation
- Real CI/CD integration
- Web/GUI interface (TUI + Grafana covers visualization needs)
