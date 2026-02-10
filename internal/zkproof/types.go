// Package zkproof provides sequence detection for ZK proof generation.
package zkproof

import "github.com/binaryphile/sofdevsim-2026/internal/events"

// BufferCrisisSequence represents a detected crisis->recovery pattern.
type BufferCrisisSequence struct {
	CrisisTick   int            // Tick when buffer entered red zone
	RecoveryTick int            // Tick when buffer returned to green
	Events       []events.Event // All events in the sequence (max 32)
}
