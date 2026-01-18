package events

import "sync"

// Store defines the interface for event storage and subscription.
type Store interface {
	Append(simID string, events ...Event) error
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
}

// NewMemoryStore creates a new in-memory event store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		events:      make(map[string][]Event),
		subscribers: make(map[string][]chan Event),
	}
}

// Append adds events to a simulation's event stream.
func (m *MemoryStore) Append(simID string, events ...Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events[simID] = append(m.events[simID], events...)

	// Notify subscribers
	for _, ch := range m.subscribers[simID] {
		for _, e := range events {
			select {
			case ch <- e:
			default:
				// Non-blocking send - subscriber slow
			}
		}
	}

	return nil
}

// Replay returns all events for a simulation.
// Returns a copy to prevent external modification.
func (m *MemoryStore) Replay(simID string) []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	original := m.events[simID]
	if original == nil {
		return nil
	}

	// Return a copy
	result := make([]Event, len(original))
	copy(result, original)
	return result
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
