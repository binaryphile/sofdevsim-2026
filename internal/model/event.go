package model

// Event represents something that happened during simulation
type Event struct {
	Type    EventType
	Message string
	Day     int
}

// NewEvent creates an event
func NewEvent(eventType EventType, message string, day int) Event {
	return Event{
		Type:    eventType,
		Message: message,
		Day:     day,
	}
}
