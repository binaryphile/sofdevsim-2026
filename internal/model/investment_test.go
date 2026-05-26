package model

import (
	"errors"
	"testing"
)

// UC40 (#15446): table-driven coverage for ParseInvestmentOption per
// Decision A (Domain heavy unit-test) + InvestmentOption String round-
// trip + errors.Is-chain guard.

func TestParseInvestmentOption(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    InvestmentOption
		wantErr error
	}{
		{"hire", "hire", InvestHire, nil},
		{"cicd-slot", "cicd-slot", InvestCICDSlot, nil},
		{"review-tool", "review-tool", InvestReviewTool, nil},
		{"verify-paydown", "verify-paydown", InvestVerifyPaydown, nil},
		{"case-insensitive HIRE", "HIRE", InvestHire, nil},
		{"case-insensitive CICD-Slot", "CICD-Slot", InvestCICDSlot, nil},
		{"empty string is invalid (no default)", "", 0, ErrInvalidInvestment},
		{"unknown garbage", "garbage", 0, ErrInvalidInvestment},
		{"snake_case (not kebab) is invalid", "cicd_slot", 0, ErrInvalidInvestment},
		{"whitespace not stripped", " hire ", 0, ErrInvalidInvestment},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseInvestmentOption(tc.input)
			if tc.wantErr == nil {
				if err != nil {
					t.Errorf("got err %v; want nil", err)
				}
				if got != tc.want {
					t.Errorf("got %v; want %v", got, tc.want)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got err %v; want errors.Is(err, %v)", err, tc.wantErr)
			}
		})
	}
}

func TestInvestmentOption_String(t *testing.T) {
	tests := []struct {
		opt  InvestmentOption
		want string
	}{
		{InvestHire, "hire"},
		{InvestCICDSlot, "cicd-slot"},
		{InvestReviewTool, "review-tool"},
		{InvestVerifyPaydown, "verify-paydown"},
	}
	for _, tc := range tests {
		if got := tc.opt.String(); got != tc.want {
			t.Errorf("InvestmentOption(%d).String() = %q; want %q", tc.opt, got, tc.want)
		}
	}
}

func TestParseInvestmentOption_ErrorsIsChain(t *testing.T) {
	_, err := ParseInvestmentOption("nonsense")
	if !errors.Is(err, ErrInvalidInvestment) {
		t.Fatalf("err must unwrap to ErrInvalidInvestment; got %v", err)
	}
}

func TestInvestmentOptionCost_CompleteAndCorrect(t *testing.T) {
	// Locked per Phase 1b Decision A — costs are part of the user-facing
	// contract; if these tests fail, the cost contract has drifted.
	want := map[InvestmentOption]int{
		InvestHire:          5,
		InvestCICDSlot:      3,
		InvestReviewTool:    2,
		InvestVerifyPaydown: 2,
	}
	for opt, wantCost := range want {
		if got := InvestmentOptionCost[opt]; got != wantCost {
			t.Errorf("InvestmentOptionCost[%v] = %d; want %d (Phase 1b Decision A cost contract)", opt, got, wantCost)
		}
	}
	if len(InvestmentOptionCost) != 4 {
		t.Errorf("InvestmentOptionCost length = %d; want 4 (all options must have a cost)", len(InvestmentOptionCost))
	}
}

func TestAllInvestmentOptions_EnumOrder(t *testing.T) {
	opts := AllInvestmentOptions()
	want := []InvestmentOption{InvestHire, InvestCICDSlot, InvestReviewTool, InvestVerifyPaydown}
	if len(opts) != len(want) {
		t.Fatalf("AllInvestmentOptions len = %d; want %d", len(opts), len(want))
	}
	for i, w := range want { // justified:IX
		if opts[i] != w {
			t.Errorf("AllInvestmentOptions[%d] = %v; want %v (enum order)", i, opts[i], w)
		}
	}
}
