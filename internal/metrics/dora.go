package metrics

import (
	"sort"
	"time"

	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/fluentfp/ternary"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

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
func NewDORAMetrics() *DORAMetrics {
	return &DORAMetrics{
		LeadTimes: make([]time.Duration, 0),
		MTTRs:     make([]time.Duration, 0),
		History:   make([]DORASnapshot, 0),
	}
}

// Update recalculates all DORA metrics from simulation state
func (m *DORAMetrics) Update(sim *model.Simulation) {
	m.updateLeadTime(sim)
	m.updateDeployFrequency(sim)
	m.updateMTTR(sim)
	m.updateChangeFailRate(sim)
	m.appendSnapshot(sim.CurrentTick)
}

func (m *DORAMetrics) updateLeadTime(sim *model.Simulation) {
	m.LeadTimes = m.LeadTimes[:0]

	for _, ticket := range sim.CompletedTickets {
		if !ticket.StartedAt.IsZero() && !ticket.CompletedAt.IsZero() {
			leadTime := ticket.CompletedAt.Sub(ticket.StartedAt)
			m.LeadTimes = append(m.LeadTimes, leadTime)
		}
	}

	if len(m.LeadTimes) == 0 {
		m.LeadTimeAvg = 0
		m.LeadTimeP50 = 0
		m.LeadTimeP95 = 0
		return
	}

	// Sort for percentiles
	sorted := make([]time.Duration, len(m.LeadTimes))
	copy(sorted, m.LeadTimes)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	// Calculate average
	var total time.Duration
	for _, lt := range m.LeadTimes {
		total += lt
	}
	m.LeadTimeAvg = total / time.Duration(len(m.LeadTimes))

	// Calculate percentiles
	m.LeadTimeP50 = percentile(sorted, 0.50)
	m.LeadTimeP95 = percentile(sorted, 0.95)
}

func (m *DORAMetrics) updateDeployFrequency(sim *model.Simulation) {
	// Count deploys in last 7 days (using simulation ticks)
	cutoff := sim.CurrentTick - 7
	m.DeploysLast7Days = slice.From(sim.CompletedTickets).KeepIf(func(t model.Ticket) bool {
		return t.CompletedTick >= cutoff
	}).Len()

	// Frequency per day
	if sim.CurrentTick > 0 {
		days := ternary.If[int](sim.CurrentTick < 7).Then(sim.CurrentTick).Else(7)
		m.DeployFrequency = float64(m.DeploysLast7Days) / float64(days)
	}
}

func (m *DORAMetrics) updateMTTR(sim *model.Simulation) {
	m.MTTRs = m.MTTRs[:0]

	for _, inc := range sim.ResolvedIncidents {
		if inc.ResolvedAt != nil {
			mttr := inc.ResolvedAt.Sub(inc.CreatedAt)
			m.MTTRs = append(m.MTTRs, mttr)
		}
	}

	if len(m.MTTRs) == 0 {
		m.MTTRAvg = 0
		return
	}

	var total time.Duration
	for _, mttr := range m.MTTRs {
		total += mttr
	}
	m.MTTRAvg = total / time.Duration(len(m.MTTRs))
}

func (m *DORAMetrics) updateChangeFailRate(sim *model.Simulation) {
	m.TotalDeploys = len(sim.CompletedTickets)
	m.TotalIncidents = len(sim.OpenIncidents) + len(sim.ResolvedIncidents)

	if m.TotalDeploys > 0 {
		m.ChangeFailRate = float64(m.TotalIncidents) / float64(m.TotalDeploys)
	} else {
		m.ChangeFailRate = 0
	}
}

func (m *DORAMetrics) appendSnapshot(day int) {
	m.History = append(m.History, DORASnapshot{
		Day:             day,
		LeadTimeAvg:     m.LeadTimeAvg.Hours() / 24,
		DeployFrequency: m.DeployFrequency,
		MTTR:            m.MTTRAvg.Hours() / 24,
		ChangeFailRate:  m.ChangeFailRate * 100,
	})
}

// LeadTimeAvgDays returns lead time in days
func (m *DORAMetrics) LeadTimeAvgDays() float64 {
	return m.LeadTimeAvg.Hours() / 24
}

// MTTRAvgDays returns MTTR in days
func (m *DORAMetrics) MTTRAvgDays() float64 {
	return m.MTTRAvg.Hours() / 24
}

// ChangeFailRatePct returns CFR as percentage
func (m *DORAMetrics) ChangeFailRatePct() float64 {
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

