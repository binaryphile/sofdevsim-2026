package model

import (
	"testing"
)

// UC40 (#15446): table-driven coverage of pure ShouldAffordInvestment
// and AvailableOptions calculations per Decision A (Domain quadrant:
// heavy unit-test with full input matrix).

func TestShouldAffordInvestment(t *testing.T) {
	// Cost matrix per Phase 1b Decision A:
	//   Hire=5, CICDSlot=3, ReviewTool=2, VerifyPaydown=2
	tests := []struct {
		budget int
		opt    InvestmentOption
		want   bool
	}{
		// Budget=0 — nothing affordable
		{0, InvestHire, false},
		{0, InvestCICDSlot, false},
		{0, InvestReviewTool, false},
		{0, InvestVerifyPaydown, false},
		// Budget=1 — nothing affordable (all costs > 1)
		{1, InvestHire, false},
		{1, InvestCICDSlot, false},
		{1, InvestReviewTool, false},
		{1, InvestVerifyPaydown, false},
		// Budget=2 — only ReviewTool + VerifyPaydown (cost 2) affordable
		{2, InvestHire, false},
		{2, InvestCICDSlot, false},
		{2, InvestReviewTool, true},
		{2, InvestVerifyPaydown, true},
		// Budget=3 — also CICDSlot (cost 3); Hire (cost 5) still out
		{3, InvestHire, false},
		{3, InvestCICDSlot, true},
		{3, InvestReviewTool, true},
		{3, InvestVerifyPaydown, true},
		// Budget=5 — Hire now affordable
		{5, InvestHire, true},
		{5, InvestCICDSlot, true},
		{5, InvestReviewTool, true},
		{5, InvestVerifyPaydown, true},
		// Budget=10 (default starting budget) — all affordable
		{10, InvestHire, true},
		{10, InvestCICDSlot, true},
		{10, InvestReviewTool, true},
		{10, InvestVerifyPaydown, true},
	}

	for _, tc := range tests {
		got := ShouldAffordInvestment(tc.budget, tc.opt)
		if got != tc.want {
			t.Errorf("ShouldAffordInvestment(budget=%d, %v) = %v; want %v",
				tc.budget, tc.opt, got, tc.want)
		}
	}
}

func TestAvailableOptions(t *testing.T) {
	tests := []struct {
		name   string
		budget int
		want   []InvestmentOption
	}{
		{"budget=0 → empty (UC40 ext §1a)", 0, []InvestmentOption{}},
		{"budget=1 → empty (below cheapest cost 2)", 1, []InvestmentOption{}},
		{"budget=2 → ReviewTool + VerifyPaydown (in enum order)", 2,
			[]InvestmentOption{InvestReviewTool, InvestVerifyPaydown}},
		{"budget=3 → CICDSlot + ReviewTool + VerifyPaydown (enum order)", 3,
			[]InvestmentOption{InvestCICDSlot, InvestReviewTool, InvestVerifyPaydown}},
		{"budget=4 → CICDSlot + ReviewTool + VerifyPaydown (Hire still unaffordable)", 4,
			[]InvestmentOption{InvestCICDSlot, InvestReviewTool, InvestVerifyPaydown}},
		{"budget=5 → all 4 (Hire just affordable)", 5,
			[]InvestmentOption{InvestHire, InvestCICDSlot, InvestReviewTool, InvestVerifyPaydown}},
		{"budget=10 (default) → all 4", 10,
			[]InvestmentOption{InvestHire, InvestCICDSlot, InvestReviewTool, InvestVerifyPaydown}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := AvailableOptions(tc.budget)
			if len(got) != len(tc.want) {
				t.Fatalf("AvailableOptions(budget=%d) len = %d; want %d", tc.budget, len(got), len(tc.want))
			}
			for i, w := range tc.want { // justified:IX
				if got[i] != w {
					t.Errorf("AvailableOptions(budget=%d)[%d] = %v; want %v (enum order required)",
						tc.budget, i, got[i], w)
				}
			}
		})
	}
}

// AvailableOptions must return an empty slice (not nil) when no options
// are affordable per UC40 ext §1a — TUI code may rely on iterating over
// it without nil-handling.
func TestAvailableOptions_EmptyNotNil(t *testing.T) {
	got := AvailableOptions(0)
	if got == nil {
		t.Errorf("AvailableOptions(0) = nil; want non-nil empty slice (UC40 ext §1a contract)")
	}
}
