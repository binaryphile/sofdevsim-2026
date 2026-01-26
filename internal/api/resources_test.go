package api

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

func TestToTicketState_DecompositionFields(t *testing.T) {
	ticket := model.Ticket{
		ID:            "parent-1",
		ParentID:      "",
		ChildIDs:      []string{"child-1", "child-2"},
		EstimatedDays: 5.0,
	}

	got := ToTicketState(ticket)

	if got.ParentID != "" {
		t.Errorf("ParentID = %q, want empty", got.ParentID)
	}
	if len(got.ChildIDs) != 2 || got.ChildIDs[0] != "child-1" {
		t.Errorf("ChildIDs = %v, want [child-1, child-2]", got.ChildIDs)
	}
	if got.EstimatedDays != 5.0 {
		t.Errorf("EstimatedDays = %v, want 5.0", got.EstimatedDays)
	}
}
