// Command sofdevsim-batch runs N independent simulations from a
// declarative JSON config, producing per-run CSV bundles + a runs.csv
// index + an experiment.json provenance file suitable for downstream
// R/Python analysis. See docs/use-cases.md §UC41 and docs/design.md
// §Batch CLI (UC41). Cycle #21831.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/binaryphile/sofdevsim-2026/internal/batch"
)

func main() {
	configPath := flag.String("config", "", "path to JSON config file (required)")
	outDir := flag.String("out", "./out", "output directory")
	flag.Parse()

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "sofdevsim-batch: -config <path> is required")
		flag.Usage()
		os.Exit(1)
	}

	cfg, err := batch.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sofdevsim-batch: %v\n", err)
		os.Exit(1)
	}

	results, err := batch.NewRunner().Run(cfg, *outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sofdevsim-batch: %v\n", err)
		os.Exit(1)
	}

	succeeded, failed := 0, 0
	for _, r := range results.Runs {
		switch r.Status {
		case "succeeded":
			succeeded++
		case "failed":
			failed++
			// Per-run failure diagnostics go to STDERR so the STDOUT
			// summary stays a single machine-parseable line.
			fmt.Fprintf(os.Stderr, "  run-%d (seed=%d): %s\n", r.Index, r.Seed, r.Error)
		}
	}

	// Machine-parseable summary on STDOUT (pipe-able for CI / harness).
	fmt.Printf("OK runs=%d succeeded=%d failed=%d outDir=%s\n",
		len(results.Runs), succeeded, failed, *outDir)
}
