# Future Use Case: LLM Team Validation

**Status:** Future consideration (after statistical simulation phase)

## Concept

Create an LLM-based "team" that works on real development problems in various roles (PM, developer, QA, reviewer). Use their observed behavior to:

1. Validate simulation predictions against "ground truth"
2. Calibrate simulation parameters empirically
3. Discover real-world complications the model doesn't capture

## The Calibration Loop

```
Real Problem → LLM Team Works It → Observe Actuals → Compare to Simulation → Refine Model
```

## What LLM Team Could Reveal

| Simulation Assumption | Validation Question |
|----------------------|---------------------|
| Phase effort distribution (55% implement) | Does real work follow this split? |
| Understanding improves 60% on decomposition | Does research/spiking actually help? |
| Low understanding = 0.5-1.5x variance | What causes variance in practice? |
| Incidents correlate with understanding | Do LLM "bugs" follow the model? |

## Open Questions

- **Role granularity:** Full team (PM, dev, QA, reviewer) or developer-only?
- **Problem source:** This repo's issues? External projects? Synthetic specs?
- **Measurement:** How to capture "actual" phase timing from LLM work sessions?
- **Reproducibility:** Same problem + same seed = same LLM behavior?

## Why This Matters

Human teams can't provide controlled experiments - too many variables. LLM teams offer:

- Reproducibility (same prompt/context = comparable behavior)
- Observability (full transcript of reasoning/decisions)
- Speed (compress weeks of work into hours)
- Control (vary one factor at a time)

## Prerequisites

- [ ] Statistical simulation complete and stable
- [ ] Export format supports parameter calibration workflow
- [ ] Clear measurement protocol for LLM work sessions

---

# Stretch Goal: Real Team Calibration

**Status:** Stretch goal (after LLM team validation)

## Concept

Calibrate simulation parameters against actual human team data from JIRA and git history. The simulation controls experimental parameters while historical data provides ground truth.

## Data Sources Available

- **JIRA:** Ticket lifecycle, estimates vs actuals, decomposition patterns
- **Git:** Commit frequency, PR cycle times, incident correlation

## Potential Calibrations

| Parameter | Historical Signal |
|-----------|-------------------|
| Phase distribution | Time between status transitions |
| Variance by understanding | Estimate accuracy by ticket type/complexity |
| Decomposition benefit | Child ticket performance vs parent estimates |
| Incident rate | Bug tickets linked to feature tickets |

## Challenges

- Historical data is noisy (meetings, PTO, context switching)
- Understanding level not explicitly tracked in JIRA
- Confounding variables harder to isolate than with LLM team

## Value

If achievable: simulation becomes predictive tool for *your specific team*, not just general theory validation.

---

## Related

- Primary path decision: LLM laboratory (not video game)
- Persistence feature needed for research workflows
