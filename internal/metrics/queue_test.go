package metrics

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

func TestQueueDepthPerPhase(t *testing.T) {
	tests := []struct {
		name    string
		tickets []model.Ticket
		want    map[model.WorkflowPhase]int
	}{
		{"empty", nil, map[model.WorkflowPhase]int{}},
		{"single phase", []model.Ticket{{Phase: model.PhaseImplement}},
			map[model.WorkflowPhase]int{model.PhaseImplement: 1}},
		{"multiple phases", []model.Ticket{
			{Phase: model.PhaseImplement},
			{Phase: model.PhaseImplement},
			{Phase: model.PhaseVerify},
		}, map[model.WorkflowPhase]int{model.PhaseImplement: 2, model.PhaseVerify: 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := QueueDepthPerPhase(tt.tickets)
			if len(got) != len(tt.want) {
				t.Errorf("QueueDepthPerPhase() len = %d, want %d", len(got), len(tt.want))
				return
			}
			for phase, count := range tt.want {
				if got[phase] != count {
					t.Errorf("QueueDepthPerPhase()[%v] = %d, want %d", phase, got[phase], count)
				}
			}
		})
	}
}

// Benchmark baseline for queue depth calculation.
// Run: go test -bench=. -benchmem ./internal/metrics/...
func BenchmarkQueueDepthPerPhase(b *testing.B) {
	tickets := []model.Ticket{
		{Phase: model.PhaseImplement}, {Phase: model.PhaseImplement},
		{Phase: model.PhaseVerify}, {Phase: model.PhaseVerify},
		{Phase: model.PhaseCICD},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		QueueDepthPerPhase(tickets)
	}
}

// Benchmark baseline for queue imbalance detection.
func BenchmarkHasQueueImbalance(b *testing.B) {
	tickets := []model.Ticket{
		{Phase: model.PhaseImplement}, {Phase: model.PhaseImplement},
		{Phase: model.PhaseImplement}, {Phase: model.PhaseImplement},
		{Phase: model.PhaseVerify}, {Phase: model.PhaseVerify},
		{Phase: model.PhaseCICD},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HasQueueImbalance(tickets)
	}
}

func TestHasQueueImbalance(t *testing.T) {
	tests := []struct {
		name    string
		tickets []model.Ticket
		want    bool
	}{
		{"empty = false", nil, false},
		{"single ticket = false", []model.Ticket{{Phase: model.PhaseImplement}}, false},
		{"balanced 2 phases = false", []model.Ticket{
			{Phase: model.PhaseImplement}, {Phase: model.PhaseVerify},
		}, false},
		{"imbalanced 3 phases = true", []model.Ticket{
			{Phase: model.PhaseImplement}, {Phase: model.PhaseImplement},
			{Phase: model.PhaseImplement}, {Phase: model.PhaseImplement},
			{Phase: model.PhaseImplement}, // 5 in implement
			{Phase: model.PhaseVerify},    // 1 in verify
			{Phase: model.PhaseCICD},      // 1 in cicd
		}, true}, // sum=7, phases=3, avg=2.33, implement(5) > 2×2.33=4.66 ✓
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasQueueImbalance(tt.tickets); got != tt.want {
				t.Errorf("HasQueueImbalance() = %v, want %v", got, tt.want)
			}
		})
	}
}
