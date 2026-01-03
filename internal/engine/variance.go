package engine

import (
	"math/rand"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// VarianceModel calculates work variance based on understanding level
// This is the key hypothesis: understanding affects predictability
type VarianceModel struct {
	seed int64
}

// NewVarianceModel creates a variance model with a seed
func NewVarianceModel(seed int64) *VarianceModel {
	return &VarianceModel{seed: seed}
}

// Calculate returns a variance multiplier for the given ticket and tick
// High understanding = predictable (0.95-1.05x)
// Low understanding = unpredictable, skewed slow (0.50-1.50x)
func (v *VarianceModel) Calculate(ticket model.Ticket, tick int) float64 {
	rng := rand.New(rand.NewSource(v.seed + int64(tick) + int64(ticket.ID[0])))

	var base, spread float64
	switch ticket.UnderstandingLevel {
	case model.HighUnderstanding:
		base, spread = 0.95, 0.10 // 0.95-1.05x (very predictable)
	case model.MediumUnderstanding:
		base, spread = 0.80, 0.40 // 0.80-1.20x (some surprise)
	case model.LowUnderstanding:
		base, spread = 0.50, 1.00 // 0.50-1.50x (high surprise, skewed slow)
	}

	variance := base + rng.Float64()*spread

	// Apply phase multiplier
	if mult, ok := model.UnderstandingPhaseMultiplier[ticket.UnderstandingLevel][ticket.Phase]; ok {
		variance *= mult
	}

	return variance
}
