// Package zkproof provides sequence detection for ZK proof generation.
package zkproof

import (
	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/fluentfp/slice"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// MaxSequenceEvents is the maximum number of events in a proof sequence.
// This matches Mina's Actions/Reducer limit of 32 pending actions.
const MaxSequenceEvents = 32

// detectionState tracks crisis detection progress during fold.
type detectionState struct {
	inCrisis    bool
	crisisStart option.Basic[int]       // Tick when crisis started
	crisisEvts  []events.Event          // Events accumulated during crisis
	completed   []BufferCrisisSequence  // Completed sequences
}

// isBufferZoneChanged returns true if the event is a BufferZoneChanged event.
func isBufferZoneChanged(e events.Event) bool {
	_, ok := e.(events.BufferZoneChanged)
	return ok
}

// accumulate processes each zone change event, tracking whether we're in a
// crisis state and building complete crisis->recovery sequences.
func accumulate(state detectionState, e events.Event) detectionState {
	bzc, ok := e.(events.BufferZoneChanged)
	if !ok {
		return state
	}

	// Not in crisis + sees RED -> start crisis
	if !state.inCrisis && bzc.NewZone == model.FeverRed {
		state.inCrisis = true
		state.crisisStart = option.Of(bzc.OccurrenceTime())
		state.crisisEvts = []events.Event{e}
		return state
	}

	// In crisis + sees GREEN -> complete sequence
	if state.inCrisis && bzc.NewZone == model.FeverGreen {
		// Add recovery event
		state.crisisEvts = append(state.crisisEvts, e)

		// Truncate to max events if needed
		evts := state.crisisEvts
		if len(evts) > MaxSequenceEvents {
			evts = evts[:MaxSequenceEvents]
		}

		crisisTick, _ := state.crisisStart.Get()
		seq := BufferCrisisSequence{
			CrisisTick:   crisisTick,
			RecoveryTick: bzc.OccurrenceTime(),
			Events:       evts,
		}
		state.completed = append(state.completed, seq)

		// Reset for next potential crisis
		state.inCrisis = false
		state.crisisStart = option.Basic[int]{}
		state.crisisEvts = nil
		return state
	}

	// In crisis but not recovery -> accumulate
	if state.inCrisis {
		state.crisisEvts = append(state.crisisEvts, e)
	}

	return state
}

// DetectBufferCrisis finds complete crisis->recovery sequences in event history.
// A crisis starts when buffer enters red zone and ends when it returns to green.
func DetectBufferCrisis(evts []events.Event) []BufferCrisisSequence {
	zoneChanges := slice.From(evts).KeepIf(isBufferZoneChanged)
	initial := detectionState{completed: make([]BufferCrisisSequence, 0)}
	final := slice.Fold(zoneChanges, initial, accumulate)
	return final.completed
}
