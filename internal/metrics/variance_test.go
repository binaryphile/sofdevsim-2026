package metrics

import (
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/model"
)

// Benchmark baseline for child variance analysis.
// Run: go test -bench=. -benchmem ./internal/metrics/...
func BenchmarkAnalyzeChildVariance(b *testing.B) {
	tickets := []model.Ticket{
		{ID: "p1", ChildIDs: []string{"c1", "c2"}},
		{ID: "c1", EstimatedDays: 2, ActualDays: 2.5},
		{ID: "c2", EstimatedDays: 3, ActualDays: 3.2},
		{ID: "p2", ChildIDs: []string{"c3"}},
		{ID: "c3", EstimatedDays: 4, ActualDays: 5.0},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AnalyzeChildVariance(tickets)
	}
}

// Benchmark baseline for high variance detection.
func BenchmarkHasHighChildVariance(b *testing.B) {
	tickets := []model.Ticket{
		{ID: "p1", ChildIDs: []string{"c1", "c2"}},
		{ID: "c1", EstimatedDays: 2, ActualDays: 2.5},
		{ID: "c2", EstimatedDays: 3, ActualDays: 3.2},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HasHighChildVariance(tickets)
	}
}

func TestHasHighChildVariance(t *testing.T) {
	tests := []struct {
		name    string
		tickets []model.Ticket
		want    bool
	}{
		{"no decomposed = false", []model.Ticket{{ID: "1"}}, false},
		{"children low variance = false", []model.Ticket{
			{ID: "parent", ChildIDs: []string{"c1", "c2"}},
			{ID: "c1", EstimatedDays: 2, ActualDays: 2.2}, // 1.1 ratio
			{ID: "c2", EstimatedDays: 2, ActualDays: 2.4}, // 1.2 ratio
		}, false},
		{"children high variance = true", []model.Ticket{
			{ID: "parent", ChildIDs: []string{"c1"}},
			{ID: "c1", EstimatedDays: 2, ActualDays: 3.0}, // 1.5 ratio > 1.3
		}, true},
		{"incomplete children = false", []model.Ticket{
			{ID: "parent", ChildIDs: []string{"c1", "c2"}},
			{ID: "c1", EstimatedDays: 2, ActualDays: 3.0},
			// c2 not in completed list
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasHighChildVariance(tt.tickets); got != tt.want {
				t.Errorf("HasHighChildVariance() = %v, want %v", got, tt.want)
			}
		})
	}
}
