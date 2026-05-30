// Package batch is the Phase-2 batch CLI runner for sofdevsim. It reads a
// declarative JSON config and runs N independent simulations unattended,
// emitting per-run CSV bundles + a runs.csv index + an experiment.json
// provenance file. See docs/use-cases.md §UC41 (WHAT) and docs/design.md
// §Batch CLI (UC41) (HOW). Cycle #21831.
package batch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// ErrInvalidConfig wraps all config-validation errors. Callers use
// errors.Is(err, ErrInvalidConfig) to distinguish validation failure
// from I/O failure in LoadConfig.
var ErrInvalidConfig = errors.New("invalid batch config")

// Config is the declarative shape of a batch experiment.
// JSON tags match docs/design.md §Batch CLI (UC41) schema table.
type Config struct {
	Name         string         `json:"name"`
	Policy       string         `json:"policy"`
	Scenario     string         `json:"scenario"`
	Sprints      int            `json:"sprints"`
	TeamSize     int            `json:"team_size"`
	PhaseWIPCaps map[string]int `json:"phase_wip_caps,omitempty"`
	ReleaseMode  string         `json:"release_mode"`
	SeedRange    []int64        `json:"seed_range,omitempty"`
	Seeds        []int64        `json:"seeds,omitempty"`
}

// LoadConfig reads a JSON config file from path and validates it.
// Returns the parsed Config on success; returns an error wrapping
// ErrInvalidConfig if validation fails, or an I/O / json.Unmarshal
// error otherwise.
func LoadConfig(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Validate enforces all batch config invariants. Returns the first
// violation found wrapped in ErrInvalidConfig, or nil if valid.
func (c Config) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidConfig)
	}
	if c.Policy == "" {
		return fmt.Errorf("%w: policy is required", ErrInvalidConfig)
	}
	if _, err := c.ResolvePolicy(); err != nil {
		return err
	}
	if c.Scenario == "" {
		return fmt.Errorf("%w: scenario is required", ErrInvalidConfig)
	}
	if _, ok := engine.Scenarios[c.Scenario]; !ok {
		return fmt.Errorf("%w: scenario %q not in engine.Scenarios", ErrInvalidConfig, c.Scenario)
	}
	if c.Sprints <= 0 {
		return fmt.Errorf("%w: sprints must be > 0, got %d", ErrInvalidConfig, c.Sprints)
	}
	if c.TeamSize <= 0 {
		return fmt.Errorf("%w: team_size must be > 0, got %d", ErrInvalidConfig, c.TeamSize)
	}
	if _, err := c.ResolveReleaseMode(); err != nil {
		return err
	}
	if _, err := c.ResolvePhaseWIPCaps(); err != nil {
		return err
	}
	if err := c.validateSeeds(); err != nil {
		return err
	}
	return nil
}

// validateSeeds enforces the SeedRange/Seeds mutual-exclusion + shape rules.
func (c Config) validateSeeds() error {
	hasRange := len(c.SeedRange) > 0
	hasList := len(c.Seeds) > 0
	if hasRange && hasList {
		return fmt.Errorf("%w: seed_range and seeds are mutually exclusive (set exactly one)", ErrInvalidConfig)
	}
	if !hasRange && !hasList {
		return fmt.Errorf("%w: exactly one of seed_range or seeds must be set", ErrInvalidConfig)
	}
	if hasRange {
		if len(c.SeedRange) != 2 {
			return fmt.Errorf("%w: seed_range must have exactly 2 elements [lo, hi], got %d", ErrInvalidConfig, len(c.SeedRange))
		}
		if c.SeedRange[0] > c.SeedRange[1] {
			return fmt.Errorf("%w: seed_range lo (%d) must be <= hi (%d)", ErrInvalidConfig, c.SeedRange[0], c.SeedRange[1])
		}
	}
	return nil
}

// ResolveSeeds expands SeedRange into a list, or returns Seeds as-is.
// Callers must call Validate first; the result is empty if both fields
// are unset.
func (c Config) ResolveSeeds() []int64 {
	if len(c.Seeds) > 0 {
		out := make([]int64, len(c.Seeds))
		copy(out, c.Seeds)
		return out
	}
	if len(c.SeedRange) == 2 {
		lo, hi := c.SeedRange[0], c.SeedRange[1]
		out := make([]int64, 0, hi-lo+1)
		for s := lo; s <= hi; s++ {
			out = append(out, s)
		}
		return out
	}
	return nil
}

// ResolvePolicy maps the config string to the model enum.
// Mirrors the precedent at internal/api/handlers.go:131-141.
func (c Config) ResolvePolicy() (model.SizingPolicy, error) {
	switch c.Policy {
	case "none":
		return model.PolicyNone, nil
	case "dora-strict":
		return model.PolicyDORAStrict, nil
	case "tameflow-cognitive":
		return model.PolicyTameFlowCognitive, nil
	case "hybrid":
		return model.PolicyHybrid, nil
	}
	return 0, fmt.Errorf("%w: policy %q (valid: none, dora-strict, tameflow-cognitive, hybrid)", ErrInvalidConfig, c.Policy)
}

// ResolveReleaseMode wraps model.ParseReleaseMode + rewraps errors in
// ErrInvalidConfig so callers can errors.Is them uniformly.
func (c Config) ResolveReleaseMode() (model.ReleaseMode, error) {
	mode, err := model.ParseReleaseMode(c.ReleaseMode)
	if err != nil {
		return mode, fmt.Errorf("%w: release_mode %q: %v", ErrInvalidConfig, c.ReleaseMode, err)
	}
	return mode, nil
}

// ResolvePhaseWIPCaps translates the JSON string-keyed map to the
// model-enum-keyed map registry.CreateSimulation expects. Mirrors the
// precedent at internal/api/handlers.go::parsePhaseWIPConfig.
func (c Config) ResolvePhaseWIPCaps() (map[model.WorkflowPhase]int, error) {
	if len(c.PhaseWIPCaps) == 0 {
		return nil, nil
	}
	out := make(map[model.WorkflowPhase]int, len(c.PhaseWIPCaps))
	for k, v := range c.PhaseWIPCaps {
		phase, ok := parsePhaseName(k)
		if !ok {
			return nil, fmt.Errorf("%w: phase_wip_caps unknown phase %q (valid: Research, Sizing, Planning, Implement, Verify, CICD, Review)", ErrInvalidConfig, k)
		}
		out[phase] = v
	}
	return out, nil
}

// parsePhaseName accepts canonical uppercase phase names; case-insensitive
// via the caller. Mirror of internal/api::parsePhaseName.
func parsePhaseName(s string) (model.WorkflowPhase, bool) {
	switch toUpper(s) {
	case "RESEARCH":
		return model.PhaseResearch, true
	case "SIZING":
		return model.PhaseSizing, true
	case "PLANNING":
		return model.PhasePlanning, true
	case "IMPLEMENT":
		return model.PhaseImplement, true
	case "VERIFY":
		return model.PhaseVerify, true
	case "CI/CD", "CICD":
		return model.PhaseCICD, true
	case "REVIEW":
		return model.PhaseReview, true
	}
	return 0, false
}

// toUpper is a stdlib-only ASCII uppercase (avoids importing strings just
// for ToUpper in this single call site).
func toUpper(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
