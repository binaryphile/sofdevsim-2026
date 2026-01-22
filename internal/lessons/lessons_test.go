package lessons

import (
	"testing"
)

// TestSelect tests the pure lesson selection logic.
// Per Khorikov: Domain logic gets comprehensive unit tests.
func TestSelect(t *testing.T) {
	tests := []struct {
		name                string
		view                ViewContext
		state               State
		hasActiveSprint     bool
		hasComparisonResult bool
		want                LessonID
	}{
		{
			name:  "first enable shows orientation",
			view:  ViewPlanning,
			state: State{},
			want:  Orientation,
		},
		{
			name:  "first enable in execution also shows orientation",
			view:  ViewExecution,
			state: State{},
			want:  Orientation,
		},
		{
			name:  "planning view after orientation shows understanding",
			view:  ViewPlanning,
			state: State{SeenMap: map[LessonID]bool{Orientation: true}},
			want:  Understanding,
		},
		{
			name:            "execution view with active sprint shows fever chart",
			view:            ViewExecution,
			state:           State{SeenMap: map[LessonID]bool{Orientation: true}},
			hasActiveSprint: true,
			want:            FeverChart,
		},
		{
			name:            "execution view without active sprint shows phase progress",
			view:            ViewExecution,
			state:           State{SeenMap: map[LessonID]bool{Orientation: true}},
			hasActiveSprint: false,
			want:            PhaseProgress,
		},
		{
			name:  "metrics view shows DORA metrics",
			view:  ViewMetrics,
			state: State{SeenMap: map[LessonID]bool{Orientation: true}},
			want:  DORAMetrics,
		},
		{
			name:                "comparison view with results shows policy comparison",
			view:                ViewComparison,
			state:               State{SeenMap: map[LessonID]bool{Orientation: true}},
			hasComparisonResult: true,
			want:                PolicyComparison,
		},
		{
			name:                "comparison view without results shows intro",
			view:                ViewComparison,
			state:               State{SeenMap: map[LessonID]bool{Orientation: true}},
			hasComparisonResult: false,
			want:                PolicyComparison, // ComparisonIntroLesson uses same ID
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Select(tt.view, tt.state, tt.hasActiveSprint, tt.hasComparisonResult)
			if got.ID != tt.want {
				t.Errorf("Select() = %v, want %v", got.ID, tt.want)
			}
		})
	}
}

// TestState_WithSeen tests the value semantics of State.
// Per Khorikov: Domain logic (state transitions) gets unit tests.
func TestLessonState_WithSeenDoesNotMutate(t *testing.T) {
	t.Run("marks lesson as seen", func(t *testing.T) {
		state := State{}
		newState := state.WithSeen(Orientation)

		if !newState.SeenMap[Orientation] {
			t.Error("expected Orientation to be marked as seen")
		}
		if newState.Current != Orientation {
			t.Errorf("Current = %v, want %v", newState.Current, Orientation)
		}
	})

	t.Run("does not mutate original state", func(t *testing.T) {
		state := State{}
		_ = state.WithSeen(Orientation)

		if state.SeenMap != nil && state.SeenMap[Orientation] {
			t.Error("original state was mutated")
		}
	})

	t.Run("accumulates seen lessons", func(t *testing.T) {
		state := State{}
		state = state.WithSeen(Orientation)
		state = state.WithSeen(Understanding)
		state = state.WithSeen(FeverChart)

		if state.SeenCount() != 3 {
			t.Errorf("SeenCount() = %d, want 3", state.SeenCount())
		}
	})
}

// TestState_WithVisible - PRUNED per Khorikov: trivial setter, can't meaningfully fail
