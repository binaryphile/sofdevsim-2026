package events

import (
	"fmt"
	"sync"

	"github.com/binaryphile/fluentfp/slice"
)

// Store defines the interface for event storage and subscription.
type Store interface {
	Append(simID string, expectedVersion int, events ...Event) error
	Replay(simID string) []Event
	Subscribe(simID string) <-chan Event
	Unsubscribe(simID string, ch <-chan Event)
	EventCount(simID string) int
	Close()
}

// MemoryStore is an in-memory implementation of Store.
// Uses pointer receiver because it contains sync.RWMutex.
type MemoryStore struct {
	mu          sync.RWMutex
	events      map[string][]Event
	subscribers map[string][]chan Event
	upcaster    Upcaster
}

// NewMemoryStore creates a new in-memory event store with DefaultUpcaster.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		events:      make(map[string][]Event),
		subscribers: make(map[string][]chan Event),
		upcaster:    DefaultUpcaster,
	}
}

// NewMemoryStoreWithUpcaster creates a new in-memory event store with a custom Upcaster.
// Used for testing - allows injection of transforms.
func NewMemoryStoreWithUpcaster(upcaster Upcaster) *MemoryStore {
	return &MemoryStore{
		events:      make(map[string][]Event),
		subscribers: make(map[string][]chan Event),
		upcaster:    upcaster,
	}
}

// Append adds events to a simulation's event stream.
// Uses optimistic concurrency: fails if expectedVersion doesn't match current version.
func (m *MemoryStore) Append(simID string, expectedVersion int, events ...Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Optimistic concurrency check
	currentVersion := len(m.events[simID])
	if expectedVersion != currentVersion {
		return fmt.Errorf("concurrency conflict: expected version %d, got %d", expectedVersion, currentVersion)
	}

	m.events[simID] = append(m.events[simID], events...)

	// Notify subscribers with upcasts applied
	for _, ch := range m.subscribers[simID] {
		for _, e := range events {
			upcasted := m.upcaster.Apply(e)
			select {
			case ch <- upcasted:
			default:
				// Non-blocking send - subscriber slow
			}
		}
	}

	return nil
}

// Replay returns all events for a simulation with upcasts applied.
// Returns a copy to prevent external modification.
func (m *MemoryStore) Replay(simID string) []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	original := m.events[simID]
	if original == nil {
		return nil
	}

	// Apply upcasts and return copy (per Go Dev Guide §9)
	return slice.From(original).Transform(m.upcaster.Apply)
}

// Subscribe returns a channel that receives new events for a simulation.
func (m *MemoryStore) Subscribe(simID string) <-chan Event {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan Event, 100)
	m.subscribers[simID] = append(m.subscribers[simID], ch)
	return ch
}

// Unsubscribe removes a subscription and closes its channel.
func (m *MemoryStore) Unsubscribe(simID string, ch <-chan Event) {
	m.mu.Lock()
	defer m.mu.Unlock()

	subs := m.subscribers[simID]
	for i, sub := range subs {
		if sub == ch {
			close(sub)
			m.subscribers[simID] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}

// EventCount returns the number of events for a simulation.
func (m *MemoryStore) EventCount(simID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.events[simID])
}

// Close closes all subscriber channels.
func (m *MemoryStore) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, subs := range m.subscribers {
		for _, ch := range subs {
			close(ch)
		}
	}
	m.subscribers = make(map[string][]chan Event)
}
