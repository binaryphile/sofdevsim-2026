package events

import (
	"strings"
	"testing"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

func TestSimulationCreated(t *testing.T) {
	before := time.Now()
	e := NewSimulationCreated("sim-1", 0, SimConfig{
		TeamSize:     5,
		SprintLength: 10,
		Seed:         42,
	})
	after := time.Now()

	if e.SimulationID() != "sim-1" {
		t.Errorf("SimulationID() = %s, want sim-1", e.SimulationID())
	}
	if e.EventType() != "SimulationCreated" {
		t.Errorf("EventType() = %s, want SimulationCreated", e.EventType())
	}
	if !strings.HasPrefix(e.EventID(), "SimulationCreated-") {
		t.Errorf("EventID() = %s, want prefix SimulationCreated-", e.EventID())
	}
	if e.OccurrenceTime() != 0 {
		t.Errorf("OccurrenceTime() = %d, want 0", e.OccurrenceTime())
	}
	if e.DetectionTime().Before(before) || e.DetectionTime().After(after) {
		t.Errorf("DetectionTime() not in expected range")
	}
	if e.Config.TeamSize != 5 {
		t.Errorf("Config.TeamSize = %d, want 5", e.Config.TeamSize)
	}
}

func TestSprintStarted(t *testing.T) {
	e := NewSprintStarted("sim-1", 0, 1, 2.0)

	if e.SimulationID() != "sim-1" {
		t.Errorf("SimulationID() = %s, want sim-1", e.SimulationID())
	}
	if e.EventType() != "SprintStarted" {
		t.Errorf("EventType() = %s, want SprintStarted", e.EventType())
	}
	if !strings.HasPrefix(e.EventID(), "SprintStarted-") {
		t.Errorf("EventID() = %s, want prefix SprintStarted-", e.EventID())
	}
	if e.Number != 1 {
		t.Errorf("Number = %d, want 1", e.Number)
	}
	if e.StartTick != 0 {
		t.Errorf("StartTick = %d, want 0", e.StartTick)
	}
}

func TestTicked(t *testing.T) {
	e := NewTicked("sim-1", 42)

	if e.SimulationID() != "sim-1" {
		t.Errorf("SimulationID() = %s, want sim-1", e.SimulationID())
	}
	if e.EventType() != "Ticked" {
		t.Errorf("EventType() = %s, want Ticked", e.EventType())
	}
	if !strings.HasPrefix(e.EventID(), "Ticked-") {
		t.Errorf("EventID() = %s, want prefix Ticked-", e.EventID())
	}
	if e.Tick != 42 {
		t.Errorf("Tick = %d, want 42", e.Tick)
	}
	if e.OccurrenceTime() != 42 {
		t.Errorf("OccurrenceTime() = %d, want 42", e.OccurrenceTime())
	}
}

func TestTicketAssigned(t *testing.T) {
	e := NewTicketAssigned("sim-1", 10, "TKT-001", "DEV-001", time.Now())

	if e.SimulationID() != "sim-1" {
		t.Errorf("SimulationID() = %s, want sim-1", e.SimulationID())
	}
	if e.EventType() != "TicketAssigned" {
		t.Errorf("EventType() = %s, want TicketAssigned", e.EventType())
	}
	if !strings.HasPrefix(e.EventID(), "TicketAssigned-") {
		t.Errorf("EventID() = %s, want prefix TicketAssigned-", e.EventID())
	}
	if e.TicketID != "TKT-001" {
		t.Errorf("TicketID = %s, want TKT-001", e.TicketID)
	}
	if e.DeveloperID != "DEV-001" {
		t.Errorf("DeveloperID = %s, want DEV-001", e.DeveloperID)
	}
	if e.OccurrenceTime() != 10 {
		t.Errorf("OccurrenceTime() = %d, want 10", e.OccurrenceTime())
	}
}

func TestTicketCompleted(t *testing.T) {
	e := NewTicketCompleted("sim-1", 25, "TKT-001", "DEV-001", 5.0)

	if e.SimulationID() != "sim-1" {
		t.Errorf("SimulationID() = %s, want sim-1", e.SimulationID())
	}
	if e.EventType() != "TicketCompleted" {
		t.Errorf("EventType() = %s, want TicketCompleted", e.EventType())
	}
	if !strings.HasPrefix(e.EventID(), "TicketCompleted-") {
		t.Errorf("EventID() = %s, want prefix TicketCompleted-", e.EventID())
	}
}

func TestIncidentStarted(t *testing.T) {
	e := NewIncidentStarted("sim-1", 15, "INC-001", "DEV-001", "TKT-001", model.SeverityHigh)

	if e.SimulationID() != "sim-1" {
		t.Errorf("SimulationID() = %s, want sim-1", e.SimulationID())
	}
	if e.EventType() != "IncidentStarted" {
		t.Errorf("EventType() = %s, want IncidentStarted", e.EventType())
	}
	if !strings.HasPrefix(e.EventID(), "IncidentStarted-") {
		t.Errorf("EventID() = %s, want prefix IncidentStarted-", e.EventID())
	}
}

func TestIncidentResolved(t *testing.T) {
	e := NewIncidentResolved("sim-1", 20, "INC-001", "DEV-001")

	if e.SimulationID() != "sim-1" {
		t.Errorf("SimulationID() = %s, want sim-1", e.SimulationID())
	}
	if e.EventType() != "IncidentResolved" {
		t.Errorf("EventType() = %s, want IncidentResolved", e.EventType())
	}
	if !strings.HasPrefix(e.EventID(), "IncidentResolved-") {
		t.Errorf("EventID() = %s, want prefix IncidentResolved-", e.EventID())
	}
}

func TestSprintEnded(t *testing.T) {
	e := NewSprintEnded("sim-1", 10, 1)

	if e.SimulationID() != "sim-1" {
		t.Errorf("SimulationID() = %s, want sim-1", e.SimulationID())
	}
	if e.EventType() != "SprintEnded" {
		t.Errorf("EventType() = %s, want SprintEnded", e.EventType())
	}
	if !strings.HasPrefix(e.EventID(), "SprintEnded-") {
		t.Errorf("EventID() = %s, want prefix SprintEnded-", e.EventID())
	}
	if e.EndTick != 10 {
		t.Errorf("EndTick = %d, want 10", e.EndTick)
	}
}

// Test that all event types implement Event interface
func TestEventInterfaceCompliance(t *testing.T) {
	var events []Event = []Event{
		NewSimulationCreated("s", 0, SimConfig{}),
		NewSprintStarted("s", 0, 1, 2.0),
		NewTicked("s", 0),
		NewTicketAssigned("s", 0, "t", "d", time.Time{}),
		NewTicketCompleted("s", 0, "t", "d", 5.0),
		NewIncidentStarted("s", 0, "i", "d", "", model.SeverityLow),
		NewIncidentResolved("s", 0, "i", "d"),
		NewSprintEnded("s", 0, 1),
	}

	for _, e := range events {
		if e.SimulationID() != "s" {
			t.Errorf("%T.SimulationID() = %s, want s", e, e.SimulationID())
		}
		// Verify all interface methods exist and don't panic
		_ = e.EventID()
		_ = e.EventType()
		_ = e.OccurrenceTime()
		_ = e.DetectionTime()
		_ = e.CausedBy()
		_ = e.TraceID()
		_ = e.SpanID()
		_ = e.ParentSpanID()
	}
}

func TestWithTrace(t *testing.T) {
	e := NewTicketAssigned("sim-1", 10, "TKT-001", "DEV-001", time.Now()).
		WithTrace("trace-123", "span-456", "span-parent")

	if e.TraceID() != "trace-123" {
		t.Errorf("TraceID() = %s, want trace-123", e.TraceID())
	}
	if e.SpanID() != "span-456" {
		t.Errorf("SpanID() = %s, want span-456", e.SpanID())
	}
	if e.ParentSpanID() != "span-parent" {
		t.Errorf("ParentSpanID() = %s, want span-parent", e.ParentSpanID())
	}
}

func TestWithCausedBy(t *testing.T) {
	e := NewTicketCompleted("sim-1", 25, "TKT-001", "DEV-001", 5.0).
		WithCausedBy("TicketAssigned-1")

	if e.CausedBy() != "TicketAssigned-1" {
		t.Errorf("CausedBy() = %s, want TicketAssigned-1", e.CausedBy())
	}
}

func TestNextSpanID(t *testing.T) {
	id1 := NextSpanID()
	id2 := NextSpanID()

	if id1 == id2 {
		t.Errorf("NextSpanID() should generate unique IDs, got %s twice", id1)
	}
	if !strings.HasPrefix(id1, "span-") {
		t.Errorf("NextSpanID() = %s, want prefix span-", id1)
	}
}

func TestNextTraceID(t *testing.T) {
	id1 := NextTraceID()
	id2 := NextTraceID()

	if id1 == id2 {
		t.Errorf("NextTraceID() should generate unique IDs, got %s twice", id1)
	}
	if !strings.HasPrefix(id1, "trace-") {
		t.Errorf("NextTraceID() = %s, want prefix trace-", id1)
	}
}

// TestAllEventTypes_WithTrace verifies WithTrace works on all event types.
func TestEvents_TraceContextWorksOnAllTypes(t *testing.T) {
	tests := []struct {
		name  string
		event Event
	}{
		{"SimulationCreated", NewSimulationCreated("s", 0, SimConfig{}).WithTrace("t", "s", "p")},
		{"SprintStarted", NewSprintStarted("s", 0, 1, 2.0).WithTrace("t", "s", "p")},
		{"SprintEnded", NewSprintEnded("s", 0, 1).WithTrace("t", "s", "p")},
		{"Ticked", NewTicked("s", 0).WithTrace("t", "s", "p")},
		{"TicketAssigned", NewTicketAssigned("s", 0, "t", "d", time.Time{}).WithTrace("t", "s", "p")},
		{"TicketCompleted", NewTicketCompleted("s", 0, "t", "d", 5.0).WithTrace("t", "s", "p")},
		{"IncidentStarted", NewIncidentStarted("s", 0, "i", "d", "", model.SeverityLow).WithTrace("t", "s", "p")},
		{"IncidentResolved", NewIncidentResolved("s", 0, "i", "d").WithTrace("t", "s", "p")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.event.TraceID() != "t" {
				t.Errorf("TraceID() = %s, want t", tt.event.TraceID())
			}
			if tt.event.SpanID() != "s" {
				t.Errorf("SpanID() = %s, want s", tt.event.SpanID())
			}
			if tt.event.ParentSpanID() != "p" {
				t.Errorf("ParentSpanID() = %s, want p", tt.event.ParentSpanID())
			}
		})
	}
}

// TestAllEventTypes_WithCausedBy verifies WithCausedBy works on all event types.
func TestTraceContext_RootHasGeneratedIDs(t *testing.T) {
	tc := NewTraceContext()

	if tc.TraceID == "" {
		t.Error("TraceID should not be empty")
	}
	if tc.SpanID == "" {
		t.Error("SpanID should not be empty")
	}
	if tc.ParentSpanID != "" {
		t.Errorf("ParentSpanID should be empty for root, got %s", tc.ParentSpanID)
	}
	if tc.IsEmpty() {
		t.Error("New trace context should not be empty")
	}
}

func TestTraceContext_ChildMaintainsHierarchy(t *testing.T) {
	parent := NewTraceContext()
	child := parent.NewChildSpan()

	// Same trace ID
	if child.TraceID != parent.TraceID {
		t.Errorf("Child TraceID = %s, want %s", child.TraceID, parent.TraceID)
	}

	// Different span ID
	if child.SpanID == parent.SpanID {
		t.Errorf("Child SpanID should differ from parent")
	}

	// Parent reference
	if child.ParentSpanID != parent.SpanID {
		t.Errorf("Child ParentSpanID = %s, want %s", child.ParentSpanID, parent.SpanID)
	}
}

func TestTraceContext_EmptyVsInitialized(t *testing.T) {
	empty := TraceContext{}
	if !empty.IsEmpty() {
		t.Error("Zero TraceContext should be empty")
	}

	nonEmpty := NewTraceContext()
	if nonEmpty.IsEmpty() {
		t.Error("NewTraceContext should not be empty")
	}
}

func TestEvents_CausedByWorksOnAllTypes(t *testing.T) {
	tests := []struct {
		name  string
		event Event
	}{
		{"SimulationCreated", NewSimulationCreated("s", 0, SimConfig{}).WithCausedBy("cause-1")},
		{"SprintStarted", NewSprintStarted("s", 0, 1, 2.0).WithCausedBy("cause-1")},
		{"SprintEnded", NewSprintEnded("s", 0, 1).WithCausedBy("cause-1")},
		{"Ticked", NewTicked("s", 0).WithCausedBy("cause-1")},
		{"TicketAssigned", NewTicketAssigned("s", 0, "t", "d", time.Time{}).WithCausedBy("cause-1")},
		{"TicketCompleted", NewTicketCompleted("s", 0, "t", "d", 5.0).WithCausedBy("cause-1")},
		{"IncidentStarted", NewIncidentStarted("s", 0, "i", "d", "", model.SeverityLow).WithCausedBy("cause-1")},
		{"IncidentResolved", NewIncidentResolved("s", 0, "i", "d").WithCausedBy("cause-1")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.event.CausedBy() != "cause-1" {
				t.Errorf("CausedBy() = %s, want cause-1", tt.event.CausedBy())
			}
		})
	}
}

func TestApplyTrace(t *testing.T) {
	tc := TraceContext{
		TraceID:      "trace-test",
		SpanID:       "span-test",
		ParentSpanID: "parent-test",
	}

	// Test with TicketAssigned (representative event type)
	original := NewTicketAssigned("sim-1", 10, "TKT-001", "DEV-001", time.Now())
	result := ApplyTrace(original, tc)

	if result.TraceID() != "trace-test" {
		t.Errorf("TraceID() = %s, want trace-test", result.TraceID())
	}
	if result.SpanID() != "span-test" {
		t.Errorf("SpanID() = %s, want span-test", result.SpanID())
	}
	if result.ParentSpanID() != "parent-test" {
		t.Errorf("ParentSpanID() = %s, want parent-test", result.ParentSpanID())
	}

	// Verify original unchanged (value semantics)
	if original.TraceID() != "" {
		t.Errorf("Original should be unchanged, TraceID = %s", original.TraceID())
	}
}
