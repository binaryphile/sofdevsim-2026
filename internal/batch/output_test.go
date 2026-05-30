package batch

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixedTime is the deterministic timestamp used by golden tests.
const fixedTime = "2026-05-30T20:00:00Z"

func sampleConfig() Config {
	return Config{
		Name:        "test-exp",
		Policy:      "dora-strict",
		Scenario:    "healthy",
		Sprints:     2,
		TeamSize:    3,
		ReleaseMode: "push",
		Seeds:       []int64{42, 99},
	}
}

func TestWriteExperimentJSON_GoldenShape(t *testing.T) {
	dir := t.TempDir()
	cfg := sampleConfig()
	if err := WriteExperimentJSON(dir, cfg, "abc123def456", fixedTime); err != nil {
		t.Fatalf("WriteExperimentJSON: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "experiment.json"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	// Parse + verify each known field exists and matches.
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parse: %v\nbody=%s", err, data)
	}
	// schema_version per /i P4-I1
	if v, ok := got["schema_version"].(float64); !ok || int(v) != 1 {
		t.Errorf("schema_version=%v, want 1", got["schema_version"])
	}
	if got["git_sha"] != "abc123def456" {
		t.Errorf("git_sha=%v, want abc123def456", got["git_sha"])
	}
	if got["started_at"] != fixedTime {
		t.Errorf("started_at=%v, want %s", got["started_at"], fixedTime)
	}
	if got["tool_version"] == nil || got["tool_version"] == "" {
		t.Errorf("tool_version missing/empty: %v", got["tool_version"])
	}
	// Config nested under "config" key with all fields preserved
	configMap, ok := got["config"].(map[string]any)
	if !ok {
		t.Fatalf("config is not an object: %v", got["config"])
	}
	if configMap["name"] != "test-exp" {
		t.Errorf("config.name=%v, want test-exp", configMap["name"])
	}
	if configMap["sprints"].(float64) != 2 {
		t.Errorf("config.sprints=%v, want 2", configMap["sprints"])
	}
}

func TestWriteExperimentJSON_UnknownGitSHA(t *testing.T) {
	dir := t.TempDir()
	if err := WriteExperimentJSON(dir, sampleConfig(), "unknown", fixedTime); err != nil {
		t.Fatalf("WriteExperimentJSON: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "experiment.json"))
	var got map[string]any
	json.Unmarshal(data, &got)
	if got["git_sha"] != "unknown" {
		t.Errorf("git_sha=%v, want unknown", got["git_sha"])
	}
}

func TestWriteRunsCSV_ThreeMixedResults(t *testing.T) {
	dir := t.TempDir()
	runs := []RunResult{
		{Index: 0, Seed: 42, Policy: "dora-strict", Scenario: "healthy", Status: "succeeded", OutputDir: "run-0-seed-42/sofdevsim-export-X", SprintsRun: 2, Error: ""},
		{Index: 1, Seed: 99, Policy: "dora-strict", Scenario: "healthy", Status: "failed", OutputDir: "run-1-seed-99", SprintsRun: 0, Error: "engine: simulation panic at tick 3"},
		{Index: 2, Seed: 123, Policy: "dora-strict", Scenario: "healthy", Status: "succeeded", OutputDir: "run-2-seed-123/sofdevsim-export-Y", SprintsRun: 2, Error: ""},
	}
	if err := WriteRunsCSV(dir, runs); err != nil {
		t.Fatalf("WriteRunsCSV: %v", err)
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
	// Header + 3 data rows
	if len(rows) != 4 {
		t.Fatalf("row count=%d, want 4 (1 header + 3 data)", len(rows))
	}
	want := []string{"run_index", "seed", "policy", "scenario", "status", "output_dir", "sprints_run", "error_message"}
	for i, col := range want {
		if rows[0][i] != col {
			t.Errorf("header col %d = %q, want %q", i, rows[0][i], col)
		}
	}
	if len(rows[0]) != len(want) {
		t.Errorf("header column count = %d, want %d", len(rows[0]), len(want))
	}
	// row[1] = succeeded run-0
	if rows[1][0] != "0" || rows[1][1] != "42" || rows[1][4] != "succeeded" || rows[1][7] != "" {
		t.Errorf("row 1 unexpected: %v", rows[1])
	}
	// row[2] = failed run-1 with error
	if rows[2][4] != "failed" || rows[2][7] != "engine: simulation panic at tick 3" {
		t.Errorf("row 2 unexpected: %v", rows[2])
	}
}

func TestWriteRunsCSV_EmptyRunsHeaderOnly(t *testing.T) {
	// Per /i P4-I1: header row always emitted even when Results.Runs is empty —
	// consumer scripts get parseable shape regardless of N.
	dir := t.TempDir()
	if err := WriteRunsCSV(dir, nil); err != nil {
		t.Fatalf("WriteRunsCSV(nil): %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "runs.csv"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	// Single line: the header.
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("empty-Runs CSV has %d lines, want 1 (header only); content=%q", len(lines), data)
	}
	if !strings.HasPrefix(lines[0], "run_index,seed,") {
		t.Errorf("first line missing header prefix: %q", lines[0])
	}
}

func TestCaptureGitSHA_NonEmpty(t *testing.T) {
	// In CI / repo CWD: returns real SHA.
	// Outside a repo: returns "unknown" (best-effort fallback per /i P2-I5).
	// Either way: non-empty string.
	sha := captureGitSHA()
	if sha == "" {
		t.Error("captureGitSHA() returned empty string; expected SHA or 'unknown'")
	}
}
