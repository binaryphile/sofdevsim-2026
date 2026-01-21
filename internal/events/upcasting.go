package events

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
func newUpcaster() Upcaster {
	return Upcaster{
		transforms: map[string]func(Event) Event{
			// Add transforms here as schema evolves:
			// "TicketAssignedV1": upcastTicketAssignedV1ToV2,
		},
	}
}

// NewUpcasterWithTransforms creates an Upcaster with the given transforms.
// Used for testing - allows injection of custom transforms.
func NewUpcasterWithTransforms(transforms map[string]func(Event) Event) Upcaster {
	return Upcaster{transforms: transforms}
}

// Apply transforms event if upcast exists, otherwise returns unchanged.
// Value receiver - safe for concurrent use.
func (u Upcaster) Apply(evt Event) Event {
	if fn, ok := u.transforms[evt.EventType()]; ok {
		return fn(evt)
	}
	return evt
}
