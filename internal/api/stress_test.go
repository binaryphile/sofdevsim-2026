package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/api"
)

func TestAPI_ConcurrentTicks(t *testing.T) {
	registry := api.NewSimRegistry()
	router := api.NewRouter(registry)
	server := httptest.NewServer(router)
	defer server.Close()

	// Create simulation
	body := bytes.NewBufferString(`{"seed":12345}`)
	resp, err := http.Post(server.URL+"/simulations", "application/json", body)
	if err != nil {
		t.Fatalf("Failed to create simulation: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	// Start sprint (required for tick to work)
	body = bytes.NewBufferString(`{}`)
	resp, err = http.Post(server.URL+"/simulations/sim-12345/sprints", "application/json", body)
	if err != nil {
		t.Fatalf("Failed to start sprint: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	var wg sync.WaitGroup
	var successCount, conflictCount atomic.Int32

	// 10 concurrent tick requests - with retry+409 some will fail under contention
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := bytes.NewBufferString(`{}`)
			resp, err := http.Post(server.URL+"/simulations/sim-12345/tick", "application/json", body)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			io.Copy(io.Discard, resp.Body)
			switch resp.StatusCode {
			case 200:
				successCount.Add(1)
			case 409:
				conflictCount.Add(1) // Expected under contention
			}
		}()
	}
	wg.Wait()

	t.Logf("Concurrent ticks: %d success, %d conflict", successCount.Load(), conflictCount.Load())

	// Under extreme contention, some conflicts are expected (retries exhausted)
	// Key invariant: all requests complete (no panics, no 500s)
	total := successCount.Load() + conflictCount.Load()
	if total != 10 {
		t.Errorf("Expected 10 total responses (success+conflict), got %d", total)
	}

	// At least some ticks should succeed
	if successCount.Load() == 0 {
		t.Error("Expected at least some successful ticks")
	}

	// Check final state - should equal success count (each success = 1 tick)
	resp, _ = http.Get(server.URL + "/simulations/sim-12345")
	var result struct {
		Simulation struct {
			CurrentTick int `json:"currentTick"`
		} `json:"simulation"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()

	// Final tick should equal number of successful operations
	if result.Simulation.CurrentTick != int(successCount.Load()) {
		t.Errorf("Final tick = %d, want %d (success count)", result.Simulation.CurrentTick, successCount.Load())
	}
}

func TestAPI_ConcurrentMixedOperations(t *testing.T) {
	registry := api.NewSimRegistry()
	router := api.NewRouter(registry)
	server := httptest.NewServer(router)
	defer server.Close()

	// Create simulation
	body := bytes.NewBufferString(`{"seed":99999}`)
	resp, _ := http.Post(server.URL+"/simulations", "application/json", body)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	// Start sprint
	body = bytes.NewBufferString(`{}`)
	resp, _ = http.Post(server.URL+"/simulations/sim-99999/sprints", "application/json", body)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	var wg sync.WaitGroup
	var readSuccess, writeSuccess atomic.Int32

	// Mixed read/write operations (10 iterations, sprint is 10 days)
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			resp, err := http.Get(server.URL + "/simulations/sim-99999")
			if err != nil {
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			readSuccess.Add(1)
		}()
		go func() {
			defer wg.Done()
			body := bytes.NewBufferString(`{}`)
			resp, err := http.Post(server.URL+"/simulations/sim-99999/tick", "application/json", body)
			if err != nil {
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			writeSuccess.Add(1)
		}()
	}
	wg.Wait()
	
	t.Logf("Mixed ops: %d reads, %d writes succeeded", readSuccess.Load(), writeSuccess.Load())

	if readSuccess.Load() < 8 || writeSuccess.Load() < 8 {
		t.Errorf("Too many failures: reads=%d, writes=%d", readSuccess.Load(), writeSuccess.Load())
	}
}
