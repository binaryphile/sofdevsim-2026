package batch

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// toolVersion is the batch-CLI tool version embedded in experiment.json.
// Cycle 1 = "sofdevsim-batch/1.0". Bumped on wire-visible behavior changes.
const toolVersion = "sofdevsim-batch/1.0"

// runsCSVHeader is the row-0 header for runs.csv. Stable wire surface
// per docs/design.md §Batch CLI (UC41); /i P4-I1 contract: header always
// emitted (even for empty Runs) so consumer scripts get a parseable shape.
var runsCSVHeader = []string{
	"run_index", "seed", "policy", "scenario",
	"status", "output_dir", "sprints_run", "error_message",
}

// RunResult captures the outcome of a single batch run. Populated by
// Runner.Run; consumed by WriteRunsCSV.
type RunResult struct {
	Index      int
	Seed       int64
	Policy     string
	Scenario   string
	Status     string // "succeeded" | "failed"
	OutputDir  string
	SprintsRun int
	Error      string // empty when Status == "succeeded"
}

// Results bundles all per-run outcomes from a Runner.Run invocation.
type Results struct {
	Runs []RunResult
}

// experimentJSON is the on-disk shape of experiment.json. Schema-versioned
// (v1) per /i P4-I1 for forward-compat with downstream R/Python tooling.
type experimentJSON struct {
	SchemaVersion int    `json:"schema_version"`
	Name          string `json:"name"`
	Config        Config `json:"config"`
	GitSHA        string `json:"git_sha"`
	ToolVersion   string `json:"tool_version"`
	StartedAt     string `json:"started_at"`
}

// WriteExperimentJSON writes outDir/experiment.json with provenance
// metadata. startedAt is an RFC3339 timestamp string; gitSHA is the
// SHA captured by captureGitSHA (or "unknown" fallback).
func WriteExperimentJSON(outDir string, cfg Config, gitSHA, startedAt string) error {
	doc := experimentJSON{
		SchemaVersion: 1,
		Name:          cfg.Name,
		Config:        cfg,
		GitSHA:        gitSHA,
		ToolVersion:   toolVersion,
		StartedAt:     startedAt,
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal experiment.json: %w", err)
	}
	// Newline-terminate so POSIX tools treat it as a proper text file.
	data = append(data, '\n')
	path := filepath.Join(outDir, "experiment.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write experiment.json: %w", err)
	}
	return nil
}

// WriteRunsCSV writes outDir/runs.csv with the index header + one data row
// per run. Header row always emitted (even for empty runs) per the /i P4-I1
// contract: consumer scripts parse a stable shape regardless of N.
func WriteRunsCSV(outDir string, runs []RunResult) error {
	path := filepath.Join(outDir, "runs.csv")
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create runs.csv: %w", err)
	}
	defer file.Close()

	w := csv.NewWriter(file)
	defer w.Flush()

	if err := w.Write(runsCSVHeader); err != nil {
		return fmt.Errorf("runs.csv header: %w", err)
	}
	for _, r := range runs {
		row := []string{
			strconv.Itoa(r.Index),
			strconv.FormatInt(r.Seed, 10),
			r.Policy,
			r.Scenario,
			r.Status,
			r.OutputDir,
			strconv.Itoa(r.SprintsRun),
			r.Error,
		}
		if err := w.Write(row); err != nil {
			return fmt.Errorf("runs.csv row %d: %w", r.Index, err)
		}
	}
	return nil
}

// captureGitSHA best-effort returns the current git HEAD SHA. Falls back
// to "unknown" per /i P2-I5 if git is absent OR CWD not in a repo OR
// the command times out (2s). Production-safe for installed-binary use.
func captureGitSHA() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	sha := strings.TrimSpace(string(out))
	if sha == "" {
		return "unknown"
	}
	return sha
}
