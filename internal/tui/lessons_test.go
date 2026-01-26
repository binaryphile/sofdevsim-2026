package tui

import (
	"encoding/json"
	"testing"
)

func TestHasQueueImbalanceFromTickets(t *testing.T) {
	tests := []struct {
		name    string
		tickets []TicketState
		want    bool
	}{
		{"empty = false", nil, false},
		{"single ticket = false", []TicketState{{Phase: "implement"}}, false},
		{"balanced 2 phases = false", []TicketState{
			{Phase: "implement"}, {Phase: "verify"},
		}, false},
		{"imbalanced 3 phases = true", []TicketState{
			{Phase: "implement"}, {Phase: "implement"}, {Phase: "implement"},
			{Phase: "implement"}, {Phase: "implement"}, // 5 in implement
			{Phase: "verify"},                          // 1 in verify
			{Phase: "cicd"},                            // 1 in cicd
		}, true}, // sum=7, phases=3, avg=2.33, implement(5) > 2×2.33=4.66 ✓
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasQueueImbalanceFromTickets(tt.tickets); got != tt.want {
				t.Errorf("HasQueueImbalanceFromTickets() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasHighChildVarianceFromTickets(t *testing.T) {
	tests := []struct {
		name    string
		tickets []TicketState
		want    bool
	}{
		{"no decomposed = false", []TicketState{{ID: "1"}}, false},
		{"children low variance = false", []TicketState{
			{ID: "parent", ChildIDs: []string{"c1", "c2"}},
			{ID: "c1", EstimatedDays: 2, ActualDays: 2.2},
			{ID: "c2", EstimatedDays: 2, ActualDays: 2.4},
		}, false},
		{"children high variance = true", []TicketState{
			{ID: "parent", ChildIDs: []string{"c1"}},
			{ID: "c1", EstimatedDays: 2, ActualDays: 3.0},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasHighChildVarianceFromTickets(tt.tickets); got != tt.want {
				t.Errorf("HasHighChildVarianceFromTickets() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestClientModeTriggers_JSONRoundTrip verifies triggers work with JSON-deserialized TicketState.
// This catches field naming mismatches between api and tui packages.
func TestClientModeTriggers_JSONRoundTrip(t *testing.T) {
	// Simulate JSON from API (what client receives)
	// Queue math: 5 implement + 1 verify + 1 cicd = 7 total, 3 phases
	// avg = 7/3 = 2.33, 2*avg = 4.66, implement(5) > 4.66 ✓
	jsonData := `{
		"completedTickets": [
			{"id": "parent", "childIds": ["c1"], "estimatedDays": 2, "actualDays": 2},
			{"id": "c1", "estimatedDays": 2, "actualDays": 3.0}
		],
		"activeTickets": [
			{"id": "t1", "phase": "implement"},
			{"id": "t2", "phase": "implement"},
			{"id": "t3", "phase": "implement"},
			{"id": "t4", "phase": "implement"},
			{"id": "t5", "phase": "implement"},
			{"id": "t6", "phase": "verify"},
			{"id": "t7", "phase": "cicd"}
		]
	}`

	var state struct {
		CompletedTickets []TicketState `json:"completedTickets"`
		ActiveTickets    []TicketState `json:"activeTickets"`
	}
	if err := json.Unmarshal([]byte(jsonData), &state); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Verify triggers detect correctly from deserialized data
	if !HasHighChildVarianceFromTickets(state.CompletedTickets) {
		t.Error("Expected high child variance from JSON data (3.0/2.0 = 1.5 > 1.3)")
	}
	if !HasQueueImbalanceFromTickets(state.ActiveTickets) {
		t.Error("Expected queue imbalance from JSON data (implement:5, verify:1, cicd:1)")
	}
}
