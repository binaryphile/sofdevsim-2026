package batch

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/events"
)

// runnerTestConfig is the minimal config used by Run() integration tests.
// 2 seeds × 1 sprint × healthy scenario × team_size 3.
func runnerTestConfig() Config {
	return Config{
		Name:        "runner-test",
		Policy:      "dora-strict",
		Scenario:    "healthy",
		Sprints:     1,
		TeamSize:    3,
		ReleaseMode: "push",
		Seeds:       []int64{42, 99},
	}
}

func TestRunner_Run_ProducesPerSeedSubdirs(t *testing.T) {
	dir := t.TempDir()
	r := NewRunner()
	results, err := r.Run(runnerTestConfig(), dir)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(results.Runs) != 2 {
		t.Fatalf("Runs count=%d, want 2", len(results.Runs))
	}
	for _, run := range results.Runs {
		if run.Status != "succeeded" {
			t.Errorf("run-%d status=%q (err=%q), want succeeded", run.Index, run.Status, run.Error)
		}
		if run.OutputDir == "" {
			t.Errorf("run-%d OutputDir empty", run.Index)
		}
		// OutputDir should be under dir; verify the file exists
		if _, statErr := os.Stat(run.OutputDir); statErr != nil {
			t.Errorf("run-%d output dir %q missing: %v", run.Index, run.OutputDir, statErr)
		}
	}
	// experiment.json + runs.csv + aggregate.csv at outDir root (fu1 #21832 adds aggregate.csv)
	if _, err := os.Stat(filepath.Join(dir, "experiment.json")); err != nil {
		t.Errorf("experiment.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "aggregate.csv")); err != nil {
		t.Errorf("aggregate.csv missing (fu1): %v", err)
	}
	// aggregate.csv shape: 7 rows (header + 6 metrics); mean within [min, max]
	aggFile, err := os.Open(filepath.Join(dir, "aggregate.csv"))
	if err == nil {
		defer aggFile.Close()
		aggRows, _ := csv.NewReader(aggFile).ReadAll()
		if len(aggRows) != 7 {
			t.Errorf("aggregate.csv row count=%d, want 7 (header + 6 metric rows)", len(aggRows))
		}
		// Header check
		wantAggHeader := []string{"metric", "mean", "stddev", "min", "max", "n"}
		for i, c := range wantAggHeader {
			if i < len(aggRows[0]) && aggRows[0][i] != c {
				t.Errorf("aggregate header[%d]=%q want %q", i, aggRows[0][i], c)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "runs.csv")); err != nil {
		t.Errorf("runs.csv missing: %v", err)
	}
}

func TestRunner_Run_RunsCsvHasTwoDataRows(t *testing.T) {
	dir := t.TempDir()
	r := NewRunner()
	if _, err := r.Run(runnerTestConfig(), dir); err != nil {
		t.Fatalf("Run: %v", err)
	}
	file, err := os.Open(filepath.Join(dir, "runs.csv"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("csv parse: %v", err)
	}
	// 1 header + 2 data
	if len(rows) != 3 {
		t.Errorf("row count=%d, want 3", len(rows))
	}
}

func TestRunner_Run_ExperimentJSONParseable(t *testing.T) {
	dir := t.TempDir()
	r := NewRunner()
	if _, err := r.Run(runnerTestConfig(), dir); err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "experiment.json"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got["schema_version"].(float64) != 1 {
		t.Errorf("schema_version=%v, want 1", got["schema_version"])
	}
}

func TestRunner_Run_RejectsInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := runnerTestConfig()
	cfg.Sprints = 0 // invalid
	r := NewRunner()
	_, err := r.Run(cfg, dir)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestRunner_Run_Isolation_PerRunStoresPairwiseDistinct(t *testing.T) {
	// F3 mitigation per /grade R2-F3 + 3a→1c supersedes: assert that each
	// per-run store is a distinct interface value (not just N invocations).
	// Defective Runner could call StoreFactory N times but return the SAME
	// store each time — this test catches that.
	var capturedStores []events.Store
	r := NewRunner()
	r.StoreFactory = func() events.Store {
		s := events.NewMemoryStore()
		capturedStores = append(capturedStores, s)
		return s
	}
	cfg := runnerTestConfig()
	cfg.Seeds = []int64{1, 2, 3}
	dir := t.TempDir()
	if _, err := r.Run(cfg, dir); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(capturedStores) != 3 {
		t.Errorf("StoreFactory invoked %d times, want 3", len(capturedStores))
	}
	for i := 0; i < len(capturedStores); i++ {
		for j := i + 1; j < len(capturedStores); j++ {
			if capturedStores[i] == capturedStores[j] {
				t.Errorf("store[%d] == store[%d] (shared state — P3 isolation violated)", i, j)
			}
		}
	}
}

// panickyStore wraps a real MemoryStore but panics on Append so we can
// verify runOne's defer-recover captures the panic into RunResult.
// Per /i pass-1-I1 regression test.
type panickyStore struct{ events.Store }

func (panickyStore) Append(stream string, expectedVersion int, evts ...events.Event) error {
	panic("injected store panic for /i pass-1-I1 regression test")
}

func TestRunner_Run_PerRunPanic_CapturedInResult(t *testing.T) {
	// Regression test for /i pass-1-I1: defer-recover in runOne must
	// propagate via named return. With unnamed return (the bug),
	// Results.Runs[0] would be zero-valued (Status=""), not failed.
	r := NewRunner()
	r.StoreFactory = func() events.Store {
		return panickyStore{Store: events.NewMemoryStore()}
	}
	dir := t.TempDir()
	cfg := runnerTestConfig()
	cfg.Seeds = []int64{42}
	results, err := r.Run(cfg, dir)
	if err != nil {
		t.Fatalf("Run should swallow per-run panic: %v", err)
	}
	if len(results.Runs) != 1 {
		t.Fatalf("Runs count=%d, want 1", len(results.Runs))
	}
	if results.Runs[0].Status != "failed" {
		t.Errorf("Status=%q, want failed (defer-recover must mutate named return)", results.Runs[0].Status)
	}
	if !strings.Contains(results.Runs[0].Error, "panic:") {
		t.Errorf("Error=%q, want to contain 'panic:'", results.Runs[0].Error)
	}
}

// aggregateTestConfig is a separate 3-seed config helper per /i pass-2
// P2-I2 — existing TestRunner_Run_RunsCsvHasTwoDataRows has a literal
// 2-row assertion; bumping runnerTestConfig to 3 seeds would break it,
// so the StddevNonZero subtest gets its own config.
func aggregateTestConfig() Config {
	return Config{
		Name: "aggregate-test", Policy: "dora-strict", Scenario: "healthy",
		Sprints: 1, TeamSize: 3, ReleaseMode: "push",
		Seeds: []int64{42, 99, 123},
	}
}

func TestRunner_Run_Aggregate_StddevNonZero(t *testing.T) {
	// Semantic-plausibility check per /i pass-2 P2-I2: with 3 seeds (3 distinct
	// per-run metric values are likely), Stddev > 0 for at least one of the 6
	// metrics. With 2 seeds Stddev>0 only if a≠b which is fragile; 3 seeds
	// makes the check robust.
	dir := t.TempDir()
	if _, err := NewRunner().Run(aggregateTestConfig(), dir); err != nil {
		t.Fatalf("Run: %v", err)
	}
	file, err := os.Open(filepath.Join(dir, "aggregate.csv"))
	if err != nil {
		t.Fatalf("read aggregate.csv: %v", err)
	}
	defer file.Close()
	rows, _ := csv.NewReader(file).ReadAll()
	if len(rows) != 7 {
		t.Fatalf("aggregate.csv row count=%d, want 7", len(rows))
	}
	anyNonZero := false
	for i := 1; i < len(rows); i++ {
		// Column 2 is stddev (%.2f); parse and check > 0
		if rows[i][2] != "0.00" {
			anyNonZero = true
			break
		}
	}
	if !anyNonZero {
		t.Errorf("expected Stddev>0 for at least one of 6 metrics across 3 seeds; got all-zero: %v", rows[1:])
	}
}

func TestRunner_Run_Aggregate_AllRunsFailed_HeaderOnly(t *testing.T) {
	// n=0 case per Q6 — all runs panic, AggregateMetrics returns nil,
	// WriteAggregateCSV writes header-only file. Reuses panickyStore
	// from #21831 /i-pass-1 regression test.
	r := NewRunner()
	r.StoreFactory = func() events.Store {
		return panickyStore{Store: events.NewMemoryStore()}
	}
	dir := t.TempDir()
	cfg := runnerTestConfig()
	cfg.Seeds = []int64{1, 2, 3}
	if _, err := r.Run(cfg, dir); err != nil {
		t.Fatalf("Run: %v (should swallow per-run panics)", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "aggregate.csv"))
	if err != nil {
		t.Fatalf("read aggregate.csv: %v", err)
	}
	// Header-only: single non-empty line
	lines := []string{}
	for _, l := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) != 1 {
		t.Errorf("all-failed aggregate.csv has %d lines, want 1 (header only); got: %q", len(lines), data)
	}
}

func TestRunner_Run_AllSeedsFailedStillReturnsNilErr(t *testing.T) {
	// Multi-seed all-fail contract per criterion 3: per-run failures must
	// NOT propagate as Run-level errors; caller decides whether to treat
	// all-failed as success. Complements TestRunner_Run_PerRunPanic
	// (single-seed) by exercising the N-seed batch-continues path.
	r := NewRunner()
	r.StoreFactory = func() events.Store {
		return panickyStore{Store: events.NewMemoryStore()}
	}
	cfg := runnerTestConfig()
	cfg.Seeds = []int64{1, 2, 3}
	dir := t.TempDir()
	results, err := r.Run(cfg, dir)
	if err != nil {
		t.Fatalf("Run should swallow per-run panics; got err: %v", err)
	}
	if len(results.Runs) != 3 {
		t.Fatalf("Runs count=%d, want 3 (batch continues past each failure)", len(results.Runs))
	}
	for i, run := range results.Runs {
		if run.Status != "failed" {
			t.Errorf("run-%d Status=%q, want failed", i, run.Status)
		}
	}
	// runs.csv still gets written even when all runs failed
	if _, err := os.Stat(filepath.Join(dir, "runs.csv")); err != nil {
		t.Errorf("runs.csv missing on all-failed batch: %v", err)
	}
}

func TestRunner_Run_Determinism_RerunYieldsByteIdenticalDeterministicCSVs(t *testing.T) {
	// F1 determinism contract per /grade R2 absorption (3 CSVs byte-identical
	// + 2 wall-clock-bearing). Uses Go csv.Reader column-extraction (NOT shell
	// diff -I regex) per /i I3.
	cfg := runnerTestConfig()
	cfg.Seeds = []int64{777} // single seed → simpler comparison

	dirA := t.TempDir()
	dirB := t.TempDir()
	if _, err := NewRunner().Run(cfg, dirA); err != nil {
		t.Fatalf("Run A: %v", err)
	}
	if _, err := NewRunner().Run(cfg, dirB); err != nil {
		t.Fatalf("Run B: %v", err)
	}

	// Find the nested per-run sofdevsim-export-<timestamp> subdir on each side.
	nestedA := findNestedExportDir(t, filepath.Join(dirA, "run-0-seed-777"))
	nestedB := findNestedExportDir(t, filepath.Join(dirB, "run-0-seed-777"))

	// Byte-identical CSVs
	for _, name := range []string{"tickets.csv", "sprints.csv", "metrics.csv"} {
		assertByteIdentical(t, filepath.Join(nestedA, name), filepath.Join(nestedB, name))
	}

	// metadata.csv: identical except export_timestamp column (col 3)
	assertCSVColsEqualExcept(t, filepath.Join(nestedA, "metadata.csv"), filepath.Join(nestedB, "metadata.csv"), []int{3})

	// incidents.csv: identical except CreatedAt + ResolvedAt + mttr_days
	// columns (indices 3, 4, 5 per schema). If both runs produced zero
	// incidents the comparison is trivially equal (header-only).
	assertCSVColsEqualExcept(t, filepath.Join(nestedA, "incidents.csv"), filepath.Join(nestedB, "incidents.csv"), []int{3, 4, 5})

	// fu1 #21832 P-fresh-1: aggregate.csv is a pure function of deterministic
	// per-run metrics, so it MUST be byte-identical across the two outDirs.
	// Verifies the /grade R1 P3 in-principle claim with an actual assertion.
	assertByteIdentical(t, filepath.Join(dirA, "aggregate.csv"), filepath.Join(dirB, "aggregate.csv"))
}

// findNestedExportDir locates the single sofdevsim-export-<timestamp>/
// subdir inside parent (created by internal/export.ExportTo per
// writers.go:64).
func findNestedExportDir(t *testing.T, parent string) string {
	t.Helper()
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatalf("readdir %s: %v", parent, err)
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "sofdevsim-export-") {
			return filepath.Join(parent, e.Name())
		}
	}
	t.Fatalf("no sofdevsim-export-* subdir under %s", parent)
	return ""
}

func assertByteIdentical(t *testing.T, a, b string) {
	t.Helper()
	dataA, err := os.ReadFile(a)
	if err != nil {
		t.Fatalf("read %s: %v", a, err)
	}
	dataB, err := os.ReadFile(b)
	if err != nil {
		t.Fatalf("read %s: %v", b, err)
	}
	if string(dataA) != string(dataB) {
		t.Errorf("%s and %s differ (determinism contract violated)\n--- A ---\n%s\n--- B ---\n%s",
			filepath.Base(a), filepath.Base(b), dataA, dataB)
	}
}

// assertCSVColsEqualExcept reads two CSV files, drops the excludeCols
// indices from each row, and asserts the remaining columns are equal.
// Header row included in comparison.
func assertCSVColsEqualExcept(t *testing.T, a, b string, excludeCols []int) {
	t.Helper()
	rowsA := readCSV(t, a)
	rowsB := readCSV(t, b)
	if len(rowsA) != len(rowsB) {
		t.Errorf("%s row count differs: %d vs %d", filepath.Base(a), len(rowsA), len(rowsB))
		return
	}
	excludeSet := map[int]bool{}
	for _, i := range excludeCols {
		excludeSet[i] = true
	}
	for r, rowA := range rowsA {
		rowB := rowsB[r]
		for c := range rowA {
			if excludeSet[c] {
				continue
			}
			if c >= len(rowB) {
				t.Errorf("%s row %d col %d missing from B", filepath.Base(a), r, c)
				continue
			}
			if rowA[c] != rowB[c] {
				t.Errorf("%s row %d col %d differs: %q vs %q", filepath.Base(a), r, c, rowA[c], rowB[c])
			}
		}
	}
}

func readCSV(t *testing.T, path string) [][]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return rows
}
