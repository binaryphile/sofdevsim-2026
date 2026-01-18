package events

import (
	"testing"
	"time"
)

// mockState tracks how many events were applied.
type mockState struct {
	appliedCount int
	lastEvent    Event
}

// mockProjection is a test projection that counts applications.
type mockProjection struct {
	state mockState
}

func (p *mockProjection) Apply(e Event) {
	p.state.appliedCount++
	p.state.lastEvent = e
}

func (p *mockProjection) State() mockState {
	return p.state
}

func TestProjection_ApplyFromStore(t *testing.T) {
	store := NewMemoryStore()

	// Add events to store
	store.Append("sim-1",
		testEvent{simID: "sim-1", name: "Event1"},
		testEvent{simID: "sim-1", name: "Event2"},
		testEvent{simID: "sim-1", name: "Event3"},
	)

	// Create projection and replay
	proj := &mockProjection{}
	events := store.Replay("sim-1")
	for _, e := range events {
		proj.Apply(e)
	}

	if proj.state.appliedCount != 3 {
		t.Errorf("Applied %d events, want 3", proj.state.appliedCount)
	}

	if proj.state.lastEvent.EventType() != "Event3" {
		t.Errorf("Last event = %s, want Event3", proj.state.lastEvent.EventType())
	}
}

func TestProjection_SubscribeAndApply(t *testing.T) {
	store := NewMemoryStore()
	proj := &mockProjection{}

	// Subscribe before appending
	ch := store.Subscribe("sim-1")

	// Start goroutine to apply events from subscription
	done := make(chan struct{})
	go func() {
		for e := range ch {
			proj.Apply(e)
		}
		close(done)
	}()

	// Append events
	store.Append("sim-1", testEvent{simID: "sim-1", name: "Live1"})
	store.Append("sim-1", testEvent{simID: "sim-1", name: "Live2"})

	// Give time for events to be processed
	time.Sleep(10 * time.Millisecond)

	// Unsubscribe to close channel and stop goroutine
	store.Unsubscribe("sim-1", ch)
	<-done

	if proj.state.appliedCount != 2 {
		t.Errorf("Applied %d events, want 2", proj.state.appliedCount)
	}
}

func TestProjection_ReplayThenSubscribe(t *testing.T) {
	store := NewMemoryStore()

	// Add historical events
	store.Append("sim-1",
		testEvent{simID: "sim-1", name: "Historical1"},
		testEvent{simID: "sim-1", name: "Historical2"},
	)

	proj := &mockProjection{}

	// Replay historical events
	for _, e := range store.Replay("sim-1") {
		proj.Apply(e)
	}

	if proj.state.appliedCount != 2 {
		t.Errorf("After replay: applied %d events, want 2", proj.state.appliedCount)
	}

	// Subscribe for new events
	ch := store.Subscribe("sim-1")
	done := make(chan struct{})
	go func() {
		for e := range ch {
			proj.Apply(e)
		}
		close(done)
	}()

	// Add new event
	store.Append("sim-1", testEvent{simID: "sim-1", name: "Live1"})

	time.Sleep(10 * time.Millisecond)
	store.Unsubscribe("sim-1", ch)
	<-done

	if proj.state.appliedCount != 3 {
		t.Errorf("After subscribe: applied %d events, want 3", proj.state.appliedCount)
	}
}
