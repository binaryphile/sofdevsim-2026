package batch

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
)

// MetricStats captures the cross-run statistics for one metric. Mean
// and Stddev use population formulas (÷N, not ÷N-1) per /grade R1 F1:
// aggregate.csv is descriptive statistics across the closed batch, not
// inference about a sampled population.
type MetricStats struct {
	Metric string
	Mean   float64
	Stddev float64
	Min    float64
	Max    float64
	N      int
}

// metricKeys is the stable iteration order for aggregate.csv data rows.
// Cycle 1's metrics.csv had 7 numeric columns; lead_time_stddev is
// EXCLUDED here because writers.go:218 hardcodes it to "0.00" with
// the comment "stddev - not tracked" — meta-aggregating an all-zero
// column adds zero information AND misleads readers. See design.md.
var metricKeys = []string{
	"lead_time_avg",
	"deploy_frequency",
	"mttr_avg",
	"change_fail_rate",
	"total_tickets",
	"total_incidents",
}

// aggregateCSVHeader is the row-0 header for aggregate.csv.
var aggregateCSVHeader = []string{"metric", "mean", "stddev", "min", "max", "n"}

// AggregateMetrics computes per-metric statistics across the succeeded
// subset of runs. Failed runs are skipped per Q4-i; the resulting N
// column documents the actual count aggregated.
//
// n=0 semantics (all runs failed OR runs empty): returns nil. Downstream
// WriteAggregateCSV(nil) then writes header-only file naturally (no
// special-case logic in the writer — composition over conditionals per
// /i pass-fresh-2).
//
// n=1 semantics: returns 6 entries each with Stddev=0 (single value
// has zero variance) per Q6.
//
// n>=2 semantics: Mean/Stddev/Min/Max computed per-metric across the
// succeeded subset using population stddev formula ÷N.
func AggregateMetrics(runs []RunResult) []MetricStats {
	succeeded := make([]RunResult, 0, len(runs))
	for _, r := range runs {
		if r.Status == "succeeded" {
			succeeded = append(succeeded, r)
		}
	}
	if len(succeeded) == 0 {
		return nil
	}
	stats := make([]MetricStats, 0, len(metricKeys))
	for _, key := range metricKeys {
		values := make([]float64, 0, len(succeeded))
		for _, r := range succeeded {
			if v, ok := r.Metrics[key]; ok {
				values = append(values, v)
			}
		}
		stats = append(stats, MetricStats{
			Metric: key,
			Mean:   mean(values),
			Stddev: populationStddev(values),
			Min:    minFloat(values),
			Max:    maxFloat(values),
			N:      len(values),
		})
	}
	return stats
}

// WriteAggregateCSV writes outDir/aggregate.csv with the header row + one
// data row per MetricStats entry. Header always emitted (even on empty
// stats) so consumer scripts get a parseable shape regardless of N.
//
// Floats formatted via fmt.Sprintf("%.2f", v) per /grade R1 F2 matching
// internal/export/writers.go:217 precedent (locks deterministic golden
// tests). N formatted via strconv.Itoa.
func WriteAggregateCSV(outDir string, stats []MetricStats) error {
	path := filepath.Join(outDir, "aggregate.csv")
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create aggregate.csv: %w", err)
	}
	defer file.Close()

	w := csv.NewWriter(file)
	defer w.Flush()

	if err := w.Write(aggregateCSVHeader); err != nil {
		return fmt.Errorf("aggregate.csv header: %w", err)
	}
	for _, s := range stats {
		row := []string{
			s.Metric,
			fmt.Sprintf("%.2f", s.Mean),
			fmt.Sprintf("%.2f", s.Stddev),
			fmt.Sprintf("%.2f", s.Min),
			fmt.Sprintf("%.2f", s.Max),
			strconv.Itoa(s.N),
		}
		if err := w.Write(row); err != nil {
			return fmt.Errorf("aggregate.csv row %q: %w", s.Metric, err)
		}
	}
	return nil
}

// mean returns the arithmetic mean of values; 0 for empty input.
func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// populationStddev returns the population standard deviation ÷N (NOT
// sample ÷N-1) per /grade R1 F1. Returns 0 for empty input or single
// value (single value has zero variance).
func populationStddev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	m := mean(values)
	var sumSq float64
	for _, v := range values {
		d := v - m
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(values)))
}

// minFloat returns the smallest value; 0 for empty input.
func minFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	m := values[0]
	for _, v := range values[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

// maxFloat returns the largest value; 0 for empty input.
func maxFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	m := values[0]
	for _, v := range values[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
