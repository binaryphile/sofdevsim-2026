package engine_test

import (
	"testing"
	"time"

	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// FeverSnapshot mirrors metrics.FeverSnapshot for benchmarking.
type FeverSnapshot struct {
	Day         int
	PercentUsed float64
	Status      model.FeverStatus
}

// GetPercentUsed returns the percent used (for method expression).
func (f FeverSnapshot) GetPercentUsed() float64 { return f.PercentUsed }

// generateFeverSnapshots creates test data for fever benchmarks.
func generateFeverSnapshots(n int) []FeverSnapshot {
	snapshots := make([]FeverSnapshot, n)
	for i := 0; i < n; i++ {
		snapshots[i] = FeverSnapshot{
			Day:         i,
			PercentUsed: float64(i%100) / 100.0,
			Status:      model.FeverStatus(i % 3),
		}
	}
	return snapshots
}

// generateTickets creates test tickets for benchmarks.
func generateTickets(n int) []model.Ticket {
	tickets := make([]model.Ticket, n)
	for i := 0; i < n; i++ {
		tickets[i] = model.NewTicket(
			idFor("TKT", i),
			"Benchmark ticket",
			float64(3+(i%5)),
			model.UnderstandingLevel(i%3),
		)
		tickets[i].CompletedTick = i % 100
	}
	return tickets
}

// generateDurations creates test durations for fold benchmarks.
func generateDurations(n int) []time.Duration {
	durations := make([]time.Duration, n)
	for i := 0; i < n; i++ {
		durations[i] = time.Duration(i) * time.Hour
	}
	return durations
}

// =============================================================================
// Pattern 1: ToFloat64 - Field extraction
// =============================================================================

// BenchmarkFluentFP_ToFloat64 measures slice.From().ToFloat64() performance.
func BenchmarkFluentFP_ToFloat64(b *testing.B) {
	snapshots := generateFeverSnapshots(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = slice.From(snapshots).ToFloat64(FeverSnapshot.GetPercentUsed)
	}
}

// BenchmarkLoop_ToFloat64 measures equivalent loop performance.
func BenchmarkLoop_ToFloat64(b *testing.B) {
	snapshots := generateFeverSnapshots(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := make([]float64, 0, len(snapshots))
		for _, s := range snapshots {
			result = append(result, s.PercentUsed)
		}
		_ = result
	}
}

// =============================================================================
// Pattern 2: KeepIf + Len - Filter and count
// =============================================================================

// BenchmarkFluentFP_KeepIfLen measures slice.From().KeepIf().Len() performance.
func BenchmarkFluentFP_KeepIfLen(b *testing.B) {
	tickets := generateTickets(100)
	cutoff := 50

	// completedAfterCutoff returns true if ticket was completed after cutoff.
	completedAfterCutoff := func(t model.Ticket) bool { return t.CompletedTick >= cutoff }

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = slice.From(tickets).KeepIf(completedAfterCutoff).Len()
	}
}

// BenchmarkLoop_FilterCount measures equivalent loop performance.
func BenchmarkLoop_FilterCount(b *testing.B) {
	tickets := generateTickets(100)
	cutoff := 50

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count := 0
		for _, t := range tickets {
			if t.CompletedTick >= cutoff {
				count++
			}
		}
		_ = count
	}
}

// =============================================================================
// Pattern 3: Fold - Reduction/accumulation
// =============================================================================

// sumDuration adds two durations.
var sumDuration = func(acc, d time.Duration) time.Duration { return acc + d }

// BenchmarkFluentFP_Fold measures slice.Fold() performance.
func BenchmarkFluentFP_Fold(b *testing.B) {
	durations := generateDurations(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = slice.Fold(durations, time.Duration(0), sumDuration)
	}
}

// BenchmarkLoop_Accumulate measures equivalent loop performance.
func BenchmarkLoop_Accumulate(b *testing.B) {
	durations := generateDurations(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total := time.Duration(0)
		for _, d := range durations {
			total += d
		}
		_ = total
	}
}

// =============================================================================
// Pattern 4: Multi-field extraction (simulated Unzip4)
// =============================================================================

// DORASnapshot mirrors metrics.DORASnapshot for benchmarking.
type DORASnapshot struct {
	Day             int
	LeadTimeAvg     float64
	DeployFrequency float64
	MTTR            float64
	ChangeFailRate  float64
}

// Accessors for FluentFP method expression syntax.
func (s DORASnapshot) GetLeadTimeAvg() float64     { return s.LeadTimeAvg }
func (s DORASnapshot) GetDeployFrequency() float64 { return s.DeployFrequency }
func (s DORASnapshot) GetMTTR() float64            { return s.MTTR }
func (s DORASnapshot) GetChangeFailRate() float64  { return s.ChangeFailRate }

// generateDORASnapshots creates test data for multi-field benchmarks.
func generateDORASnapshots(n int) []DORASnapshot {
	snapshots := make([]DORASnapshot, n)
	for i := 0; i < n; i++ {
		snapshots[i] = DORASnapshot{
			Day:             i,
			LeadTimeAvg:     float64(i) * 0.1,
			DeployFrequency: float64(i) * 0.01,
			MTTR:            float64(i) * 0.05,
			ChangeFailRate:  float64(i) * 0.001,
		}
	}
	return snapshots
}

// BenchmarkFluentFP_Unzip4 measures slice.Unzip4() performance.
func BenchmarkFluentFP_Unzip4(b *testing.B) {
	snapshots := generateDORASnapshots(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = slice.Unzip4(snapshots,
			DORASnapshot.GetLeadTimeAvg,
			DORASnapshot.GetDeployFrequency,
			DORASnapshot.GetMTTR,
			DORASnapshot.GetChangeFailRate,
		)
	}
}

// BenchmarkLoop_FourPass measures equivalent 4-loop approach.
func BenchmarkLoop_FourPass(b *testing.B) {
	snapshots := generateDORASnapshots(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		leadTimes := make([]float64, len(snapshots))
		deployFreqs := make([]float64, len(snapshots))
		mttrs := make([]float64, len(snapshots))
		cfrs := make([]float64, len(snapshots))

		for j, s := range snapshots {
			leadTimes[j] = s.LeadTimeAvg
		}
		for j, s := range snapshots {
			deployFreqs[j] = s.DeployFrequency
		}
		for j, s := range snapshots {
			mttrs[j] = s.MTTR
		}
		for j, s := range snapshots {
			cfrs[j] = s.ChangeFailRate
		}

		_, _, _, _ = leadTimes, deployFreqs, mttrs, cfrs
	}
}

// BenchmarkLoop_SinglePass measures optimized single-pass loop approach.
// This is the fair comparison - a well-written loop does it in one pass.
func BenchmarkLoop_SinglePass(b *testing.B) {
	snapshots := generateDORASnapshots(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		leadTimes := make([]float64, len(snapshots))
		deployFreqs := make([]float64, len(snapshots))
		mttrs := make([]float64, len(snapshots))
		cfrs := make([]float64, len(snapshots))

		for j, s := range snapshots {
			leadTimes[j] = s.LeadTimeAvg
			deployFreqs[j] = s.DeployFrequency
			mttrs[j] = s.MTTR
			cfrs[j] = s.ChangeFailRate
		}

		_, _, _, _ = leadTimes, deployFreqs, mttrs, cfrs
	}
}
