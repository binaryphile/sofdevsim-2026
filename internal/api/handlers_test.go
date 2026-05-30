package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/binaryphile/fluentfp/must"
	"github.com/binaryphile/fluentfp/rslt"
	"github.com/binaryphile/fluentfp/web"
	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/metrics"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
	"github.com/binaryphile/sofdevsim-2026/internal/registry"
)

// Per-handler rslt.Result-level tests for the 13 closure-based handlers
// per /grade R1 plan §criterion 6. Each test constructs a request via
// httptest.NewRequest, calls the handler closure directly, and asserts on
// the returned rslt.Result[web.Response] — independent of http.ResponseWriter
// + middleware. Khorikov Algorithm-tier (pure function of req → result given
// registry capture).
//
// These tests complement (don't replace) the integration tests in
// api_test.go which exercise the full HTTP boundary including middleware.

// newTestRegistry constructs a SimRegistry with one pre-populated simulation
// (id = "sim-42") for handlers that need an existing simulation.
func newTestRegistry(t *testing.T) SimRegistry {
	t.Helper()
	reg := NewSimRegistry()
	sim := model.NewSimulation("sim-42", model.PolicyDORAStrict, 42)
	tracker := metrics.NewTracker()
	eng := engine.NewEngineWithStore(sim.Seed, reg.Store())
	eng = must.Get(eng.EmitCreated(sim.ID, 0, events.SimConfig{
		TeamSize: 1, SprintLength: sim.SprintLength, Seed: sim.Seed, Policy: model.PolicyDORAStrict,
	}))
	eng = must.Get(eng.AddDeveloper("dev-1", "Alice", 1.0))
	reg.SetInstance("sim-42", registry.SimInstance{Sim: sim, Engine: eng, Tracker: tracker})
	return reg
}

// pathReq builds a request with a {id} path value pre-bound.
func pathReq(method, path, id string, body []byte) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.SetPathValue("id", id)
	return req
}

// unwrapOk asserts the result is Ok and returns the Response, or fatal.
func unwrapOk(t *testing.T, r rslt.Result[web.Response]) web.Response {
	t.Helper()
	resp, err := r.Unpack()
	if err != nil {
		t.Fatalf("expected Ok, got Err: %v", err)
	}
	return resp
}

// unwrapErr asserts the result is Err and returns *web.Error, or fatal.
func unwrapErr(t *testing.T, r rslt.Result[web.Response]) *web.Error {
	t.Helper()
	_, err := r.Unpack()
	if err == nil {
		t.Fatal("expected Err, got Ok")
	}
	we, ok := err.(*web.Error)
	if !ok {
		t.Fatalf("expected *web.Error, got %T: %v", err, err)
	}
	return we
}

func TestHandleEntryPoint(t *testing.T) {
	reg := newTestRegistry(t)
	resp := unwrapOk(t, handleEntryPoint(reg)(httptest.NewRequest("GET", "/", nil)))
	if resp.Status != http.StatusOK {
		t.Errorf("Status=%d, want 200", resp.Status)
	}
	body, ok := resp.Body.(EntryPointResponse)
	if !ok {
		t.Fatalf("Body type=%T, want EntryPointResponse", resp.Body)
	}
	if body.Links["simulations"] == "" {
		t.Error("missing simulations link")
	}
}

func TestHandleListSimulations(t *testing.T) {
	reg := newTestRegistry(t)
	resp := unwrapOk(t, handleListSimulations(reg)(httptest.NewRequest("GET", "/simulations", nil)))
	if resp.Status != http.StatusOK {
		t.Errorf("Status=%d, want 200", resp.Status)
	}
	body, ok := resp.Body.(SimulationListResponse)
	if !ok {
		t.Fatalf("Body type=%T", resp.Body)
	}
	if len(body.Simulations) != 1 {
		t.Errorf("len(Simulations)=%d, want 1", len(body.Simulations))
	}
}

func TestHandleGetSimulation(t *testing.T) {
	reg := newTestRegistry(t)
	t.Run("found", func(t *testing.T) {
		resp := unwrapOk(t, handleGetSimulation(reg)(pathReq("GET", "/simulations/sim-42", "sim-42", nil)))
		if resp.Status != http.StatusOK {
			t.Errorf("Status=%d, want 200", resp.Status)
		}
	})
	t.Run("not found", func(t *testing.T) {
		we := unwrapErr(t, handleGetSimulation(reg)(pathReq("GET", "/simulations/missing", "missing", nil)))
		if we.Status != http.StatusNotFound {
			t.Errorf("Status=%d, want 404", we.Status)
		}
	})
}

func TestHandleCreateSimulation(t *testing.T) {
	t.Run("invalid policy returns 400", func(t *testing.T) {
		reg := newTestRegistry(t)
		body := must.Get(json.Marshal(CreateSimulationRequest{Seed: 99, Policy: "bogus-policy"}))
		we := unwrapErr(t, handleCreateSimulation(reg)(httptest.NewRequest("POST", "/simulations", bytes.NewReader(body))))
		if we.Status != http.StatusBadRequest {
			t.Errorf("Status=%d, want 400", we.Status)
		}
	})
	t.Run("success returns 201", func(t *testing.T) {
		reg := NewSimRegistry()
		body := must.Get(json.Marshal(CreateSimulationRequest{Seed: 123}))
		resp := unwrapOk(t, handleCreateSimulation(reg)(httptest.NewRequest("POST", "/simulations", bytes.NewReader(body))))
		if resp.Status != http.StatusCreated {
			t.Errorf("Status=%d, want 201", resp.Status)
		}
	})
}

func TestHandleStartSprint(t *testing.T) {
	reg := newTestRegistry(t)
	t.Run("not found returns 404", func(t *testing.T) {
		we := unwrapErr(t, handleStartSprint(reg)(pathReq("POST", "/simulations/missing/sprints", "missing", nil)))
		if we.Status != http.StatusNotFound {
			t.Errorf("Status=%d, want 404", we.Status)
		}
	})
}

func TestHandleTick(t *testing.T) {
	reg := newTestRegistry(t)
	t.Run("no active sprint returns 409", func(t *testing.T) {
		we := unwrapErr(t, handleTick(reg)(pathReq("POST", "/simulations/sim-42/tick", "sim-42", nil)))
		if we.Status != http.StatusConflict {
			t.Errorf("Status=%d, want 409", we.Status)
		}
	})
}

func TestHandleAssignTicket(t *testing.T) {
	reg := newTestRegistry(t)
	t.Run("not found returns 404", func(t *testing.T) {
		we := unwrapErr(t, handleAssignTicket(reg)(pathReq("POST", "/simulations/missing/assignments", "missing", []byte(`{}`))))
		if we.Status != http.StatusNotFound {
			t.Errorf("Status=%d, want 404", we.Status)
		}
	})
}

func TestHandleDecompose(t *testing.T) {
	reg := newTestRegistry(t)
	t.Run("not found returns 404", func(t *testing.T) {
		we := unwrapErr(t, handleDecompose(reg)(pathReq("POST", "/simulations/missing/decompose", "missing", []byte(`{"ticketId":"x"}`))))
		if we.Status != http.StatusNotFound {
			t.Errorf("Status=%d, want 404", we.Status)
		}
	})
}

func TestHandleUpdateSimulation(t *testing.T) {
	reg := newTestRegistry(t)
	t.Run("invalid policy returns 400", func(t *testing.T) {
		body := must.Get(json.Marshal(UpdateSimulationRequest{Policy: "bogus"}))
		we := unwrapErr(t, handleUpdateSimulation(reg)(pathReq("PATCH", "/simulations/sim-42", "sim-42", body)))
		if we.Status != http.StatusBadRequest {
			t.Errorf("Status=%d, want 400", we.Status)
		}
	})
}

func TestHandleSpendInvestment(t *testing.T) {
	reg := newTestRegistry(t)
	t.Run("not found returns 404", func(t *testing.T) {
		we := unwrapErr(t, handleSpendInvestment(reg)(pathReq("POST", "/simulations/missing/investments", "missing", []byte(`{"option":"hire"}`))))
		if we.Status != http.StatusNotFound {
			t.Errorf("Status=%d, want 404", we.Status)
		}
	})
	t.Run("invalid option returns 422", func(t *testing.T) {
		body := []byte(`{"option":"bogus-option"}`)
		we := unwrapErr(t, handleSpendInvestment(reg)(pathReq("POST", "/simulations/sim-42/investments", "sim-42", body)))
		if we.Status != http.StatusUnprocessableEntity {
			t.Errorf("Status=%d, want 422", we.Status)
		}
	})
}

func TestHandleGetLessons(t *testing.T) {
	reg := newTestRegistry(t)
	t.Run("not found returns 404", func(t *testing.T) {
		we := unwrapErr(t, handleGetLessons(reg)(pathReq("GET", "/simulations/missing/lessons", "missing", nil)))
		if we.Status != http.StatusNotFound {
			t.Errorf("Status=%d, want 404", we.Status)
		}
	})
	t.Run("found returns 200", func(t *testing.T) {
		resp := unwrapOk(t, handleGetLessons(reg)(pathReq("GET", "/simulations/sim-42/lessons", "sim-42", nil)))
		if resp.Status != http.StatusOK {
			t.Errorf("Status=%d, want 200", resp.Status)
		}
	})
}

func TestHandleGetOffice(t *testing.T) {
	reg := newTestRegistry(t)
	t.Run("found returns 200", func(t *testing.T) {
		resp := unwrapOk(t, handleGetOffice(reg)(pathReq("GET", "/simulations/sim-42/office", "sim-42", nil)))
		if resp.Status != http.StatusOK {
			t.Errorf("Status=%d, want 200", resp.Status)
		}
	})
}

func TestHandleCompare(t *testing.T) {
	reg := newTestRegistry(t)
	t.Run("invalid sprints returns 400", func(t *testing.T) {
		body := []byte(`{"seed":42,"sprints":-1}`)
		we := unwrapErr(t, handleCompare(reg)(httptest.NewRequest("POST", "/comparisons", bytes.NewReader(body))))
		if we.Status != http.StatusBadRequest {
			t.Errorf("Status=%d, want 400", we.Status)
		}
	})
}
