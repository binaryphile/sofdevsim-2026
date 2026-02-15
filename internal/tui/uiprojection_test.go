package tui

import "testing"

func TestUIProjection_InitialState(t *testing.T) {
	p := NewUIProjection()
	state := p.State()

	if state.CurrentView != ViewPlanning {
		t.Errorf("CurrentView = %v, want ViewPlanning", state.CurrentView)
	}
	if state.SelectedTicket != "" {
		t.Errorf("SelectedTicket = %q, want empty", state.SelectedTicket)
	}
	if state.LessonVisible {
		t.Error("LessonVisible = true, want false")
	}
	if state.ErrorMessage != "" {
		t.Errorf("ErrorMessage = %q, want empty", state.ErrorMessage)
	}
}

func TestUIProjection_ViewSwitching(t *testing.T) {
	p := NewUIProjection()
	p = p.Record(ViewSwitched{To: ViewExecution})

	state := p.State()
	if state.CurrentView != ViewExecution {
		t.Errorf("CurrentView = %v, want ViewExecution", state.CurrentView)
	}
}

func TestUIProjection_ViewSwitching_ClearsSelectionAndError(t *testing.T) {
	p := NewUIProjection()
	// Set up state with selection and error
	p = p.Record(TicketSelected{ID: "TKT-001"})
	p = p.Record(SprintStartAttempted{Outcome: Failed{Category: BusinessRule, Reason: "already active"}})

	// Verify state before switch
	state := p.State()
	if state.SelectedTicket != "TKT-001" {
		t.Errorf("SelectedTicket before switch = %q, want TKT-001", state.SelectedTicket)
	}
	if state.ErrorMessage == "" {
		t.Error("ErrorMessage before switch = empty, want error")
	}

	// Switch view
	p = p.Record(ViewSwitched{To: ViewMetrics})
	state = p.State()

	if state.SelectedTicket != "" {
		t.Errorf("SelectedTicket after switch = %q, want empty", state.SelectedTicket)
	}
	if state.ErrorMessage != "" {
		t.Errorf("ErrorMessage after switch = %q, want empty", state.ErrorMessage)
	}
}

func TestUIProjection_FailedAction_SetsError(t *testing.T) {
	tests := []struct {
		name  string
		event InputEvent
	}{
		{"SprintStartAttempted", SprintStartAttempted{Outcome: Failed{Category: BusinessRule, Reason: "sprint active"}}},
		{"TickAttempted", TickAttempted{Outcome: Failed{Category: BusinessRule, Reason: "no sprint"}}},
		{"AssignmentAttempted", AssignmentAttempted{TicketID: "TKT-001", Outcome: Failed{Category: NotFound, Reason: "ticket not found"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewUIProjection()
			p = p.Record(tt.event)

			state := p.State()
			if state.ErrorMessage == "" {
				t.Error("ErrorMessage = empty, want error message")
			}
		})
	}
}

func TestUIProjection_SuccessfulAction_ClearsError(t *testing.T) {
	p := NewUIProjection()
	// First, create an error
	p = p.Record(SprintStartAttempted{Outcome: Failed{Category: BusinessRule, Reason: "error"}})

	state := p.State()
	if state.ErrorMessage == "" {
		t.Error("Setup: ErrorMessage should be set")
	}

	// Successful action clears error
	p = p.Record(SprintStartAttempted{Outcome: Succeeded{}})

	state = p.State()
	if state.ErrorMessage != "" {
		t.Errorf("ErrorMessage = %q, want empty after success", state.ErrorMessage)
	}
}

func TestUIProjection_LessonToggle(t *testing.T) {
	p := NewUIProjection()

	// Initially false
	if p.State().LessonVisible {
		t.Error("Initial LessonVisible = true, want false")
	}

	// Toggle on
	p = p.Record(LessonPanelToggled{})
	if !p.State().LessonVisible {
		t.Error("After first toggle: LessonVisible = false, want true")
	}

	// Toggle off
	p = p.Record(LessonPanelToggled{})
	if p.State().LessonVisible {
		t.Error("After second toggle: LessonVisible = true, want false")
	}
}

func TestUIProjection_TicketSelection(t *testing.T) {
	p := NewUIProjection()
	p = p.Record(TicketSelected{ID: "TKT-042"})

	state := p.State()
	if state.SelectedTicket != "TKT-042" {
		t.Errorf("SelectedTicket = %q, want TKT-042", state.SelectedTicket)
	}
}

func TestUIProjection_AssignmentSuccess_ClearsSelection(t *testing.T) {
	p := NewUIProjection()
	p = p.Record(TicketSelected{ID: "TKT-001"})

	// Verify selected
	if p.State().SelectedTicket != "TKT-001" {
		t.Error("Setup: ticket should be selected")
	}

	// Successful assignment clears selection
	p = p.Record(AssignmentAttempted{TicketID: "TKT-001", Outcome: Succeeded{}})

	if p.State().SelectedTicket != "" {
		t.Errorf("SelectedTicket after assignment = %q, want empty", p.State().SelectedTicket)
	}
}

func TestUIProjection_AssignmentFailure_KeepsSelection(t *testing.T) {
	p := NewUIProjection()
	p = p.Record(TicketSelected{ID: "TKT-001"})
	p = p.Record(AssignmentAttempted{TicketID: "TKT-001", Outcome: Failed{Category: Conflict, Reason: "dev busy"}})

	state := p.State()
	if state.SelectedTicket != "TKT-001" {
		t.Errorf("SelectedTicket = %q, want TKT-001 (should keep on failure)", state.SelectedTicket)
	}
	if state.ErrorMessage == "" {
		t.Error("ErrorMessage = empty, want error")
	}
}

func TestUIProjection_ValueSemantics(t *testing.T) {
	p1 := NewUIProjection()
	p2 := p1.Record(ViewSwitched{To: ViewExecution})

	// p1 should be unchanged
	if p1.State().CurrentView != ViewPlanning {
		t.Error("Original projection was mutated")
	}

	// p2 should have the change
	if p2.State().CurrentView != ViewExecution {
		t.Error("New projection missing change")
	}
}

func TestUIProjection_PureFold_MultipleEvents(t *testing.T) {
	p := NewUIProjection()

	// Record a sequence of events
	p = p.Record(ViewSwitched{To: ViewExecution})
	p = p.Record(TicketSelected{ID: "TKT-001"})
	p = p.Record(LessonPanelToggled{})
	p = p.Record(AssignmentAttempted{TicketID: "TKT-001", Outcome: Failed{Category: NotFound, Reason: "dev not found"}})

	state := p.State()
	if state.CurrentView != ViewExecution {
		t.Errorf("CurrentView = %v, want ViewExecution", state.CurrentView)
	}
	if state.SelectedTicket != "TKT-001" {
		t.Errorf("SelectedTicket = %q, want TKT-001", state.SelectedTicket)
	}
	if !state.LessonVisible {
		t.Error("LessonVisible = false, want true")
	}
	if state.ErrorMessage == "" {
		t.Error("ErrorMessage = empty, want error")
	}
}

func TestUIProjection_Idempotent_SameEventsProduceSameState(t *testing.T) {
	events := []InputEvent{
		ViewSwitched{To: ViewExecution},
		TicketSelected{ID: "TKT-001"},
		LessonPanelToggled{},
		SprintStartAttempted{Outcome: Failed{Category: BusinessRule, Reason: "test"}},
	}

	// Create two projections with same events
	p1 := NewUIProjection()
	p2 := NewUIProjection()

	for _, e := range events { // justified:SM
		p1 = p1.Record(e)
		p2 = p2.Record(e)
	}

	s1 := p1.State()
	s2 := p2.State()

	if s1 != s2 {
		t.Errorf("Idempotency violated:\nState1: %+v\nState2: %+v", s1, s2)
	}

	// Call State() multiple times - should always return same result
	for i := 0; i < 3; i++ { // justified:SM
		if p1.State() != s1 {
			t.Errorf("State() not idempotent on call %d", i+1)
		}
	}
}

func BenchmarkUIProjection_State(b *testing.B) {
	// Build projection with typical session events
	p := NewUIProjection()
	for i := 0; i < 50; i++ { // justified:SM
		p = p.Record(ViewSwitched{To: View(i % 4)})
		p = p.Record(TicketSelected{ID: "TKT-001"})
		p = p.Record(SprintStartAttempted{Outcome: Succeeded{}})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = p.State()
	}
}

func BenchmarkUIProjection_Record(b *testing.B) {
	// Build projection with typical session (150 events)
	p := NewUIProjection()
	for i := 0; i < 50; i++ { // justified:SM
		p = p.Record(ViewSwitched{To: View(i % 4)})
		p = p.Record(TicketSelected{ID: "TKT-001"})
		p = p.Record(SprintStartAttempted{Outcome: Succeeded{}})
	}

	evt := ViewSwitched{To: ViewExecution}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = p.Record(evt)
	}
}
