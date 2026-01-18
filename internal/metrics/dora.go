package metrics

import (
	"sort"
	"time"

	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/fluentfp/ternary"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// sumDuration adds two time.Duration values for use with slice.Fold.
var sumDuration = func(acc, d time.Duration) time.Duration { return acc + d }

// DORAMetrics tracks the four key DORA metrics
type DORAMetrics struct {
	// Lead Time: time from first commit to deploy
	LeadTimes   []time.Duration
	LeadTimeAvg time.Duration
	LeadTimeP50 time.Duration
	LeadTimeP95 time.Duration

	// Deploy Frequency: deploys per day (rolling 7-day window)
	DeploysLast7Days int
	DeployFrequency  float64

	// MTTR: mean time to restore (from incident open to resolved)
	MTTRs   []time.Duration
	MTTRAvg time.Duration

	// Change Fail Rate: incidents / deploys
	TotalDeploys   int
	TotalIncidents int
	ChangeFailRate float64

	// History for sparklines
	History []DORASnapshot
}

// DORASnapshot captures metrics at a point in time
type DORASnapshot struct {
	Day             int
	LeadTimeAvg     float64 // in days
	DeployFrequency float64
	MTTR            float64 // in days
	ChangeFailRate  float64 // percentage
}

// NewDORAMetrics creates an initialized metrics tracker
func NewDORAMetrics() DORAMetrics {
	return DORAMetrics{
		LeadTimes: make([]time.Duration, 0),
		MTTRs:     make([]time.Duration, 0),
		History:   make([]DORASnapshot, 0),
	}
}

// Updated recalculates all DORA metrics from simulation state and returns the updated value
func (m DORAMetrics) Updated(sim *model.Simulation) DORAMetrics {
	m = m.updatedLeadTime(sim)
	m = m.updatedDeployFrequency(sim)
	m = m.updatedMTTR(sim)
	m = m.updatedChangeFailRate(sim)
	m = m.withSnapshot(sim.CurrentTick)
	return m
}

func (m DORAMetrics) updatedLeadTime(sim *model.Simulation) DORAMetrics {
	m.LeadTimes = m.LeadTimes[:0]

	for _, ticket := range sim.CompletedTickets {
		// Use tick-based calculation (1 tick = 1 day in simulation)
		// Wall-clock time (StartedAt/CompletedAt) is unreliable in fast simulations
		// Check CompletedTick > StartedTick to allow tickets starting at tick 0
		if ticket.CompletedTick > ticket.StartedTick {
			leadTimeDays := ticket.CompletedTick - ticket.StartedTick
			m.LeadTimes = append(m.LeadTimes, time.Duration(leadTimeDays)*24*time.Hour)
		}
	}

	if len(m.LeadTimes) == 0 {
		m.LeadTimeAvg = 0
		m.LeadTimeP50 = 0
		m.LeadTimeP95 = 0
		return m
	}

	// Sort for percentiles
	sorted := make([]time.Duration, len(m.LeadTimes))
	copy(sorted, m.LeadTimes)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	// Calculate average
	total := slice.Fold(m.LeadTimes, time.Duration(0), sumDuration)
	m.LeadTimeAvg = total / time.Duration(len(m.LeadTimes))

	// Calculate percentiles
	m.LeadTimeP50 = percentile(sorted, 0.50)
	m.LeadTimeP95 = percentile(sorted, 0.95)
	return m
}

func (m DORAMetrics) updatedDeployFrequency(sim *model.Simulation) DORAMetrics {
	// Count deploys in last 7 days (using simulation ticks)
	cutoff := sim.CurrentTick - 7
	// completedAfterCutoff returns true if ticket was completed after the cutoff tick.
	completedAfterCutoff := func(t model.Ticket) bool { return t.CompletedTick >= cutoff }
	m.DeploysLast7Days = slice.From(sim.CompletedTickets).
		KeepIf(completedAfterCutoff).
		Len()

	// Frequency per day
	if sim.CurrentTick > 0 {
		days := ternary.If[int](sim.CurrentTick < 7).Then(sim.CurrentTick).Else(7)
		m.DeployFrequency = float64(m.DeploysLast7Days) / float64(days)
	}
	return m
}

func (m DORAMetrics) updatedMTTR(sim *model.Simulation) DORAMetrics {
	m.MTTRs = m.MTTRs[:0]

	for _, inc := range sim.ResolvedIncidents {
		if inc.ResolvedAt != nil {
			mttr := inc.ResolvedAt.Sub(inc.CreatedAt)
			m.MTTRs = append(m.MTTRs, mttr)
		}
	}

	if len(m.MTTRs) == 0 {
		m.MTTRAvg = 0
		return m
	}

	total := slice.Fold(m.MTTRs, time.Duration(0), sumDuration)
	m.MTTRAvg = total / time.Duration(len(m.MTTRs))
	return m
}

func (m DORAMetrics) updatedChangeFailRate(sim *model.Simulation) DORAMetrics {
	m.TotalDeploys = len(sim.CompletedTickets)
	m.TotalIncidents = len(sim.OpenIncidents) + len(sim.ResolvedIncidents)

	if m.TotalDeploys > 0 {
		m.ChangeFailRate = float64(m.TotalIncidents) / float64(m.TotalDeploys)
	} else {
		m.ChangeFailRate = 0
	}
	return m
}

func (m DORAMetrics) withSnapshot(day int) DORAMetrics {
	m.History = append(m.History, DORASnapshot{
		Day:             day,
		LeadTimeAvg:     m.LeadTimeAvg.Hours() / 24,
		DeployFrequency: m.DeployFrequency,
		MTTR:            m.MTTRAvg.Hours() / 24,
		ChangeFailRate:  m.ChangeFailRate * 100,
	})
	return m
}

// LeadTimeAvgDays returns lead time in days
func (m DORAMetrics) LeadTimeAvgDays() float64 {
	return m.LeadTimeAvg.Hours() / 24
}

// MTTRAvgDays returns MTTR in days
func (m DORAMetrics) MTTRAvgDays() float64 {
	return m.MTTRAvg.Hours() / 24
}

// ChangeFailRatePct returns CFR as percentage
func (m DORAMetrics) ChangeFailRatePct() float64 {
	return m.ChangeFailRate * 100
}

// percentile calculates the pth percentile of a sorted slice
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

