// Package lessons provides teaching content for the simulation tutorial.
// Shared between TUI and API to avoid import cycles.
package lessons

// LessonID identifies a specific teaching concept.
type LessonID string

const (
	Orientation           LessonID = "orientation"
	Understanding         LessonID = "understanding"
	FeverChart            LessonID = "fever-chart"
	DORAMetrics           LessonID = "dora-metrics"
	PolicyComparison      LessonID = "policy-comparison"
	VarianceExpected      LessonID = "variance-expected"
	PhaseProgress         LessonID = "phase-progress"
	VarianceAnalysis      LessonID = "variance-analysis"
	UncertaintyConstraint LessonID = "uncertainty-constraint" // UC19
	ConstraintHunt        LessonID = "constraint-hunt"        // UC20
	ExploitFirst          LessonID = "exploit-first"          // UC21
	FiveFocusing          LessonID = "five-focusing"          // UC22
	ManagerTakeaways      LessonID = "manager-takeaways"      // UC23
)

// TotalLessons is the number of unique teaching concepts.
const TotalLessons = 13

// Winner policy constants for ComparisonSummary.
// Per Go Dev Guide: avoid magic strings.
const (
	WinnerDORAStrict        = "DORA-Strict"
	WinnerTameFlowCognitive = "TameFlow-Cognitive"
	WinnerTie               = "TIE"
)

// ComparisonSummary holds comparison results for lesson selection.
// Uses primitive fields only to avoid import cycles (lessons can't import metrics).
// Data: Transient UI state, not event-sourced.
type ComparisonSummary struct {
	HasResult     bool
	WinnerPolicy  string  // WinnerDORAStrict, WinnerTameFlowCognitive, or WinnerTie
	LeadTimeA     float64 // DORA-Strict lead time
	LeadTimeB     float64 // TameFlow-Cognitive lead time
	LeadTimeDelta float64 // Difference (A - B)
	CFRA          float64 // DORA-Strict change fail rate
	CFRB          float64 // TameFlow-Cognitive change fail rate
	CFRDelta      float64 // Difference (A - B)
	WinsA         int     // Metrics won by DORA-Strict
	WinsB         int     // Metrics won by TameFlow-Cognitive
}

// Lesson contains teaching content for one concept.
type Lesson struct {
	ID      LessonID
	Title   string
	Content string
	Tips    []string
}

// State tracks lesson panel visibility and progress.
// Data: immutable value type with copy-on-write semantics.
//
// FP justification: State uses Go's copy-on-write pattern (With* methods)
// which is functionally equivalent to immutable transformations. The struct
// is passed by value, mutations happen on the copy, and the new value is
// returned. This achieves FP immutability guarantees within Go's idioms.
// See Go Dev Guide §Value Semantics and FP Guide §7 (Immutability).
type State struct {
	Visible bool
	SeenMap map[LessonID]bool
	Current LessonID
}

// WithVisible returns a copy with visibility set.
// Calculation: (State, bool) → State
func (s State) WithVisible(v bool) State {
	s.Visible = v
	return s
}

// WithSeen returns a copy marking a lesson as seen.
// Calculation: (State, LessonID) → State
// Note: Lazy map initialization is safe because Go copies struct by value;
// the new map is created in the copy, not mutating the original.
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

// TriggerState holds event-based triggers for contextual lessons.
// Triggers override view-based selection when their conditions are met.
// Each trigger fires at most once per session (Select checks SeenMap).
type TriggerState struct {
	HasRedBufferWithLowTicket bool // UC19: buffer >66% consumed + LOW understanding ticket
	HasQueueImbalance         bool // UC20: any phase queue > 2× average
	HasHighChildVariance      bool // UC21: decomposed ticket child actual/estimate > 1.3
	SprintCount               int  // UC22: 3+ sprints triggers FiveFocusing
}

// Select chooses the appropriate lesson based on current context.
// Calculation: (ViewContext, State, bool, bool, TriggerState, ComparisonSummary) → Lesson
// Select is idempotent via SeenMap checks—same inputs produce same outputs.
func Select(view ViewContext, state State, hasActiveSprint bool, hasComparisonResult bool, triggers TriggerState, comparison ComparisonSummary) Lesson {
	// First time: orientation
	if !state.SeenMap[Orientation] {
		return OrientationLesson()
	}

	// UC19: Aha moment - understanding IS the constraint
	if triggers.HasRedBufferWithLowTicket && !state.SeenMap[UncertaintyConstraint] {
		return UncertaintyConstraintLesson()
	}

	// UC20: Constraint Hunt - queue imbalance shows symptoms
	if triggers.HasQueueImbalance && state.SeenMap[UncertaintyConstraint] && !state.SeenMap[ConstraintHunt] {
		return ConstraintHuntLesson()
	}

	// UC21: Exploit First - decomposition didn't fix uncertainty
	if triggers.HasHighChildVariance && state.SeenMap[UncertaintyConstraint] && !state.SeenMap[ExploitFirst] {
		return ExploitFirstLesson()
	}

	// UC22: Five Focusing Steps - synthesis after experiencing constraints
	// Prereq: 3+ sprints AND (UC20 OR UC21 seen)
	hasSeenSymptomLesson := state.SeenMap[ConstraintHunt] || state.SeenMap[ExploitFirst]
	if triggers.SprintCount >= 3 && hasSeenSymptomLesson && !state.SeenMap[FiveFocusing] {
		return FiveFocusingLesson()
	}

	// UC23: Manager Takeaways - dynamic synthesis with comparison metrics
	// Prereq: ViewComparison + HasResult + UC22 seen
	if view == ViewComparison && comparison.HasResult && state.SeenMap[FiveFocusing] && !state.SeenMap[ManagerTakeaways] {
		return ManagerTakeawaysLesson(comparison)
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

func UncertaintyConstraintLesson() Lesson {
	return Lesson{
		ID:    UncertaintyConstraint,
		Title: "Understanding IS the Constraint",
		Content: `Buffer consumed! This LOW understanding ticket caused the variance.

KEY INSIGHT: Your constraint isn't capacity—it's what you don't know yet.

LOW understanding (±50% variance):
  3-day estimate → actual 1.5-6.0 days possible

The buffer protects the commitment, but uncertainty eats it.
This is why "just work faster" doesn't fix missed sprints.`,
		Tips: []string{
			"Watch which tickets consume buffer",
			"HIGH understanding = predictable",
			"Decomposition can improve understanding",
		},
	}
}

func ConstraintHuntLesson() Lesson {
	return Lesson{
		ID:    ConstraintHunt,
		Title: "Finding the Constraint",
		Content: `Queue depth shows WHERE work piles up.
Understanding level shows WHY.

SYMPTOM: Long queue at a phase
ROOT CAUSE: LOW understanding tickets block flow

Look at the tickets in the longest queue.
Are they HIGH or LOW understanding?

The constraint isn't the phase—it's uncertainty.`,
		Tips: []string{
			"Queue depth = symptom",
			"Understanding = root cause",
			"Focus on WHY, not WHERE",
		},
	}
}

func ExploitFirstLesson() Lesson {
	return Lesson{
		ID:    ExploitFirst,
		Title: "Exploit Before Elevate",
		Content: `Splitting didn't fix uncertainty!

The decomposed ticket's children ALSO had high variance.
This means the split was ELEVATION (more work) not EXPLOITATION.

TOC's Five Focusing Steps:
1. IDENTIFY the constraint
2. EXPLOIT it (get more from what you have)
3. SUBORDINATE everything else
4. ELEVATE only if exploitation isn't enough
5. Repeat

Exploitation = improve understanding BEFORE splitting.
Elevation = split to add capacity.

You elevated without exploiting first.`,
		Tips: []string{
			"Exploit = improve understanding",
			"Elevate = add capacity (split)",
			"Research before decomposition",
		},
	}
}

// FiveFocusingLesson returns the TOC Five Focusing Steps framework lesson (UC22).
// Calculation: () → Lesson
func FiveFocusingLesson() Lesson {
	return Lesson{
		ID:    FiveFocusing,
		Title: "The Five Focusing Steps",
		Content: `You've now experienced the constraint cycle multiple times.

TOC's FIVE FOCUSING STEPS:

1. IDENTIFY the constraint
   What limits throughput? (Hint: it's uncertainty)

2. EXPLOIT the constraint
   Get more from what you have BEFORE adding resources.
   → Research, spike, clarify requirements

3. SUBORDINATE everything else
   Non-constraints should support the constraint.
   → Don't start new LOW tickets while one is blocking

4. ELEVATE the constraint
   Only if exploitation isn't enough, add capacity.
   → Decompose, add people, buy tools

5. REPEAT
   The constraint moves. Find the new one.

Most teams skip step 2 and jump to elevation.
That's why "just add more people" doesn't fix flow.`,
		Tips: []string{
			"Exploit = improve understanding first",
			"Elevate = add capacity (last resort)",
			"The constraint moves—keep looking",
		},
	}
}

// ManagerTakeawaysLesson returns dynamic synthesis lesson with comparison metrics (UC23).
// Calculation: ComparisonSummary → Lesson
// Generates contextual "Monday morning" questions using actual comparison data.
func ManagerTakeawaysLesson(cmp ComparisonSummary) Lesson {
	questions := generateMondayQuestions(cmp)
	summary := experimentSummary(cmp)

	content := `MONDAY MORNING QUESTIONS

` + summary + `

Ask yourself these questions before your next planning session:

1. ` + questions[0] + `

2. ` + questions[1] + `

3. ` + questions[2] + `

These questions help you transfer simulation insights to real work.
The goal isn't to copy the simulation—it's to see your backlog differently.`

	return Lesson{
		ID:      ManagerTakeaways,
		Title:   "Transfer to Practice",
		Content: content,
		Tips: []string{
			"Review LOW understanding tickets first",
			"Ask: 'What don't we know yet?'",
			"Run a comparison before each planning session",
		},
	}
}

// generateMondayQuestions creates 3 contextual questions from comparison data.
// Calculation: ComparisonSummary → [3]string
// "Monday questions" are actionable prompts for real-world planning sessions.
func generateMondayQuestions(cmp ComparisonSummary) [3]string {
	var questions [3]string

	// Question 1: Based on winner
	switch cmp.WinnerPolicy {
	case WinnerTameFlowCognitive:
		questions[0] = "Which tickets in my backlog have LOW understanding that I'm treating as 'just needs implementation'?"
	case WinnerDORAStrict:
		questions[0] = "Which large tickets might have hidden uncertainty that size-based decomposition would catch?"
	default: // WinnerTie
		questions[0] = "What would help me identify which sizing approach works better for MY team's backlog?"
	}

	// Question 2: Based on lead time delta
	if cmp.LeadTimeDelta > 1.0 {
		questions[1] = "What's causing the lead time difference? Is it queue time, work time, or rework?"
	} else if cmp.LeadTimeDelta < -1.0 {
		questions[1] = "The faster policy had shorter lead times—am I measuring the right thing?"
	} else {
		questions[1] = "Lead times were similar—where else might the policies differ in my real backlog?"
	}

	// Question 3: Based on CFR delta
	if cmp.CFRDelta > 0.05 {
		questions[2] = "The higher change fail rate suggests more incidents—what's causing the rework?"
	} else if cmp.CFRDelta < -0.05 {
		questions[2] = "Lower incidents came with a trade-off—what was it, and is it worth it?"
	} else {
		questions[2] = "Similar failure rates mean the policies differ elsewhere—where do I see variance in my team?"
	}

	return questions
}

// experimentSummary describes the comparison outcome in plain language.
// Calculation: ComparisonSummary → string
func experimentSummary(cmp ComparisonSummary) string {
	switch cmp.WinnerPolicy {
	case WinnerTameFlowCognitive:
		return "TameFlow-Cognitive won this comparison, suggesting understanding-based sizing outperformed time-based sizing for this backlog profile."
	case WinnerDORAStrict:
		return "DORA-Strict won this comparison, suggesting time-based sizing caught risks that understanding-based sizing missed."
	default: // WinnerTie
		return "The policies tied, meaning neither approach had a clear advantage for this backlog profile."
	}
}
