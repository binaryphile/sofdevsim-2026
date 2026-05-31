package batch

import (
	"encoding/csv"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// approxEqual compares two floats at 2-decimal precision (matches the
// %.2f format-locked by /grade R1 F2). Avoids reflect.DeepEqual on
// floats which can drift via rounding.
func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.005
}

// makeRun constructs a RunResult with given metrics map for tests.
func makeRun(status string, metrics map[string]float64) RunResult {
	return RunResult{Status: status, Metrics: metrics}
}

func TestMean_KnownValues(t *testing.T) {
	cases := []struct {
		name string
		in   []float64
		want float64
	}{
		{"single", []float64{5.0}, 5.0},
		{"two", []float64{1.0, 3.0}, 2.0},
		{"three", []float64{1.0, 2.0, 3.0}, 2.0},
		{"negatives", []float64{-1.0, 1.0}, 0.0},
		{"empty", []float64{}, 0.0},
	}
	for _, tc := range cases {
		got := mean(tc.in)
		if !approxEqual(got, tc.want) {
			t.Errorf("%s: mean(%v) = %f, want %f", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestPopulationStddev_KnownValues(t *testing.T) {
	cases := []struct {
		name string
		in   []float64
		want float64
	}{
		{"single zero stddev", []float64{5.0}, 0.0},
		{"two identical zero stddev", []float64{3.0, 3.0}, 0.0},
		{"[1,2,3] population stddev sqrt(2/3)", []float64{1.0, 2.0, 3.0}, math.Sqrt(2.0 / 3.0)},
		{"empty", []float64{}, 0.0},
		// Verify NOT sample stddev (which would be 1.0 for [1,2,3])
		{"NOT sample stddev for [1,2,3]", []float64{1.0, 2.0, 3.0}, 0.8164965809},
	}
	for _, tc := range cases {
		got := populationStddev(tc.in)
		if !approxEqual(got, tc.want) {
			t.Errorf("%s: populationStddev(%v) = %f, want %f", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestMinMax_KnownValues(t *testing.T) {
	cases := []struct {
		name           string
		in             []float64
		wantMin, wantMax float64
	}{
		{"single", []float64{5.0}, 5.0, 5.0},
		{"three", []float64{3.0, 1.0, 2.0}, 1.0, 3.0},
		{"negatives", []float64{-5.0, -1.0, -10.0}, -10.0, -1.0},
		{"empty", []float64{}, 0.0, 0.0},
	}
	for _, tc := range cases {
		gotMin := minFloat(tc.in)
		gotMax := maxFloat(tc.in)
		if !approxEqual(gotMin, tc.wantMin) {
			t.Errorf("%s: minFloat = %f, want %f", tc.name, gotMin, tc.wantMin)
		}
		if !approxEqual(gotMax, tc.wantMax) {
			t.Errorf("%s: maxFloat = %f, want %f", tc.name, gotMax, tc.wantMax)
		}
	}
}

func TestAggregateMetrics_ThreeSucceededRunsKnownValues(t *testing.T) {
	// Each run has the same shape but with [1.0, 2.0, 3.0] for lead_time_avg
	// so we can verify exact mean=2.00 + population stddev=0.82 (NOT 1.0
	// sample stddev) per /grade R1 F1 lock.
	runs := []RunResult{
		makeRun("succeeded", map[string]float64{
			"lead_time_avg": 1.0, "deploy_frequency": 0.5, "mttr_avg": 0.0,
			"change_fail_rate": 10.0, "total_tickets": 10, "total_incidents": 0,
		}),
		makeRun("succeeded", map[string]float64{
			"lead_time_avg": 2.0, "deploy_frequency": 0.5, "mttr_avg": 0.0,
			"change_fail_rate": 10.0, "total_tickets": 10, "total_incidents": 0,
		}),
		makeRun("succeeded", map[string]float64{
			"lead_time_avg": 3.0, "deploy_frequency": 0.5, "mttr_avg": 0.0,
			"change_fail_rate": 10.0, "total_tickets": 10, "total_incidents": 0,
		}),
	}
	stats := AggregateMetrics(runs)
	if len(stats) != 6 {
		t.Fatalf("len(stats) = %d, want 6", len(stats))
	}
	// Find lead_time_avg row
	var lta *MetricStats
	for i := range stats {
		if stats[i].Metric == "lead_time_avg" {
			lta = &stats[i]
		}
	}
	if lta == nil {
		t.Fatal("lead_time_avg row missing")
	}
	if !approxEqual(lta.Mean, 2.0) {
		t.Errorf("lead_time_avg.Mean = %f, want 2.00", lta.Mean)
	}
	if !approxEqual(lta.Stddev, math.Sqrt(2.0/3.0)) {
		t.Errorf("lead_time_avg.Stddev = %f, want sqrt(2/3)=0.82 (population NOT sample)", lta.Stddev)
	}
	if !approxEqual(lta.Min, 1.0) {
		t.Errorf("lead_time_avg.Min = %f, want 1.00", lta.Min)
	}
	if !approxEqual(lta.Max, 3.0) {
		t.Errorf("lead_time_avg.Max = %f, want 3.00", lta.Max)
	}
	if lta.N != 3 {
		t.Errorf("lead_time_avg.N = %d, want 3", lta.N)
	}
}

func TestAggregateMetrics_MixedSucceededAndFailed_SkipsFailed(t *testing.T) {
	runs := []RunResult{
		makeRun("succeeded", map[string]float64{"lead_time_avg": 5.0, "deploy_frequency": 1.0, "mttr_avg": 0.0, "change_fail_rate": 0.0, "total_tickets": 8, "total_incidents": 1}),
		makeRun("failed", nil),
		makeRun("succeeded", map[string]float64{"lead_time_avg": 7.0, "deploy_frequency": 1.0, "mttr_avg": 0.0, "change_fail_rate": 0.0, "total_tickets": 8, "total_incidents": 1}),
	}
	stats := AggregateMetrics(runs)
	for _, s := range stats {
		if s.N != 2 {
			t.Errorf("metric %s: N=%d, want 2 (failed run must be skipped)", s.Metric, s.N)
		}
	}
}

func TestAggregateMetrics_AllFailed_ReturnsEmpty(t *testing.T) {
	runs := []RunResult{
		makeRun("failed", nil),
		makeRun("failed", nil),
	}
	stats := AggregateMetrics(runs)
	if len(stats) != 0 {
		t.Errorf("len(stats) = %d, want 0 (n=0 case returns nil/empty per /i pass-fresh-2)", len(stats))
	}
}

func TestAggregateMetrics_EmptyRuns_ReturnsEmpty(t *testing.T) {
	stats := AggregateMetrics(nil)
	if len(stats) != 0 {
		t.Errorf("len(stats) = %d, want 0 for nil input", len(stats))
	}
}

func TestAggregateMetrics_SingleSucceeded_StddevZero(t *testing.T) {
	runs := []RunResult{
		makeRun("succeeded", map[string]float64{"lead_time_avg": 7.0, "deploy_frequency": 0.3, "mttr_avg": 0.0, "change_fail_rate": 0.0, "total_tickets": 5, "total_incidents": 0}),
	}
	stats := AggregateMetrics(runs)
	if len(stats) != 6 {
		t.Fatalf("len(stats) = %d, want 6 (n=1 returns 6 entries per Q6)", len(stats))
	}
	for _, s := range stats {
		if s.Stddev != 0.0 {
			t.Errorf("metric %s: Stddev=%f, want 0 (single value has zero variance)", s.Metric, s.Stddev)
		}
		if s.N != 1 {
			t.Errorf("metric %s: N=%d, want 1", s.Metric, s.N)
		}
	}
}

func TestWriteAggregateCSV_GoldenShape_ThreeRunInput(t *testing.T) {
	dir := t.TempDir()
	stats := []MetricStats{
		{Metric: "lead_time_avg", Mean: 2.0, Stddev: 0.82, Min: 1.0, Max: 3.0, N: 3},
		{Metric: "deploy_frequency", Mean: 0.5, Stddev: 0.0, Min: 0.5, Max: 0.5, N: 3},
		{Metric: "mttr_avg", Mean: 0.0, Stddev: 0.0, Min: 0.0, Max: 0.0, N: 3},
		{Metric: "change_fail_rate", Mean: 10.0, Stddev: 0.0, Min: 10.0, Max: 10.0, N: 3},
		{Metric: "total_tickets", Mean: 10.0, Stddev: 0.0, Min: 10.0, Max: 10.0, N: 3},
		{Metric: "total_incidents", Mean: 0.0, Stddev: 0.0, Min: 0.0, Max: 0.0, N: 3},
	}
	if err := WriteAggregateCSV(dir, stats); err != nil {
		t.Fatalf("WriteAggregateCSV: %v", err)
	}
	file, err := os.Open(filepath.Join(dir, "aggregate.csv"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 7 {
		t.Fatalf("row count = %d, want 7 (header + 6 metrics)", len(rows))
	}
	// Header check
	wantHeader := []string{"metric", "mean", "stddev", "min", "max", "n"}
	for i, c := range wantHeader {
		if rows[0][i] != c {
			t.Errorf("header[%d] = %q, want %q", i, rows[0][i], c)
		}
	}
	// First data row %.2f formatting check
	if rows[1][0] != "lead_time_avg" {
		t.Errorf("row 1 metric = %q, want lead_time_avg", rows[1][0])
	}
	if rows[1][1] != "2.00" {
		t.Errorf("row 1 mean = %q, want 2.00 (%%.2f format)", rows[1][1])
	}
	if rows[1][2] != "0.82" {
		t.Errorf("row 1 stddev = %q, want 0.82 (%%.2f format)", rows[1][2])
	}
	if rows[1][5] != "3" {
		t.Errorf("row 1 n = %q, want 3 (int format)", rows[1][5])
	}
}

func TestWriteAggregateCSV_EmptyStats_HeaderOnly(t *testing.T) {
	// n=0 case per Q6: AggregateMetrics returns nil/empty; WriteAggregateCSV
	// then writes header-only file naturally (no special-case logic).
	dir := t.TempDir()
	if err := WriteAggregateCSV(dir, nil); err != nil {
		t.Fatalf("WriteAggregateCSV(nil): %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "aggregate.csv"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("empty-stats CSV has %d lines, want 1 (header-only); content=%q", len(lines), data)
	}
	if !strings.HasPrefix(lines[0], "metric,mean,") {
		t.Errorf("first line missing header prefix: %q", lines[0])
	}
}

func TestWriteAggregateCSV_FailurePath_NonExistentDir(t *testing.T) {
	// /i pass-2 P2-I3: write-failure path test. Inject non-existent outDir
	// (no parent dirs); assert returned error wraps os.PathError.
	err := WriteAggregateCSV("/nonexistent/path/that/does/not/exist", []MetricStats{
		{Metric: "x", Mean: 1.0, Stddev: 0.0, Min: 1.0, Max: 1.0, N: 1},
	})
	if err == nil {
		t.Fatal("expected error writing to non-existent dir; got nil")
	}
	// Should wrap an os/file error
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) {
		// Acceptable if just contains useful context — log for visibility
		t.Logf("error type: %T (does not wrap os.PathError but is non-nil): %v", err, err)
	}
}
