package events

import (
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

func TestMemoryStore_AppendAndReplay(t *testing.T) {
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
				err := store.Append(tt.simID, tt.events...)
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

	// Append to two different simulations
	store.Append("sim-1", testEvent{simID: "sim-1", name: "Event1"})
	store.Append("sim-2", testEvent{simID: "sim-2", name: "Event2"})
	store.Append("sim-1", testEvent{simID: "sim-1", name: "Event3"})

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

func TestMemoryStore_Subscribe(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ch := store.Subscribe("sim-1")

	// Append events after subscribing
	event := testEvent{simID: "sim-1", name: "NewEvent"}
	store.Append("sim-1", event)

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
	store.Append("sim-2", testEvent{simID: "sim-2", name: "OtherEvent"})

	// Should NOT receive event
	select {
	case e := <-ch:
		t.Errorf("received unexpected event: %s", e.EventType())
	case <-time.After(50 * time.Millisecond):
		// Expected - no event received
	}
}

func TestMemoryStore_Unsubscribe(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ch := store.Subscribe("sim-1")
	store.Unsubscribe("sim-1", ch)

	// Append after unsubscribe
	store.Append("sim-1", testEvent{simID: "sim-1", name: "Event"})

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

	store.Append("sim-1", testEvent{simID: "sim-1", name: "Event1"})

	// Get replay and modify it
	events1 := store.Replay("sim-1")
	events1 = append(events1, testEvent{simID: "sim-1", name: "Fake"})

	// Second replay should be unaffected
	events2 := store.Replay("sim-1")

	if len(events2) != 1 {
		t.Errorf("Replay() should return copy, got %d events after modification", len(events2))
	}
}

func TestMemoryStore_EventCount(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	if got := store.EventCount("sim-1"); got != 0 {
		t.Errorf("EventCount() = %d for empty store, want 0", got)
	}

	store.Append("sim-1", testEvent{simID: "sim-1", name: "E1"})
	store.Append("sim-1", testEvent{simID: "sim-1", name: "E2"})

	if got := store.EventCount("sim-1"); got != 2 {
		t.Errorf("EventCount() = %d, want 2", got)
	}
}

// cmp is used in other tests - reference it here to avoid unused import
var _ = cmp.Diff
