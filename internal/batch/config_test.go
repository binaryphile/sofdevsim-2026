package batch

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// validMinimalConfig returns a Config that passes Validate. Tests start
// from this and break ONE field at a time (Khorikov Algorithm-tier
// output-based — drives validation paths via observable rejection).
func validMinimalConfig() Config {
	return Config{
		Name:        "test-experiment",
		Policy:      "dora-strict",
		Scenario:    "healthy",
		Sprints:     3,
		TeamSize:    3,
		ReleaseMode: "push",
		Seeds:       []int64{42, 99},
	}
}

func TestConfig_Validate_AcceptsMinimalConfig(t *testing.T) {
	cfg := validMinimalConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected Validate to accept minimal config, got: %v", err)
	}
}

func TestConfig_Validate_AcceptsSeedRange(t *testing.T) {
	cfg := validMinimalConfig()
	cfg.Seeds = nil
	cfg.SeedRange = []int64{100, 105}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected Validate to accept seed_range, got: %v", err)
	}
}

func TestConfig_Validate_AcceptsAllPolicies(t *testing.T) {
	for _, p := range []string{"none", "dora-strict", "tameflow-cognitive", "hybrid"} {
		cfg := validMinimalConfig()
		cfg.Policy = p
		if err := cfg.Validate(); err != nil {
			t.Errorf("Policy=%q: %v", p, err)
		}
	}
}

func TestConfig_Validate_AcceptsAllReleaseModes(t *testing.T) {
	for _, m := range []string{"push", "demand"} {
		cfg := validMinimalConfig()
		cfg.ReleaseMode = m
		if err := cfg.Validate(); err != nil {
			t.Errorf("ReleaseMode=%q: %v", m, err)
		}
	}
}

func TestConfig_Validate_AcceptsPhaseWIPCaps(t *testing.T) {
	cfg := validMinimalConfig()
	cfg.PhaseWIPCaps = map[string]int{"Implement": 4, "CICD": 1}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected Validate to accept phase_wip_caps, got: %v", err)
	}
}

func TestConfig_Validate_RejectsMissingName(t *testing.T) {
	cfg := validMinimalConfig()
	cfg.Name = ""
	assertRejected(t, cfg, "name")
}

func TestConfig_Validate_RejectsMissingPolicy(t *testing.T) {
	cfg := validMinimalConfig()
	cfg.Policy = ""
	assertRejected(t, cfg, "policy")
}

func TestConfig_Validate_RejectsMissingScenario(t *testing.T) {
	cfg := validMinimalConfig()
	cfg.Scenario = ""
	assertRejected(t, cfg, "scenario")
}

func TestConfig_Validate_RejectsSprintsZero(t *testing.T) {
	cfg := validMinimalConfig()
	cfg.Sprints = 0
	assertRejected(t, cfg, "sprints")
}

func TestConfig_Validate_RejectsSprintsNegative(t *testing.T) {
	cfg := validMinimalConfig()
	cfg.Sprints = -1
	assertRejected(t, cfg, "sprints")
}

func TestConfig_Validate_RejectsTeamSizeZero(t *testing.T) {
	cfg := validMinimalConfig()
	cfg.TeamSize = 0
	assertRejected(t, cfg, "team_size")
}

func TestConfig_Validate_RejectsInvalidPolicy(t *testing.T) {
	cfg := validMinimalConfig()
	cfg.Policy = "bogus-policy"
	assertRejected(t, cfg, "policy")
}

func TestConfig_Validate_RejectsInvalidScenario(t *testing.T) {
	cfg := validMinimalConfig()
	cfg.Scenario = "no-such-scenario"
	assertRejected(t, cfg, "scenario")
}

func TestConfig_Validate_RejectsInvalidReleaseMode(t *testing.T) {
	cfg := validMinimalConfig()
	cfg.ReleaseMode = "bogus-mode"
	assertRejected(t, cfg, "release_mode")
}

func TestConfig_Validate_RejectsBothSeedsAndSeedRange(t *testing.T) {
	cfg := validMinimalConfig()
	cfg.SeedRange = []int64{1, 10}
	// Seeds already set in validMinimalConfig
	assertRejected(t, cfg, "mutually exclusive")
}

func TestConfig_Validate_RejectsNeitherSeedsNorSeedRange(t *testing.T) {
	cfg := validMinimalConfig()
	cfg.Seeds = nil
	cfg.SeedRange = nil
	assertRejected(t, cfg, "seed")
}

func TestConfig_Validate_RejectsSeedRangeWrongLength(t *testing.T) {
	cfg := validMinimalConfig()
	cfg.Seeds = nil
	cfg.SeedRange = []int64{1, 2, 3}
	assertRejected(t, cfg, "seed_range")
}

func TestConfig_Validate_RejectsSeedRangeLoGreaterThanHi(t *testing.T) {
	cfg := validMinimalConfig()
	cfg.Seeds = nil
	cfg.SeedRange = []int64{10, 1}
	assertRejected(t, cfg, "seed_range")
}

func TestConfig_Validate_RejectsEmptySeeds(t *testing.T) {
	cfg := validMinimalConfig()
	cfg.Seeds = []int64{}
	cfg.SeedRange = nil
	assertRejected(t, cfg, "seed")
}

func TestConfig_Validate_RejectsBogusPhaseWIPKey(t *testing.T) {
	cfg := validMinimalConfig()
	cfg.PhaseWIPCaps = map[string]int{"BogusPhase": 5}
	assertRejected(t, cfg, "phase")
}

func TestConfig_ResolveSeeds_FromSeeds(t *testing.T) {
	cfg := validMinimalConfig()
	cfg.Seeds = []int64{1, 2, 3}
	cfg.SeedRange = nil
	got := cfg.ResolveSeeds()
	want := []int64{1, 2, 3}
	if !equalInt64Slice(got, want) {
		t.Errorf("ResolveSeeds() = %v, want %v", got, want)
	}
}

func TestConfig_ResolveSeeds_FromSeedRange(t *testing.T) {
	cfg := validMinimalConfig()
	cfg.Seeds = nil
	cfg.SeedRange = []int64{10, 12}
	got := cfg.ResolveSeeds()
	want := []int64{10, 11, 12}
	if !equalInt64Slice(got, want) {
		t.Errorf("ResolveSeeds() = %v, want %v", got, want)
	}
}

func TestConfig_ResolvePolicy_MapsAllValues(t *testing.T) {
	cases := []struct {
		in   string
		want model.SizingPolicy
	}{
		{"none", model.PolicyNone},
		{"dora-strict", model.PolicyDORAStrict},
		{"tameflow-cognitive", model.PolicyTameFlowCognitive},
		{"hybrid", model.PolicyHybrid},
	}
	for _, tc := range cases {
		cfg := validMinimalConfig()
		cfg.Policy = tc.in
		got, err := cfg.ResolvePolicy()
		if err != nil {
			t.Errorf("Policy=%q: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Policy=%q: got %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestLoadConfig_ReadsAndValidates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")
	body := `{
  "name": "tmp",
  "policy": "dora-strict",
  "scenario": "healthy",
  "sprints": 1,
  "team_size": 2,
  "release_mode": "push",
  "seeds": [7]
}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Name != "tmp" || cfg.Sprints != 1 || len(cfg.Seeds) != 1 || cfg.Seeds[0] != 7 {
		t.Errorf("unexpected cfg: %+v", cfg)
	}
}

func TestLoadConfig_RejectsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected LoadConfig to reject invalid JSON")
	}
}

func TestLoadConfig_ExampleMinimalJSON(t *testing.T) {
	// Pins the shipped examples/batch/minimal.json to the parser contract.
	// If example or parser drifts, this fails — guards the README's
	// copy-modify-starter promise.
	cfg, err := LoadConfig("../../examples/batch/minimal.json")
	if err != nil {
		t.Fatalf("examples/batch/minimal.json failed to load: %v", err)
	}
	if cfg.Name != "example" {
		t.Errorf("example Name=%q, want example", cfg.Name)
	}
	if len(cfg.Seeds) != 3 {
		t.Errorf("example Seeds count=%d, want 3", len(cfg.Seeds))
	}
}

func TestLoadConfig_PropagatesValidationError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")
	body := `{"name": "", "policy": "dora-strict", "scenario": "healthy", "sprints": 1, "team_size": 2, "release_mode": "push", "seeds":[1]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected LoadConfig to propagate validation error")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("expected error to mention 'name', got: %v", err)
	}
}

// assertRejected runs Validate and fails if it doesn't error OR if the
// error message doesn't contain the expected substring.
func assertRejected(t *testing.T, cfg Config, msgContains string) {
	t.Helper()
	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected Validate to reject; got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(msgContains)) {
		t.Errorf("error message %q does not contain %q", err.Error(), msgContains)
	}
	// Ensure error is the package's sentinel-shape (errors.Is for ErrInvalidConfig)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("error %v should wrap ErrInvalidConfig", err)
	}
}

func equalInt64Slice(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
