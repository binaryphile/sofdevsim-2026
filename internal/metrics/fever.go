package metrics

import (
	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// FeverChart tracks TameFlow buffer consumption
type FeverChart struct {
	BufferTotal     float64
	BufferConsumed  float64
	BufferRemaining float64
	Status          model.FeverStatus
	History         []FeverSnapshot
}

// FeverSnapshot captures buffer state at a point in time
type FeverSnapshot struct {
	Day         int
	PercentUsed float64
	Status      model.FeverStatus
}

// NewFeverChart creates an initialized fever chart
func NewFeverChart() *FeverChart {
	return &FeverChart{
		History: make([]FeverSnapshot, 0),
	}
}

// Update recalculates fever chart from sprint state
func (f *FeverChart) Update(sprint *model.Sprint) {
	if sprint == nil {
		return
	}

	f.BufferTotal = sprint.BufferDays
	f.BufferConsumed = sprint.BufferConsumed
	f.BufferRemaining = f.BufferTotal - f.BufferConsumed
	if f.BufferRemaining < 0 {
		f.BufferRemaining = 0
	}
	f.Status = sprint.FeverStatus

	pctUsed := 0.0
	if f.BufferTotal > 0 {
		pctUsed = (f.BufferConsumed / f.BufferTotal) * 100
	}

	f.History = append(f.History, FeverSnapshot{
		Day:         sprint.StartDay + len(f.History),
		PercentUsed: pctUsed,
		Status:      f.Status,
	})
}

// PercentUsed returns buffer consumption as percentage
func (f *FeverChart) PercentUsed() float64 {
	if f.BufferTotal == 0 {
		return 0
	}
	return (f.BufferConsumed / f.BufferTotal) * 100
}

// PercentRemaining returns buffer remaining as percentage
func (f *FeverChart) PercentRemaining() float64 {
	return 100 - f.PercentUsed()
}

// IsGreen returns true if buffer is healthy
func (f *FeverChart) IsGreen() bool {
	return f.Status == model.FeverGreen
}

// IsYellow returns true if buffer is concerning
func (f *FeverChart) IsYellow() bool {
	return f.Status == model.FeverYellow
}

// IsRed returns true if buffer is critical
func (f *FeverChart) IsRed() bool {
	return f.Status == model.FeverRed
}

// HistoryValues returns buffer percentages for sparkline
func (f *FeverChart) HistoryValues() []float64 {
	values := make([]float64, len(f.History))
	for i, snap := range f.History {
		values[i] = snap.PercentUsed
	}
	return values
}
