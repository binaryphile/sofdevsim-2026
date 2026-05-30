// Package api: fluentfp/web migration boundary (#18915).
//
// adapt.go houses the consolidated domain-sentinel → HTTP-status mapper and
// small response-building helpers used by the closure-based handlers. Per
// plan §Decisions C, Content-Type is left to fluentfp/web.Adapt's default
// (application/json) — the HAL body shape via HALResponse is preserved but
// the application/hal+json content-type header is dropped.
package api

import (
	"errors"
	"net/http"

	"github.com/binaryphile/fluentfp/web"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
	"github.com/binaryphile/sofdevsim-2026/internal/registry"
)

// domainErrorMapper is the single source of truth for domain-sentinel →
// HTTP-status mapping used by every Adapt call via web.WithErrorMapper.
//
// Per the pre-migration audit (event 20267), all 4 sentinel-mapped writeError
// sites in handlers.go (lines 191, 198, 208, 971) reconcile into this table.
// The other 25+ writeError sites use direct web.<Status>(msg) calls at the
// handler closure and bypass this mapper per Adapt's errors.As flow.
//
// Table:
//
//	ErrAlreadyExists                        → 409 ALREADY_EXISTS
//	registry.ErrUnknownScenario             → 400 UNKNOWN_SCENARIO
//	model.ErrCapZero/Negative/BelowMentorMin/Conflict → 422 INVALID_PHASE_CAP
//	model.ErrInsufficientBudget             → 422 INSUFFICIENT_BUDGET
//	model.ErrInvalidInvestment              → 422 INVALID_INVESTMENT
//	model.ErrInvestmentWindowClosed         → 409 INVESTMENT_WINDOW_CLOSED
//	registry.ErrSimNotFound                 → 404 SIMULATION_NOT_FOUND
//	(unmapped)                              → fallthrough to Adapt's 500
func domainErrorMapper(err error) (*web.Error, bool) {
	switch {
	case errors.Is(err, ErrAlreadyExists):
		return &web.Error{Status: http.StatusConflict, Message: err.Error(), Code: "ALREADY_EXISTS"}, true
	case errors.Is(err, registry.ErrUnknownScenario):
		return &web.Error{Status: http.StatusBadRequest, Message: err.Error(), Code: "UNKNOWN_SCENARIO"}, true
	case errors.Is(err, model.ErrCapZero) ||
		errors.Is(err, model.ErrCapNegative) ||
		errors.Is(err, model.ErrCapBelowMentorMin) ||
		errors.Is(err, model.ErrCapConflict):
		return &web.Error{Status: http.StatusUnprocessableEntity, Message: err.Error(), Code: "INVALID_PHASE_CAP"}, true
	case errors.Is(err, model.ErrInsufficientBudget):
		return &web.Error{Status: http.StatusUnprocessableEntity, Message: err.Error(), Code: "INSUFFICIENT_BUDGET"}, true
	case errors.Is(err, model.ErrInvalidInvestment):
		return &web.Error{Status: http.StatusUnprocessableEntity, Message: err.Error(), Code: "INVALID_INVESTMENT"}, true
	case errors.Is(err, model.ErrInvestmentWindowClosed):
		return &web.Error{Status: http.StatusConflict, Message: err.Error(), Code: "INVESTMENT_WINDOW_CLOSED"}, true
	case errors.Is(err, registry.ErrSimNotFound):
		return &web.Error{Status: http.StatusNotFound, Message: err.Error(), Code: "SIMULATION_NOT_FOUND"}, true
	}
	return nil, false
}

// simulationResponse builds the HAL-body Response for a simulation instance.
// Replaces respondWithSimulation (which mutated ResponseWriter). Returns a
// pure value; the caller wraps with rslt.Ok.
//
// Content-Type left to Adapt's default (application/json) per Decision C —
// HAL body shape (_links / _embedded) preserved.
func simulationResponse(inst registry.SimInstance, status int) web.Response {
	state := ToState(inst.Engine.Sim(), inst.Tracker)
	return web.Response{
		Status: status,
		Body: HALResponse{
			State: state,
			Links: LinksFor(state),
		},
	}
}

// decodeError translates decode-failure sentinels (ErrBodyTooLarge,
// ErrInvalidJSON) into typed *web.Error values. Replaces respondDecodeError
// (which mutated ResponseWriter). Caller wraps with rslt.Err.
func decodeError(err error) *web.Error {
	switch {
	case errors.Is(err, ErrBodyTooLarge):
		return &web.Error{Status: http.StatusRequestEntityTooLarge, Message: err.Error(), Code: "BODY_TOO_LARGE"}
	default:
		return &web.Error{Status: http.StatusBadRequest, Message: err.Error(), Code: "INVALID_JSON"}
	}
}
