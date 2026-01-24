package events

import (
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// testEvent is a simple event for testing.
type testEvent struct {
	simID        string
	id           string
	name         string
	occurredAt   int
	detectedAt   time.Time
	causedBy     string
	traceID      string
	spanID       string
	parentSpanID string
}

func (e testEvent) SimulationID() string     { return e.simID }
func (e testEvent) EventID() string          { return e.id }
func (e testEvent) EventType() string        { return e.name }
func (e testEvent) OccurrenceTime() int      { return e.occurredAt }
func (e testEvent) DetectionTime() time.Time { return e.detectedAt }
func (e testEvent) CausedBy() string         { return e.causedBy }
func (e testEvent) TraceID() string          { return e.traceID }
func (e testEvent) SpanID() string           { return e.spanID }
func (e testEvent) ParentSpanID() string     { return e.parentSpanID }

func (e testEvent) withTrace(traceID, spanID, parentSpanID string) Event {
	e.traceID = traceID
	e.spanID = spanID
	e.parentSpanID = parentSpanID
	return e
}
func (e testEvent) EventVersion() int { return 1 }

func TestMemoryStore_AppendAndReplay_ReturnsStoredEvents(t *testing.T) {
	tests := []struct {
		name   string
		simID  string
		events []Event
		want   int
	}{
		{
			name:   "empty store returns nil",
			simID:  "sim-1",
			events: nil,
			want:   0,
		},
		{
			name:  "single event appended and replayed",
			simID: "sim-1",
			events: []Event{
				testEvent{simID: "sim-1", name: "Created"},
			},
			want: 1,
		},
		{
			name:  "multiple events preserve order",
			simID: "sim-1",
			events: []Event{
				testEvent{simID: "sim-1", name: "Event1"},
				testEvent{simID: "sim-1", name: "Event2"},
				testEvent{simID: "sim-1", name: "Event3"},
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMemoryStore()
			defer store.Close()

			if len(tt.events) > 0 {
				err := store.Append(tt.simID, 0, tt.events...)
				if err != nil {
					t.Fatalf("Append() error = %v", err)
				}
			}

			got := store.Replay(tt.simID)

			if len(got) != tt.want {
				t.Errorf("Replay() returned %d events, want %d", len(got), tt.want)
			}

			// Verify order preserved
			for i, e := range got {
				if e.EventType() != tt.events[i].EventType() {
					t.Errorf("Replay()[%d] = %s, want %s", i, e.EventType(), tt.events[i].EventType())
				}
			}
		})
	}
}

func TestMemoryStore_IsolatesSimulations(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	// Append to two different simulations (tracking versions independently)
	store.Append("sim-1", 0, testEvent{simID: "sim-1", name: "Event1"})
	store.Append("sim-2", 0, testEvent{simID: "sim-2", name: "Event2"})
	store.Append("sim-1", 1, testEvent{simID: "sim-1", name: "Event3"})

	// Replay should be isolated
	sim1Events := store.Replay("sim-1")
	sim2Events := store.Replay("sim-2")

	if len(sim1Events) != 2 {
		t.Errorf("sim-1 has %d events, want 2", len(sim1Events))
	}
	if len(sim2Events) != 1 {
		t.Errorf("sim-2 has %d events, want 1", len(sim2Events))
	}
}

func TestMemoryStore_Subscribe_DeliversNewEvents(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ch := store.Subscribe("sim-1")

	// Append events after subscribing
	event := testEvent{simID: "sim-1", name: "NewEvent"}
	store.Append("sim-1", 0, event)

	// Should receive event on channel
	select {
	case received := <-ch:
		if received.EventType() != "NewEvent" {
			t.Errorf("received event type = %s, want NewEvent", received.EventType())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for event")
	}
}

func TestMemoryStore_SubscribeDoesNotReceiveOtherSimulations(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ch := store.Subscribe("sim-1")

	// Append to different simulation
	store.Append("sim-2", 0, testEvent{simID: "sim-2", name: "OtherEvent"})

	// Should NOT receive event
	select {
	case e := <-ch:
		t.Errorf("received unexpected event: %s", e.EventType())
	case <-time.After(50 * time.Millisecond):
		// Expected - no event received
	}
}

func TestMemoryStore_Unsubscribe_StopsDelivery(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ch := store.Subscribe("sim-1")
	store.Unsubscribe("sim-1", ch)

	// Append after unsubscribe
	store.Append("sim-1", 0, testEvent{simID: "sim-1", name: "Event"})

	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed after unsubscribe")
		}
	case <-time.After(50 * time.Millisecond):
		t.Error("channel should be closed, not blocking")
	}
}

func TestMemoryStore_ReplayReturnsCopy(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	store.Append("sim-1", 0, testEvent{simID: "sim-1", name: "Event1"})

	// Get replay and modify it
	events1 := store.Replay("sim-1")
	events1 = append(events1, testEvent{simID: "sim-1", name: "Fake"})

	// Second replay should be unaffected
	events2 := store.Replay("sim-1")

	if len(events2) != 1 {
		t.Errorf("Replay() should return copy, got %d events after modification", len(events2))
	}
}

func TestMemoryStore_EventCount_ReturnsAccurateTotal(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	if got := store.EventCount("sim-1"); got != 0 {
		t.Errorf("EventCount() = %d for empty store, want 0", got)
	}

	store.Append("sim-1", 0, testEvent{simID: "sim-1", name: "E1"})
	store.Append("sim-1", 1, testEvent{simID: "sim-1", name: "E2"})

	if got := store.EventCount("sim-1"); got != 2 {
		t.Errorf("EventCount() = %d, want 2", got)
	}
}

// TestMemoryStore_Append_VersionConflicts tests optimistic concurrency control
// using table-driven tests for all version conflict scenarios.
func TestMemoryStore_Append_VersionConflicts(t *testing.T) {
	tests := []struct {
		name            string
		setup           func(*MemoryStore)
		simID           string
		expectedVersion int
		events          []Event
		wantErr         bool
		errContains     string
		wantCount       int // expected event count after append (if no error)
	}{
		{
			name:            "empty store accepts version 0",
			setup:           func(s *MemoryStore) {},
			simID:           "sim-1",
			expectedVersion: 0,
			events:          []Event{testEvent{simID: "sim-1", name: "E1"}},
			wantErr:         false,
			wantCount:       1,
		},
		{
			name:            "empty store rejects version 1",
			setup:           func(s *MemoryStore) {},
			simID:           "sim-1",
			expectedVersion: 1,
			events:          []Event{testEvent{simID: "sim-1", name: "E1"}},
			wantErr:         true,
			errContains:     "expected version 1",
		},
		{
			name: "after one event accepts version 1",
			setup: func(s *MemoryStore) {
				s.Append("sim-1", 0, testEvent{simID: "sim-1", name: "E0"})
			},
			simID:           "sim-1",
			expectedVersion: 1,
			events:          []Event{testEvent{simID: "sim-1", name: "E1"}},
			wantErr:         false,
			wantCount:       2,
		},
		{
			name: "after one event rejects version 0 (conflict)",
			setup: func(s *MemoryStore) {
				s.Append("sim-1", 0, testEvent{simID: "sim-1", name: "E0"})
			},
			simID:           "sim-1",
			expectedVersion: 0,
			events:          []Event{testEvent{simID: "sim-1", name: "E1"}},
			wantErr:         true,
			errContains:     "got 1",
		},
		{
			name: "multiple events increment version correctly",
			setup: func(s *MemoryStore) {
				s.Append("sim-1", 0,
					testEvent{simID: "sim-1", name: "E1"},
					testEvent{simID: "sim-1", name: "E2"},
					testEvent{simID: "sim-1", name: "E3"},
				)
			},
			simID:           "sim-1",
			expectedVersion: 3,
			events:          []Event{testEvent{simID: "sim-1", name: "E4"}},
			wantErr:         false,
			wantCount:       4,
		},
		{
			name: "error message includes both versions",
			setup: func(s *MemoryStore) {
				s.Append("sim-1", 0, testEvent{simID: "sim-1", name: "E0"})
			},
			simID:           "sim-1",
			expectedVersion: 5,
			events:          []Event{testEvent{simID: "sim-1", name: "E1"}},
			wantErr:         true,
			errContains:     "expected version 5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMemoryStore()
			defer store.Close()

			tt.setup(store)

			err := store.Append(tt.simID, tt.expectedVersion, tt.events...)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error should contain %q, got: %s", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if got := store.EventCount(tt.simID); got != tt.wantCount {
					t.Errorf("EventCount() = %d, want %d", got, tt.wantCount)
				}
			}
		})
	}
}

// contains checks if s contains substr (simple helper to avoid strings import).
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// cmp is used in other tests - reference it here to avoid unused import
var _ = cmp.Diff

func TestMemoryStore_Replay_AppliesUpcasts(t *testing.T) {
	// Create a transform that marks events as "upcasted" by changing the name
	// markUpcasted appends "-upcasted" to testEvent name.
	markUpcasted := func(evt Event) Event {
		te := evt.(testEvent)
		te.name = te.name + "-upcasted"
		return te
	}

	upcaster := NewUpcasterWithTransforms(map[string]func(Event) Event{
		"OriginalEvent:v1": markUpcasted,
	})

	store := NewMemoryStoreWithUpcaster(upcaster)
	defer store.Close()

	// Append original event
	store.Append("sim-1", 0, testEvent{simID: "sim-1", name: "OriginalEvent"})

	// Replay should apply upcast
	events := store.Replay("sim-1")

	if len(events) != 1 {
		t.Fatalf("Replay() returned %d events, want 1", len(events))
	}
	if events[0].EventType() != "OriginalEvent-upcasted" {
		t.Errorf("Upcast not applied: got %s, want OriginalEvent-upcasted", events[0].EventType())
	}
}

func TestMemoryStore_Subscribe_AppliesUpcasts(t *testing.T) {
	// Create a transform that marks events as "upcasted" by changing the name
	// markUpcasted appends "-upcasted" to testEvent name.
	markUpcasted := func(evt Event) Event {
		te := evt.(testEvent)
		te.name = te.name + "-upcasted"
		return te
	}

	upcaster := NewUpcasterWithTransforms(map[string]func(Event) Event{
		"LiveEvent:v1": markUpcasted,
	})

	store := NewMemoryStoreWithUpcaster(upcaster)
	defer store.Close()

	ch := store.Subscribe("sim-1")

	// Append event after subscribing
	store.Append("sim-1", 0, testEvent{simID: "sim-1", name: "LiveEvent"})

	// Should receive upcasted event
	select {
	case received := <-ch:
		if received.EventType() != "LiveEvent-upcasted" {
			t.Errorf("Upcast not applied to subscription: got %s, want LiveEvent-upcasted", received.EventType())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for event")
	}
}

// TestMemoryStore_ConcurrentAppend verifies that concurrent goroutines can
// safely append events without data corruption or panics.
// Per Khorikov: edge case test for concurrent access (regression protection).
func TestMemoryStore_ConcurrentAppend(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	simID := "stress-test"

	// Initialize with first event (version 0 -> 1)
	err := store.Append(simID, 0, testEvent{simID: simID, id: "init", name: "Init"})
	if err != nil {
		t.Fatalf("Initial append failed: %v", err)
	}

	// Launch goroutines that all try to append
	const goroutines = 10
	const attemptsPerGoroutine = 100

	var wg sync.WaitGroup
	var successCount, conflictCount int64
	var mu sync.Mutex

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < attemptsPerGoroutine; i++ {
				version := store.EventCount(simID)
				evt := testEvent{
					simID: simID,
					id:    "", // Empty ID to bypass idempotency
					name:  "Tick",
				}
				err := store.Append(simID, version, evt)

				mu.Lock()
				if err != nil {
					conflictCount++
				} else {
					successCount++
				}
				mu.Unlock()
			}
		}(g)
	}

	wg.Wait()

	// All attempts should either succeed or conflict (no panics, no data corruption)
	totalAttempts := int64(goroutines * attemptsPerGoroutine)
	if successCount+conflictCount != totalAttempts {
		t.Errorf("Missing attempts: success=%d, conflict=%d, total=%d",
			successCount, conflictCount, totalAttempts)
	}

	// Verify store is consistent
	finalCount := store.EventCount(simID)
	expectedCount := int(successCount) + 1 // +1 for initial event
	if finalCount != expectedCount {
		t.Errorf("EventCount = %d, want %d (successes + 1)", finalCount, expectedCount)
	}

	// Verify replay returns correct number of events
	events := store.Replay(simID)
	if len(events) != expectedCount {
		t.Errorf("Replay returned %d events, want %d", len(events), expectedCount)
	}
}
