package events

import "fmt"

// Upcaster transforms old event versions to current schema.
// Immutable after initialization - safe for concurrent use.
// Value receiver: enables clean composition, no nil receiver panics.
type Upcaster struct {
	transforms map[string]func(Event) Event
}

// DefaultUpcaster is the package-level upcaster with registered transforms.
// Initialized once at startup, never mutated after.
var DefaultUpcaster = newUpcaster()

// newUpcaster creates an Upcaster with all registered transforms.
// Add transforms here as schema evolves.
// Key format: "EventType:vN" (e.g., "TicketAssigned:v1")
func newUpcaster() Upcaster {
	return Upcaster{
		transforms: map[string]func(Event) Event{
			// Add transforms here as schema evolves:
			// "TicketAssigned:v1": upcastTicketAssignedV1ToV2,
		},
	}
}

// NewUpcasterWithTransforms creates an Upcaster with the given transforms.
// Used for testing - allows injection of custom transforms.
func NewUpcasterWithTransforms(transforms map[string]func(Event) Event) Upcaster {
	return Upcaster{transforms: transforms}
}

// Apply transforms event if upcast exists, otherwise returns unchanged.
// Uses "Type:vN" key format. Loops until no transform matches (transitive).
// Panics on cycle detection (per CQRS Guide §11: version chains must be DAG).
// Value receiver - safe for concurrent use.
func (u Upcaster) Apply(evt Event) Event {
	seen := make(map[string]bool)
	for {
		key := fmt.Sprintf("%s:v%d", evt.EventType(), evt.EventVersion())
		if seen[key] {
			panic(fmt.Sprintf("upcaster cycle detected: %s", key))
		}
		seen[key] = true

		fn, ok := u.transforms[key]
		if !ok {
			return evt
		}
		evt = fn(evt)
	}
}
