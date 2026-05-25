package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/api"
	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/lesson"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
	"github.com/binaryphile/sofdevsim-2026/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	seed := flag.Int64("seed", 0, "Random seed for reproducibility (0 = use current time)")
	port := flag.Int("port", 8080, "HTTP API port")
	clientMode := flag.Bool("client", false, "Use HTTP client mode (creates simulation via API)")
	localMode := flag.Bool("local", false, "Force local engine mode (for export, save/load, comparison)")
	lessonName := flag.String("lesson", "", "Run interactive lesson (e.g., buffer-crisis)")
	proverPath := flag.String("prover-path", "", "Path to zk-event-proofs project (default: ~/projects/zk-event-proofs)")
	mix := flag.String("mix", "healthy", "Backlog mix profile (UC37): healthy|overloaded|uncertain|mixed|uc37-default|bug-heavy|migration-quarter|infra-push|research-shop")
	phaseWIP := flag.String("phase-wip", "", "Per-phase WIP caps (UC38): comma-separated phase=cap, e.g. 'Implement=4,Verify=2,CICD=1,Review=2'. Empty = unlimited (regression-safe)")
	phaseWIPProfile := flag.String("phase-wip-profile", "", "Bundled phase-WIP profile (UC38): uncapped|balanced. Mutually exclusive with --phase-wip when both nonempty")
	releaseMode := flag.String("release-mode", "push", "Release controller mode (UC39): push|demand. Demand mode requires warm-up before dripping; falls back to push if analyzer can't lock constraint within 5 sprints")
	flag.Parse()

	// UC37: validate --mix upfront (registry will also reject unknown names, but
	// startup-time validation matches the README contract per UC37 ext §1b).
	if _, ok := engine.Scenarios[*mix]; !ok {
		names := make([]string, 0, len(engine.Scenarios))
		for n := range engine.Scenarios {
			names = append(names, n)
		}
		sort.Strings(names)
		fmt.Fprintf(os.Stderr, "Unknown --mix scenario %q. Registered: %v\n", *mix, names)
		os.Exit(1)
	}

	// UC38: parse --phase-wip / --phase-wip-profile. Syntax-only validation
	// here; semantic validation (ErrCapZero/Negative/BelowMentorMin/Conflict)
	// happens in registry.CreateSimulation (single-pass per Decision B).
	if *phaseWIP != "" && *phaseWIPProfile != "" {
		fmt.Fprintln(os.Stderr, "--phase-wip and --phase-wip-profile are mutually exclusive")
		os.Exit(1)
	}
	phaseWIPConfig, err := parsePhaseWIPFlags(*phaseWIP, *phaseWIPProfile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	// UC39: parse --release-mode upfront. Syntax-only checking here;
	// semantic validation (the enum value itself) flows through
	// registry.CreateSimulation per Decision E single-pass validation.
	parsedReleaseMode, err := model.ParseReleaseMode(*releaseMode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	// Handle lesson mode
	if *lessonName != "" {
		l, ok := lesson.Lessons[*lessonName]
		if !ok {
			fmt.Fprintf(os.Stderr, "Unknown lesson: %s\n", *lessonName)
			fmt.Fprintln(os.Stderr, "Available lessons:")
			for name, les := range lesson.Lessons {
				fmt.Fprintf(os.Stderr, "  %s - %s\n", name, les.Description())
			}
			os.Exit(1)
		}
		if err := l.Run(*proverPath); err != nil {
			fmt.Fprintf(os.Stderr, "Lesson error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Negative seeds treated as 0 (random)
	if *seed < 0 {
		*seed = 0
	}

	baseURL := fmt.Sprintf("http://localhost:%d", *port)

	// Auto-discovery: check if server already running
	existingServer := serverRunning(baseURL)

	// Determine mode: --local forces engine mode, --client forces HTTP mode
	// Otherwise, use client mode if server already running
	useClientMode := *clientMode || (existingServer && !*localMode)

	var registry api.SimRegistry
	if !existingServer {
		// Start our own server
		registry = api.NewSimRegistry()
		router := api.NewRouter(registry)
		go func() {
			addr := fmt.Sprintf(":%d", *port)
			if err := http.ListenAndServe(addr, router); err != nil {
				fmt.Fprintf(os.Stderr, "API server error: %v\n", err)
			}
		}()
		// Wait briefly for server to start
		time.Sleep(50 * time.Millisecond)
	}

	var app *tui.App

	if useClientMode {
		// HTTP client mode: create simulation via API
		client := tui.NewClient(baseURL)

		resp, err := client.CreateSimulation(*seed, "dora-strict", *mix, phaseWIPConfigStringKeys(phaseWIPConfig), parsedReleaseMode.String())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create simulation: %v\n", err)
			os.Exit(1)
		}

		app = tui.NewAppWithClient(client, resp.Simulation)
	} else {
		// Local engine mode: direct registry access (supports export, save/load, comparison)
		if registry.SimRegistry == nil {
			// No server started, create standalone registry
			registry = api.NewSimRegistry()
		}
		app = tui.NewAppWithRegistry(*seed, registry.SimRegistry, *mix, phaseWIPConfig, parsedReleaseMode)
	}

	app.EnableOpeningAnimation()
	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// serverRunning checks if a sofdevsim server is already running on the given URL.
// Verifies the service name to avoid connecting to unrelated servers.
func serverRunning(baseURL string) bool {
	client := &http.Client{Timeout: 100 * time.Millisecond}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var health struct {
		Service string `json:"service"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return false
	}

	return health.Service == "sofdevsim"
}

// UC38 #15443: --phase-wip / --phase-wip-profile parsing. Syntax-only;
// semantic validation flows through registry.CreateSimulation +
// model.ValidatePhaseWIPConfig (single-pass per Decision B).

// phaseWIPProfiles holds the two bundled cap profiles documented in
// README.md and design.md §"Per-Phase WIP Caps".
var phaseWIPProfiles = map[string]map[model.WorkflowPhase]int{
	"uncapped": nil, // nil = unlimited; CI/CD still bounded by CICDSlots fallback
	"balanced": {
		model.PhaseImplement: 4,
		model.PhaseVerify:    2,
		model.PhaseCICD:      1,
		model.PhaseReview:    2,
	},
}

// parsePhaseWIPFlags resolves the --phase-wip / --phase-wip-profile pair.
// Mutual exclusivity is checked by the caller (main); this function
// translates the resolved input into the domain map. Empty inputs return
// nil (unlimited; regression-safe).
func parsePhaseWIPFlags(rawList, profileName string) (map[model.WorkflowPhase]int, error) {
	if profileName != "" {
		cfg, ok := phaseWIPProfiles[profileName]
		if !ok {
			names := make([]string, 0, len(phaseWIPProfiles))
			for n := range phaseWIPProfiles {
				names = append(names, n)
			}
			sort.Strings(names)
			return nil, fmt.Errorf("unknown --phase-wip-profile %q (registered: %v)", profileName, names)
		}
		return cfg, nil
	}
	if rawList == "" {
		return nil, nil
	}
	out := make(map[model.WorkflowPhase]int)
	for _, kv := range strings.Split(rawList, ",") {
		kv = strings.TrimSpace(kv)
		if kv == "" {
			continue
		}
		i := strings.IndexByte(kv, '=')
		if i < 0 {
			return nil, fmt.Errorf("--phase-wip: invalid syntax %q (want phase=cap)", kv)
		}
		key := strings.TrimSpace(kv[:i])
		val := strings.TrimSpace(kv[i+1:])
		phase, ok := parsePhaseName(key)
		if !ok {
			return nil, fmt.Errorf("--phase-wip: unknown phase %q (valid: Research, Sizing, Planning, Implement, Verify, CI/CD or CICD, Review)", key)
		}
		cap, err := strconv.Atoi(val)
		if err != nil {
			return nil, fmt.Errorf("--phase-wip: invalid integer %q for phase %q", val, key)
		}
		out[phase] = cap
	}
	return out, nil
}

// parsePhaseName maps an operator-supplied phase string to WorkflowPhase.
// Case-insensitive; accepts both "CI/CD" and "CICD" for the CI/CD phase.
// Mirrors api.parsePhaseName so CLI + REST share the same vocabulary.
func parsePhaseName(s string) (model.WorkflowPhase, bool) {
	switch strings.ToUpper(s) {
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

// phaseWIPConfigStringKeys converts the domain-typed cap config into the
// string-keyed wire form Client.CreateSimulation expects.
func phaseWIPConfigStringKeys(cfg map[model.WorkflowPhase]int) map[string]int {
	if len(cfg) == 0 {
		return nil
	}
	out := make(map[string]int, len(cfg))
	for k, v := range cfg {
		out[k.String()] = v
	}
	return out
}
