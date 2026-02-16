package tui

// HeaderVM — data for the header bar.
type HeaderVM struct {
	CurrentView    View
	Policy         string
	Paused         bool
	CurrentTick    int
	BacklogCount   int
	CompletedCount int
	Seed           int64
	Width          int
}

// HelpVM — data for the help bar.
type HelpVM struct {
	CurrentView View
}

// BacklogTicketVM — one row in the planning backlog table.
type BacklogTicketVM struct {
	ID            string
	Title         string
	EstimatedDays float64
	Understanding string
	Phase         string
	Selected      bool
}

// PlanningVM — data for the planning view.
type PlanningVM struct {
	Tickets     []BacklogTicketVM
	OfficeState OfficeState
	DevNames    []string
	Width       int
	Height      int
}

// SprintProgressVM — sprint progress bar data.
type SprintProgressVM struct {
	HasSprint    bool
	SprintNumber int
	DaysElapsed  int
	DurationDays int
	Progress     float64
}

// ActiveWorkRowVM — one developer's current work.
type ActiveWorkRowVM struct {
	DevName  string
	TicketID string
	Progress float64
	Phase    string
	IsIdle   bool
}

// FeverVM — fever chart panel data.
type FeverVM struct {
	HasSprint   bool
	WorkPct     float64
	BufferPct   float64
	RatioStr    string
	Zone        string
	Remaining   float64
	TotalBuffer float64
}

// EventVM — one event row.
type EventVM struct {
	Day     int
	Message string
}

// ExecutionVM — data for the execution view.
type ExecutionVM struct {
	Sprint      SprintProgressVM
	ActiveWork  []ActiveWorkRowVM
	Fever       FeverVM
	Events      []EventVM
	OfficeState OfficeState
	DevNames    []string
	Width       int
	Height      int
}

// DORAMetricsVM — DORA metrics with sparkline history.
type DORAMetricsVM struct {
	LeadTimeAvg     float64
	DeployFrequency float64
	MTTRAvg         float64
	ChangeFailRate  float64
	LeadTimeHistory []float64
	DeployHistory   []float64
	MTTRHistory     []float64
	CFRHistory      []float64
}

// CompletedTicketVM — one row in completed tickets table.
type CompletedTicketVM struct {
	ID            string
	Title         string
	EstimatedDays float64
	ActualDays    float64
	Understanding string
}

// MetricsVM — data for the metrics view.
type MetricsVM struct {
	DORA             DORAMetricsVM
	CompletedTickets []CompletedTicketVM
	CompletedCount   int
	TotalIncidents   int
	Policy           string
	Width            int
}

// ComparisonVM — data for the comparison view.
type ComparisonVM struct {
	HasResult                        bool
	Seed                             int64
	PolicyAName, PolicyBName         string
	LeadTimeA, LeadTimeB             float64
	DeployFreqA, DeployFreqB         float64
	MTTRA, MTTRB                     float64
	CFRA, CFRB                       float64
	TicketsA, TicketsB               int
	IncidentsA, IncidentsB           int
	LeadTimeWinner, DeployFreqWinner string
	MTTRWinner, CFRWinner            string
	TicketsWinner, IncidentsWinner   string
	OverallWinner                    string
	IsTie                            bool
	WinsA, WinsB                     int
	Insight                          string
	Width                            int
}

// LessonVM — lesson panel data.
type LessonVM struct {
	Title    string
	Content  string
	Tips     []string
	Progress int
	Total    int
	Width    int
}
