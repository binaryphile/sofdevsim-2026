package engine

import (
	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// defaultLowMaxEstimate is the fallback for ExperienceConfig.LowMaxEstimate.
const defaultLowMaxEstimate = 5.0

// RoundRobinPolicy assigns devs in round-robin order with experience constraints.
// Low-experience devs don't get tickets above LowExpMaxEstimate.
// When a Low dev is assigned, an idle High dev is paired as mentor if available.
// Self-review prohibition: contributors can't review their own ticket (solo-team exception).
// Uses Simulation.AssignCursor for persistent round-robin (advances on each assignment).
type RoundRobinPolicy struct{}

// NewRoundRobinPolicy creates a round-robin assignment policy.
func NewRoundRobinPolicy() RoundRobinPolicy {
	return RoundRobinPolicy{}
}

// SelectDev picks the next eligible developer in round-robin order.
// Uses Simulation.AssignCursor for persistent round-robin — each assignment
// advances the cursor, ensuring fair distribution across all assignments.
func (p RoundRobinPolicy) SelectDev(state model.Simulation, ticketID string, phase model.WorkflowPhase) AssignmentResult {
	n := len(state.Developers)
	if n == 0 {
		return AssignmentResult{}
	}

	ticketIdx := state.FindActiveTicketIndex(ticketID)
	if ticketIdx == -1 {
		return AssignmentResult{}
	}
	ticket := state.ActiveTickets[ticketIdx]
	// Persistent round-robin: start from cursor+1, wrapping around.
	// Two-pass for Review: first try non-contributors, then fall back to contributors
	// (small teams may have all devs as contributors due to handoffs).
	startIdx := (state.AssignCursor + 1) % n
	lowMax := option.NonZero(state.ExperienceConfig.LowMaxEstimate).Or(defaultLowMaxEstimate)
	var devID string
	var devExp model.ExperienceLevel
	var fallbackDevID string
	var fallbackDevExp model.ExperienceLevel
	for attempt := 0; attempt < n; attempt++ {
		idx := (startIdx + attempt) % n
		dev := state.Developers[idx]

		if !state.IsDevAvailable(dev.ID) {
			continue
		}

		exp := dev.PhaseExperience[phase]

		// Low-experience devs can't take large tickets
		if exp == model.ExperienceLow && ticket.EstimatedDays > lowMax {
			continue
		}

		// Self-review prohibition: prefer non-contributors for Review
		if phase == model.PhaseReview && slice.Contains(ticket.Contributors, dev.ID) {
			if fallbackDevID == "" {
				fallbackDevID = dev.ID
				fallbackDevExp = exp
			}
			continue
		}

		devID = dev.ID
		devExp = exp
		break
	}
	// Fallback: if no non-contributor available for Review, allow a contributor
	if devID == "" && fallbackDevID != "" {
		devID = fallbackDevID
		devExp = fallbackDevExp
	}

	if devID == "" {
		return AssignmentResult{}
	}

	// Mentor pairing: if assigned dev is Low, find an idle High for mentoring
	var mentorID string
	if devExp == model.ExperienceLow {
		for _, dev := range state.Developers { // justified:CF
			if dev.ID == devID {
				continue
			}
			if !state.IsDevAvailable(dev.ID) {
				continue
			}
			if dev.PhaseExperience[phase] == model.ExperienceHigh {
				mentorID = dev.ID
				break
			}
		}
	}

	return AssignmentResult{
		DevID:    devID,
		MentorID: mentorID,
	}
}
