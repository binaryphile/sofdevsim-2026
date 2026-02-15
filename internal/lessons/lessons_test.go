package lessons

import (
	"testing"
)

// TestSelect_ViewBasedSelection_ReturnsContextualLesson tests the pure lesson selection logic.
// Per Khorikov: Domain logic gets comprehensive unit tests.
func TestSelect_ViewBasedSelection_ReturnsContextualLesson(t *testing.T) {
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
			got := Select(tt.view, tt.state, tt.hasActiveSprint, tt.hasComparisonResult, TriggerState{}, ComparisonSummary{})
			if got.ID != tt.want {
				t.Errorf("Select() = %v, want %v", got.ID, tt.want)
			}
		})
	}
}

// TestSelect_UC19_UncertaintyConstraint tests the aha moment trigger.
func TestSelect_UC19_UncertaintyConstraint(t *testing.T) {
	tests := []struct {
		name     string
		view     ViewContext
		state    State
		triggers TriggerState
		want     LessonID
	}{
		{
			name:     "triggers on red buffer + low ticket",
			view:     ViewExecution,
			state:    State{SeenMap: map[LessonID]bool{Orientation: true}},
			triggers: TriggerState{HasRedBufferWithLowTicket: true},
			want:     UncertaintyConstraint,
		},
		{
			name:     "does not trigger if already seen",
			view:     ViewExecution,
			state:    State{SeenMap: map[LessonID]bool{Orientation: true, UncertaintyConstraint: true}},
			triggers: TriggerState{HasRedBufferWithLowTicket: true},
			want:     FeverChart, // Falls through to view-based selection
		},
		{
			name:     "does not trigger without red buffer",
			view:     ViewExecution,
			state:    State{SeenMap: map[LessonID]bool{Orientation: true}},
			triggers: TriggerState{HasRedBufferWithLowTicket: false},
			want:     FeverChart,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Select(tt.view, tt.state, true, false, tt.triggers, ComparisonSummary{})
			if got.ID != tt.want {
				t.Errorf("Select() = %v, want %v", got.ID, tt.want)
			}
		})
	}
}

// TestSelect_UC20_ConstraintHunt tests the queue imbalance trigger.
func TestSelect_UC20_ConstraintHunt(t *testing.T) {
	tests := []struct {
		name     string
		state    State
		triggers TriggerState
		want     LessonID
	}{
		{
			name:     "triggers on queue imbalance + UC19 seen",
			state:    State{SeenMap: map[LessonID]bool{Orientation: true, UncertaintyConstraint: true}},
			triggers: TriggerState{HasQueueImbalance: true},
			want:     ConstraintHunt,
		},
		{
			name:     "does not trigger without UC19 seen (prerequisite)",
			state:    State{SeenMap: map[LessonID]bool{Orientation: true}},
			triggers: TriggerState{HasQueueImbalance: true},
			want:     FeverChart, // Falls through to view-based
		},
		{
			name:     "does not trigger if already seen",
			state:    State{SeenMap: map[LessonID]bool{Orientation: true, UncertaintyConstraint: true, ConstraintHunt: true}},
			triggers: TriggerState{HasQueueImbalance: true},
			want:     FeverChart,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Select(ViewExecution, tt.state, true, false, tt.triggers, ComparisonSummary{})
			if got.ID != tt.want {
				t.Errorf("Select() = %v, want %v", got.ID, tt.want)
			}
		})
	}
}

// TestSelect_UC21_ExploitFirst tests the high child variance trigger.
func TestSelect_UC21_ExploitFirst(t *testing.T) {
	tests := []struct {
		name     string
		state    State
		triggers TriggerState
		want     LessonID
	}{
		{
			name:     "triggers on high child variance + UC19 seen",
			state:    State{SeenMap: map[LessonID]bool{Orientation: true, UncertaintyConstraint: true}},
			triggers: TriggerState{HasHighChildVariance: true},
			want:     ExploitFirst,
		},
		{
			name:     "does not trigger without UC19 seen (prerequisite)",
			state:    State{SeenMap: map[LessonID]bool{Orientation: true}},
			triggers: TriggerState{HasHighChildVariance: true},
			want:     FeverChart,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Select(ViewExecution, tt.state, true, false, tt.triggers, ComparisonSummary{})
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

// TestSelect_UC22_FiveFocusing tests the synthesis lesson trigger.
// UC22 triggers on 3+ sprints AND (UC20 OR UC21 seen).
func TestSelect_UC22_FiveFocusing(t *testing.T) {
	tests := []struct {
		name     string
		state    State
		triggers TriggerState
		want     LessonID
	}{
		{
			name:     "triggers on 3+ sprints + ConstraintHunt seen",
			state:    State{SeenMap: map[LessonID]bool{Orientation: true, UncertaintyConstraint: true, ConstraintHunt: true}},
			triggers: TriggerState{SprintCount: 3},
			want:     FiveFocusing,
		},
		{
			name:     "triggers on 3+ sprints + ExploitFirst seen",
			state:    State{SeenMap: map[LessonID]bool{Orientation: true, UncertaintyConstraint: true, ExploitFirst: true}},
			triggers: TriggerState{SprintCount: 3},
			want:     FiveFocusing,
		},
		{
			name:     "does not trigger with only 2 sprints",
			state:    State{SeenMap: map[LessonID]bool{Orientation: true, UncertaintyConstraint: true, ConstraintHunt: true}},
			triggers: TriggerState{SprintCount: 2},
			want:     FeverChart, // Falls through to view-based
		},
		{
			name:     "does not trigger without symptom lesson (UC20 or UC21)",
			state:    State{SeenMap: map[LessonID]bool{Orientation: true, UncertaintyConstraint: true}},
			triggers: TriggerState{SprintCount: 3},
			want:     FeverChart,
		},
		{
			name:     "does not trigger if already seen",
			state:    State{SeenMap: map[LessonID]bool{Orientation: true, UncertaintyConstraint: true, ConstraintHunt: true, FiveFocusing: true}},
			triggers: TriggerState{SprintCount: 3},
			want:     FeverChart,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Select(ViewExecution, tt.state, true, false, tt.triggers, ComparisonSummary{})
			if got.ID != tt.want {
				t.Errorf("Select() = %v, want %v", got.ID, tt.want)
			}
		})
	}
}

// TestSelect_UC23_ManagerTakeaways tests the dynamic synthesis lesson trigger.
// UC23 triggers on ViewComparison + HasResult + UC22 seen.
func TestSelect_UC23_ManagerTakeaways(t *testing.T) {
	tests := []struct {
		name        string
		view        ViewContext
		state       State
		triggers    TriggerState
		comparison  ComparisonSummary
		want        LessonID
	}{
		{
			name:       "triggers on comparison view + result + FiveFocusing seen",
			view:       ViewComparison,
			state:      State{SeenMap: map[LessonID]bool{Orientation: true, UncertaintyConstraint: true, ConstraintHunt: true, FiveFocusing: true}},
			triggers:   TriggerState{SprintCount: 3},
			comparison: ComparisonSummary{HasResult: true, WinnerPolicy: WinnerTameFlowCognitive},
			want:       ManagerTakeaways,
		},
		{
			name:       "does not trigger without FiveFocusing seen",
			view:       ViewComparison,
			state:      State{SeenMap: map[LessonID]bool{Orientation: true, UncertaintyConstraint: true}},
			triggers:   TriggerState{SprintCount: 0}, // SprintCount: 0 to avoid UC22 triggering
			comparison: ComparisonSummary{HasResult: true},
			want:       PolicyComparison, // Falls through to view-based
		},
		{
			name:       "does not trigger without comparison result",
			view:       ViewComparison,
			state:      State{SeenMap: map[LessonID]bool{Orientation: true, UncertaintyConstraint: true, ConstraintHunt: true, FiveFocusing: true}},
			triggers:   TriggerState{SprintCount: 3},
			comparison: ComparisonSummary{HasResult: false},
			want:       PolicyComparison,
		},
		{
			name:       "does not trigger on other views",
			view:       ViewExecution,
			state:      State{SeenMap: map[LessonID]bool{Orientation: true, UncertaintyConstraint: true, ConstraintHunt: true, FiveFocusing: true}},
			triggers:   TriggerState{SprintCount: 3},
			comparison: ComparisonSummary{HasResult: true},
			want:       FeverChart,
		},
		{
			name:       "does not trigger if already seen",
			view:       ViewComparison,
			state:      State{SeenMap: map[LessonID]bool{Orientation: true, UncertaintyConstraint: true, ConstraintHunt: true, FiveFocusing: true, ManagerTakeaways: true}},
			triggers:   TriggerState{SprintCount: 3},
			comparison: ComparisonSummary{HasResult: true},
			want:       PolicyComparison,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Select(tt.view, tt.state, true, tt.comparison.HasResult, tt.triggers, tt.comparison)
			if got.ID != tt.want {
				t.Errorf("Select() = %v, want %v", got.ID, tt.want)
			}
		})
	}
}

// TestManagerTakeawaysLesson_DynamicContent verifies dynamic question generation.
func TestManagerTakeawaysLesson_DynamicContent(t *testing.T) {
	cmp := ComparisonSummary{
		HasResult:     true,
		WinnerPolicy:  WinnerTameFlowCognitive,
		LeadTimeA:     5.0,
		LeadTimeB:     3.5,
		LeadTimeDelta: 1.5,
		CFRA:          0.15,
		CFRB:          0.08,
		CFRDelta:      0.07,
		WinsA:         1,
		WinsB:         3,
	}

	lesson := ManagerTakeawaysLesson(cmp)

	if lesson.ID != ManagerTakeaways {
		t.Errorf("ID = %v, want %v", lesson.ID, ManagerTakeaways)
	}
	if lesson.Title == "" {
		t.Error("expected non-empty title")
	}
	// Content should mention the winner
	if !containsString(lesson.Content, "TameFlow") {
		t.Error("expected content to mention TameFlow winner")
	}
	// Should have tips
	if len(lesson.Tips) == 0 {
		t.Error("expected non-empty tips")
	}
}

// TestGenerateMondayQuestions_EdgeCases tests question generation edge cases.
func TestGenerateMondayQuestions_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		cmp  ComparisonSummary
	}{
		{
			name: "zero CFRDelta",
			cmp:  ComparisonSummary{HasResult: true, WinnerPolicy: WinnerTie, CFRDelta: 0},
		},
		{
			name: "extreme lead time delta",
			cmp:  ComparisonSummary{HasResult: true, WinnerPolicy: WinnerDORAStrict, LeadTimeDelta: 10.0},
		},
		{
			name: "tie scenario",
			cmp:  ComparisonSummary{HasResult: true, WinnerPolicy: WinnerTie, WinsA: 2, WinsB: 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			questions := generateMondayQuestions(tt.cmp)
			// Should always return 3 non-empty questions
			for i, q := range questions {
				if q == "" {
					t.Errorf("question[%d] is empty", i)
				}
			}
		})
	}
}

// TestExperimentSummary tests winner description generation.
func TestExperimentSummary(t *testing.T) {
	tests := []struct {
		name   string
		cmp    ComparisonSummary
		wantIn string
	}{
		{
			name:   "DORA winner",
			cmp:    ComparisonSummary{WinnerPolicy: WinnerDORAStrict, WinsA: 3, WinsB: 1},
			wantIn: "DORA-Strict",
		},
		{
			name:   "TameFlow winner",
			cmp:    ComparisonSummary{WinnerPolicy: WinnerTameFlowCognitive, WinsA: 1, WinsB: 3},
			wantIn: "TameFlow",
		},
		{
			name:   "tie",
			cmp:    ComparisonSummary{WinnerPolicy: WinnerTie, WinsA: 2, WinsB: 2},
			wantIn: "tie",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := experimentSummary(tt.cmp)
			if !containsString(got, tt.wantIn) {
				t.Errorf("experimentSummary() = %q, want to contain %q", got, tt.wantIn)
			}
		})
	}
}

// TestTotalLessons - PRUNED per Khorikov: trivial constant, compiler catches mismatches

// BenchmarkGenerateMondayQuestions benchmarks question generation.
// Per Go Dev Guide: calculation functions need benchmark baselines.
func BenchmarkGenerateMondayQuestions(b *testing.B) {
	cmp := ComparisonSummary{
		HasResult:     true,
		WinnerPolicy:  WinnerTameFlowCognitive,
		LeadTimeDelta: 1.5,
		CFRDelta:      0.07,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = generateMondayQuestions(cmp)
	}
}

// BenchmarkExperimentSummary benchmarks summary generation.
func BenchmarkExperimentSummary(b *testing.B) {
	cmp := ComparisonSummary{
		WinnerPolicy: WinnerTameFlowCognitive,
		WinsA:        1,
		WinsB:        3,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = experimentSummary(cmp)
	}
}

// containsString is a test helper.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ { // justified:IX
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestState_WithVisible - PRUNED per Khorikov: trivial setter, can't meaningfully fail
