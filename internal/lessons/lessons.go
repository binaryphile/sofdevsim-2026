// Package lessons provides teaching content for the simulation tutorial.
// Shared between TUI and API to avoid import cycles.
package lessons

// LessonID identifies a specific teaching concept.
type LessonID string

const (
	Orientation      LessonID = "orientation"
	Understanding    LessonID = "understanding"
	FeverChart       LessonID = "fever-chart"
	DORAMetrics      LessonID = "dora-metrics"
	PolicyComparison LessonID = "policy-comparison"
	VarianceExpected LessonID = "variance-expected"
	PhaseProgress    LessonID = "phase-progress"
	VarianceAnalysis LessonID = "variance-analysis"
)

// TotalLessons is the number of unique teaching concepts.
const TotalLessons = 8

// Lesson contains teaching content for one concept.
type Lesson struct {
	ID      LessonID
	Title   string
	Content string
	Tips    []string
}

// State tracks lesson panel visibility and progress.
// Value type - copy freely.
type State struct {
	Visible bool
	SeenMap map[LessonID]bool
	Current LessonID
}

// WithVisible returns a copy with visibility set.
func (s State) WithVisible(v bool) State {
	s.Visible = v
	return s
}

// WithSeen returns a copy marking a lesson as seen.
func (s State) WithSeen(id LessonID) State {
	if s.SeenMap == nil {
		s.SeenMap = make(map[LessonID]bool)
	}
	s.SeenMap[id] = true
	s.Current = id
	return s
}

// SeenCount returns number of unique lessons seen.
func (s State) SeenCount() int { return len(s.SeenMap) }

// ViewContext represents the current UI context for lesson selection.
type ViewContext int

const (
	ViewPlanning ViewContext = iota
	ViewExecution
	ViewMetrics
	ViewComparison
)

// Select chooses the appropriate lesson based on current context.
// Pure function: (view, state, hasActiveSprint, hasComparison) → Lesson
func Select(view ViewContext, state State, hasActiveSprint bool, hasComparisonResult bool) Lesson {
	// First time: orientation
	if !state.SeenMap[Orientation] {
		return OrientationLesson()
	}

	switch view {
	case ViewPlanning:
		return UnderstandingLesson()
	case ViewExecution:
		if hasActiveSprint {
			return FeverChartLesson()
		}
		return PhaseProgressLesson()
	case ViewMetrics:
		return DORAMetricsLesson()
	case ViewComparison:
		if hasComparisonResult {
			return PolicyComparisonLesson()
		}
		return ComparisonIntroLesson()
	}
	return OrientationLesson()
}

// --- Lesson Content Functions ---

func OrientationLesson() Lesson {
	return Lesson{
		ID:    Orientation,
		Title: "Welcome to the Simulation",
		Content: `This simulation teaches software delivery concepts.

KEY INSIGHT: Understanding determines variance.

• High understanding: ±5% (predictable)
• Medium understanding: ±20% (moderate)
• Low understanding: ±50% (unpredictable)

A 3-day LOW ticket is riskier than a 6-day HIGH ticket.`,
		Tips: []string{
			"Tab switches views",
			"Space pauses/resumes",
			"Press 'h' to toggle lessons",
		},
	}
}

func UnderstandingLesson() Lesson {
	return Lesson{
		ID:    Understanding,
		Title: "Understanding Levels",
		Content: `Each ticket has an understanding level that determines outcome predictability:

HIGH (±5%):
  4-day estimate → 3.8-4.2 days actual

MEDIUM (±20%):
  4-day estimate → 3.2-4.8 days actual

LOW (±50%):
  4-day estimate → 2.0-6.0 days actual

This is the simulation's core insight.`,
		Tips: []string{
			"Press 'd' to decompose tickets",
			"Decomposition improves understanding",
		},
	}
}

func FeverChartLesson() Lesson {
	return Lesson{
		ID:    FeverChart,
		Title: "Fever Chart (Buffer)",
		Content: `The fever chart tracks sprint buffer consumption:

GREEN (0-33%): On track
  Buffer absorbing normal variance.

YELLOW (33-66%): Warning
  Variance consuming buffer faster.

RED (66-100%): Danger
  Sprint at risk of overrun.

Buffer protects against variance. Without it, every delay causes schedule slip.`,
		Tips: []string{
			"Watch buffer vs sprint progress",
			"If buffer > progress, investigate",
		},
	}
}

func PhaseProgressLesson() Lesson {
	return Lesson{
		ID:    PhaseProgress,
		Title: "Ticket Phases",
		Content: `Each ticket progresses through phases:

1. Design     - Architecture decisions
2. Implement  - Writing code
3. Test       - Verification
4. CI/CD      - Pipeline execution
5. Deploy     - Release to production

Low-understanding tickets spend more time in each phase due to rework and discovery.`,
		Tips: []string{
			"Phase duration varies by understanding",
			"Incidents can occur in any phase",
		},
	}
}

func DORAMetricsLesson() Lesson {
	return Lesson{
		ID:    DORAMetrics,
		Title: "DORA Metrics",
		Content: `Four key metrics from DevOps Research:

LEAD TIME (↓ lower is better)
  Days from start to deploy.

DEPLOY FREQUENCY (↑ higher is better)
  Deploys per day.

MTTR (↓ lower is better)
  Mean Time To Recovery from incidents.

CHANGE FAIL RATE (↓ lower is better)
  Percent of deploys causing incidents.

Elite teams: <1 day lead time, >1/day deploys.`,
		Tips: []string{
			"Sparklines show trends over time",
			"Focus on trends, not absolutes",
		},
	}
}

func PolicyComparisonLesson() Lesson {
	return Lesson{
		ID:    PolicyComparison,
		Title: "Policy Comparison",
		Content: `Two decomposition policies:

DORA-Strict:
  Decompose tickets > 5 days.
  Assumes: time correlates with risk.

TameFlow-Cognitive:
  Decompose LOW understanding tickets.
  Assumes: understanding correlates with risk.

The comparison shows which assumption holds for this backlog.`,
		Tips: []string{
			"Press 'c' to run new comparison",
			"Different seeds may change winner",
		},
	}
}

func ComparisonIntroLesson() Lesson {
	return Lesson{
		ID:    PolicyComparison,
		Title: "Run a Comparison",
		Content: `Press 'c' to run a policy comparison.

The simulation will:
1. Create identical backlogs (same seed)
2. Run 3 sprints with DORA-Strict
3. Run 3 sprints with TameFlow-Cognitive
4. Compare DORA metrics

This reveals which sizing approach produces better flow.`,
		Tips: []string{
			"Press 'c' to start comparison",
			"Results appear in this view",
		},
	}
}

func VarianceExpectedLesson() Lesson {
	return Lesson{
		ID:    VarianceExpected,
		Title: "Expected Variance",
		Content: `When you assign a ticket, expect variance based on its understanding level:

HIGH: Estimate is reliable (±5%)
  A 4-day ticket finishes in 3.8-4.2 days.

MEDIUM: Some uncertainty (±20%)
  A 4-day ticket finishes in 3.2-4.8 days.

LOW: High uncertainty (±50%)
  A 4-day ticket could take 2-6 days.

Consider decomposing LOW tickets before assignment.`,
		Tips: []string{
			"Check understanding before assigning",
			"'d' decomposes the selected ticket",
		},
	}
}

func VarianceAnalysisLesson() Lesson {
	return Lesson{
		ID:    VarianceAnalysis,
		Title: "Variance Analysis",
		Content: `After sprint completion, compare actual vs estimated:

Variance Ratio = Actual / Estimated

Expected bounds by understanding:
• HIGH:   0.95 - 1.05
• MEDIUM: 0.80 - 1.20
• LOW:    0.50 - 1.50

Ratios outside bounds indicate:
• Incidents occurred
• Phase delays
• Discovery/rework

This validates the variance model.`,
		Tips: []string{
			"Review completed tickets in Metrics",
			"Look for patterns in outliers",
		},
	}
}
