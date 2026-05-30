package api

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
	"github.com/binaryphile/sofdevsim-2026/internal/registry"
)

// TestDomainErrorMapper — table-driven over all 8 known sentinels + the
// fallback (unknown error). Per /grade R1 F1, the pre-migration audit
// (event 20267) confirmed these are the only domain sentinels routed
// through the mapper; ANY new sentinel added to a handler must be added
// here AND to domainErrorMapper.
//
// Status mapping table (canonical):
//
//	ErrAlreadyExists                                                → 409 ALREADY_EXISTS
//	registry.ErrUnknownScenario                                     → 400 UNKNOWN_SCENARIO
//	model.ErrCapZero / Negative / BelowMentorMin / Conflict         → 422 INVALID_PHASE_CAP
//	model.ErrInsufficientBudget                                     → 422 INSUFFICIENT_BUDGET
//	model.ErrInvalidInvestment                                      → 422 INVALID_INVESTMENT
//	model.ErrInvestmentWindowClosed                                 → 409 INVESTMENT_WINDOW_CLOSED
//	registry.ErrSimNotFound                                         → 404 SIMULATION_NOT_FOUND
//	(unmapped)                                                      → handled=false (falls to Adapt's 500)
func TestDomainErrorMapper(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantCode    string
		wantHandled bool
	}{
		{name: "ErrAlreadyExists → 409", err: ErrAlreadyExists, wantStatus: http.StatusConflict, wantCode: "ALREADY_EXISTS", wantHandled: true},
		{name: "ErrUnknownScenario → 400", err: registry.ErrUnknownScenario, wantStatus: http.StatusBadRequest, wantCode: "UNKNOWN_SCENARIO", wantHandled: true},
		{name: "ErrCapZero → 422", err: model.ErrCapZero, wantStatus: http.StatusUnprocessableEntity, wantCode: "INVALID_PHASE_CAP", wantHandled: true},
		{name: "ErrCapNegative → 422", err: model.ErrCapNegative, wantStatus: http.StatusUnprocessableEntity, wantCode: "INVALID_PHASE_CAP", wantHandled: true},
		{name: "ErrCapBelowMentorMin → 422", err: model.ErrCapBelowMentorMin, wantStatus: http.StatusUnprocessableEntity, wantCode: "INVALID_PHASE_CAP", wantHandled: true},
		{name: "ErrCapConflict → 422", err: model.ErrCapConflict, wantStatus: http.StatusUnprocessableEntity, wantCode: "INVALID_PHASE_CAP", wantHandled: true},
		{name: "ErrInsufficientBudget → 422", err: model.ErrInsufficientBudget, wantStatus: http.StatusUnprocessableEntity, wantCode: "INSUFFICIENT_BUDGET", wantHandled: true},
		{name: "ErrInvalidInvestment → 422", err: model.ErrInvalidInvestment, wantStatus: http.StatusUnprocessableEntity, wantCode: "INVALID_INVESTMENT", wantHandled: true},
		{name: "ErrInvestmentWindowClosed → 409", err: model.ErrInvestmentWindowClosed, wantStatus: http.StatusConflict, wantCode: "INVESTMENT_WINDOW_CLOSED", wantHandled: true},
		{name: "ErrSimNotFound → 404", err: registry.ErrSimNotFound, wantStatus: http.StatusNotFound, wantCode: "SIMULATION_NOT_FOUND", wantHandled: true},
		{name: "unknown error → fallthrough", err: errors.New("totally unrelated error"), wantStatus: 0, wantCode: "", wantHandled: false},
		{name: "wrapped ErrAlreadyExists matches via errors.Is", err: fmt.Errorf("create failed: %w", ErrAlreadyExists), wantStatus: http.StatusConflict, wantCode: "ALREADY_EXISTS", wantHandled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, handled := domainErrorMapper(tt.err)
			if handled != tt.wantHandled {
				t.Fatalf("domainErrorMapper handled=%v, want %v", handled, tt.wantHandled)
			}
			if !handled {
				return
			}
			if got.Status != tt.wantStatus {
				t.Errorf("Status=%d, want %d", got.Status, tt.wantStatus)
			}
			if got.Code != tt.wantCode {
				t.Errorf("Code=%q, want %q", got.Code, tt.wantCode)
			}
		})
	}
}
