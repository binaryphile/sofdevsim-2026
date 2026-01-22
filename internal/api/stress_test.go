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
	var successCount atomic.Int32

	// 10 concurrent tick requests (sprint is only 10 days)
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
			if resp.StatusCode == 200 {
				successCount.Add(1)
			}
		}()
	}
	wg.Wait()

	if successCount.Load() != 10 {
		t.Errorf("Expected 10 successful ticks, got %d", successCount.Load())
	}

	// Check final state
	resp, _ = http.Get(server.URL + "/simulations/sim-12345")
	var result struct {
		Simulation struct {
			CurrentTick int `json:"currentTick"`
		} `json:"simulation"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()

	if result.Simulation.CurrentTick != 10 {
		t.Errorf("Final tick = %d, want 10 (race condition?)", result.Simulation.CurrentTick)
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
