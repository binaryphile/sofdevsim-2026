package model

// WorkflowPhase represents the 8-phase ticket workflow from the Unified Workflow Rubric
type WorkflowPhase int

const (
	PhaseBacklog   WorkflowPhase = iota // Not started
	PhaseResearch                       // Phase 1
	PhaseSizing                         // Phase 2
	PhasePlanning                       // Phase 3
	PhaseImplement                      // Phase 4
	PhaseVerify                         // Phase 5
	PhaseCICD                           // Phase 6
	PhaseReview                         // Phase 7
	PhaseDone                           // Phase 8
)

func (p WorkflowPhase) String() string {
	return [...]string{
		"Backlog",
		"Research",
		"Sizing",
		"Planning",
		"Implement",
		"Verify",
		"CI/CD",
		"Review",
		"Done",
	}[p]
}

// UnderstandingLevel represents how well the team understands the work
// This is TameFlow's core discriminant
type UnderstandingLevel int

const (
	LowUnderstanding    UnderstandingLevel = iota // "We have no idea"
	MediumUnderstanding                           // "Roughly know"
	HighUnderstanding                             // "Yeah, we can do it"
)

func (u UnderstandingLevel) String() string {
	return [...]string{"Low", "Medium", "High"}[u]
}

// FeverStatus represents the buffer consumption state in TameFlow
type FeverStatus int

const (
	FeverGreen  FeverStatus = iota // <33% buffer consumed
	FeverYellow                    // 33-66% consumed
	FeverRed                       // >66% consumed
)

func (f FeverStatus) String() string {
	return [...]string{"Green", "Yellow", "Red"}[f]
}

// Severity represents incident severity levels
type Severity int

const (
	SeverityLow Severity = iota
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

func (s Severity) String() string {
	return [...]string{"Low", "Medium", "High", "Critical"}[s]
}

// SizingPolicy represents the decomposition strategy being tested
type SizingPolicy int

const (
	PolicyNone             SizingPolicy = iota // No decomposition
	PolicyDORAStrict                           // Decompose if estimate > 5 days
	PolicyTameFlowCognitive                    // Decompose if understanding = Low
	PolicyHybrid                               // Decompose if estimate > 5 AND understanding < High
)

func (p SizingPolicy) String() string {
	return [...]string{"None", "DORA-Strict", "TameFlow-Cognitive", "Hybrid"}[p]
}

// EventType represents simulation events
type EventType int

const (
	EventTicketComplete EventType = iota
	EventPhaseAdvance
	EventBugDiscovered
	EventBlocker
	EventScopeCreep
	EventIncident
	EventIncidentResolved
)

func (e EventType) String() string {
	return [...]string{
		"TicketComplete",
		"PhaseAdvance",
		"BugDiscovered",
		"Blocker",
		"ScopeCreep",
		"Incident",
		"IncidentResolved",
	}[e]
}
