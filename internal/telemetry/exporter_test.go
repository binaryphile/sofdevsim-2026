package telemetry

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSetup_Prometheus(t *testing.T) {
	ctx := context.Background()
	tel, err := Setup(ctx, Config{Prometheus: true})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer tel.Shutdown(ctx)

	if tel.Exporter == nil {
		t.Fatal("Exporter should not be nil")
	}
	if tel.PromHandler == nil {
		t.Fatal("PromHandler should not be nil when Prometheus enabled")
	}
}

func TestExporter_SnapshotObservation(t *testing.T) {
	ctx := context.Background()
	tel, err := Setup(ctx, Config{Prometheus: true})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer tel.Shutdown(ctx)

	// Set a snapshot with constraint data
	tel.Exporter.SetSnapshot(&Snapshot{
		SimID:                "test-42",
		SimDay:               10,
		Active:               true,
		ConstraintPhase:      "Review",
		ConstraintConfidence: 0.75,
		BufferDepth:          3,
		DownstreamWIP:        6,
	})

	// Scrape metrics
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	tel.PromHandler.ServeHTTP(w, req)

	body, _ := io.ReadAll(w.Body)
	metrics := string(body)

	// Verify key metrics are present
	checks := []string{
		"sofdevsim_sim_day",
		"sofdevsim_sim_active",
		"sofdevsim_constraint_active",
		"sofdevsim_constraint_confidence",
		"sofdevsim_constraint_buffer_depth",
		"sofdevsim_downstream_wip",
		`sim_id="test-42"`,
		`phase="Review"`,
	}
	for _, check := range checks {
		if !strings.Contains(metrics, check) {
			t.Errorf("metrics output missing %q", check)
		}
	}
}

func TestExporter_InactiveReturnsNothing(t *testing.T) {
	ctx := context.Background()
	tel, err := Setup(ctx, Config{Prometheus: true})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer tel.Shutdown(ctx)

	// No snapshot set — observables should produce nothing
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	tel.PromHandler.ServeHTTP(w, req)

	body, _ := io.ReadAll(w.Body)
	metrics := string(body)

	// Should NOT contain sim-specific metrics (no snapshot → no observations)
	if strings.Contains(metrics, "sofdevsim_sim_day") {
		t.Error("inactive exporter should not produce sofdevsim_sim_day")
	}
}

func TestExporter_EventRecording(t *testing.T) {
	ctx := context.Background()
	tel, err := Setup(ctx, Config{Prometheus: true})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer tel.Shutdown(ctx)

	// Record events
	tel.Exporter.RecordDeploy(ctx, "test-42")
	tel.Exporter.RecordDeploy(ctx, "test-42")
	tel.Exporter.RecordFailedDeploy(ctx, "test-42")
	tel.Exporter.RecordLeadTime(ctx, "test-42", 5.5)
	tel.Exporter.RecordDwellTime(ctx, "test-42", "Review", 3.0)

	// Scrape
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	tel.PromHandler.ServeHTTP(w, req)

	body, _ := io.ReadAll(w.Body)
	metrics := string(body)

	checks := []string{
		"sofdevsim_deploy_total",
		"sofdevsim_deploy_failed_total",
		"sofdevsim_lead_time",
		"sofdevsim_phase_dwell_time",
	}
	for _, check := range checks {
		if !strings.Contains(metrics, check) {
			t.Errorf("metrics output missing %q", check)
		}
	}
}
