// Package export provides CSV and HTML export of simulation data.
package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/lessons"
)

// =============================================================================
// Data Types for HTML Export
// =============================================================================

// Data: SimulationParams holds simulation configuration for export.
type SimulationParams struct {
	Seed           int64
	Policy         string
	DeveloperCount int
}

// Data: TrackerData holds metrics data for export.
type TrackerData struct {
	LeadTime        float64
	DeployFrequency float64
	ChangeFailRate  float64
	MTTR            float64
	LeadTimeHistory []float64
	FeverHistory    []float64
}

// Data: ComparisonSummary holds comparison results for export.
type ComparisonSummary struct {
	PolicyA     string
	PolicyB     string
	Winner      string
	LeadTimeA   float64
	LeadTimeB   float64
	DeployFreqA float64
	DeployFreqB float64
}

// Data: HTMLExporter generates shareable HTML reports.
type HTMLExporter struct {
	sim         SimulationParams
	tracker     TrackerData
	lessonsSeen map[lessons.LessonID]bool
	comparison  *ComparisonSummary
}

// Calculation: creates a new HTMLExporter.
func NewHTMLExporter(sim SimulationParams, tracker TrackerData, lessonsSeen map[lessons.LessonID]bool, comparison *ComparisonSummary) HTMLExporter {
	return HTMLExporter{
		sim:         sim,
		tracker:     tracker,
		lessonsSeen: lessonsSeen,
		comparison:  comparison,
	}
}

// Calculation: generates complete HTML report (pure, no I/O).
func (e HTMLExporter) GenerateHTML() string {
	var b strings.Builder

	b.WriteString(e.htmlHeader())
	b.WriteString(e.parametersSection())
	b.WriteString(e.metricsSection())
	b.WriteString(e.bufferSection())
	b.WriteString(e.lessonsSection())
	b.WriteString(e.questionsSection())
	if e.comparison != nil {
		b.WriteString(e.comparisonSection())
	}
	b.WriteString(e.htmlFooter())

	return b.String()
}

// Action: writes HTML report to file (I/O side effect).
// Creates parent directories if they don't exist.
func (e HTMLExporter) ExportToFile(path string) error {
	// Create parent directories if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	html := e.GenerateHTML()
	if err := os.WriteFile(path, []byte(html), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

func (e HTMLExporter) htmlHeader() string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Simulation Report - %s</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 800px; margin: 0 auto; padding: 2rem; }
    h1 { color: #1f2937; }
    h2 { color: #374151; border-bottom: 1px solid #e5e7eb; padding-bottom: 0.5rem; }
    .metric-card { padding: 1rem; border: 1px solid #e5e7eb; border-radius: 0.5rem; margin: 0.5rem 0; }
    .metric-value { font-size: 1.5rem; font-weight: 600; }
    .timeline { display: flex; gap: 2px; margin: 1rem 0; }
    .bar { width: 20px; height: 30px; }
    .%s { background-color: %s; }
    .%s { background-color: %s; }
    .%s { background-color: %s; }
    .question-item { padding: 0.5rem; background: #f9fafb; margin-bottom: 0.5rem; border-radius: 0.25rem; }
    .lesson { margin: 1rem 0; padding: 1rem; background: #f3f4f6; border-radius: 0.5rem; }
    .placeholder { color: #6b7280; font-style: italic; }
    table { width: 100%%; border-collapse: collapse; margin: 1rem 0; }
    th, td { padding: 0.5rem; text-align: left; border-bottom: 1px solid #e5e7eb; }
    th { background: #f9fafb; }
    .winner { font-weight: 600; color: #059669; }
  </style>
</head>
<body>
  <h1>Simulation Report</h1>
`, time.Now().Format("2006-01-02"),
		CSSClassBufferGreen, ColorBufferGreen,
		CSSClassBufferYellow, ColorBufferYellow,
		CSSClassBufferRed, ColorBufferRed)
}

func (e HTMLExporter) htmlFooter() string {
	return fmt.Sprintf(`
  <footer style="margin-top: 2rem; padding-top: 1rem; border-top: 1px solid #e5e7eb; color: #6b7280; font-size: 0.875rem;">
    Generated: %s
  </footer>
</body>
</html>`, time.Now().Format("2006-01-02 15:04:05"))
}

func (e HTMLExporter) parametersSection() string {
	return fmt.Sprintf(`
  <section id="parameters">
    <h2>Simulation Parameters</h2>
    <dl>
      <dt>Seed</dt><dd>%d</dd>
      <dt>Policy</dt><dd>%s</dd>
      <dt>Developers</dt><dd>%d</dd>
    </dl>
  </section>
`, e.sim.Seed, e.sim.Policy, e.sim.DeveloperCount)
}

func (e HTMLExporter) metricsSection() string {
	leadSparkline := generateSparklineSVG(e.tracker.LeadTimeHistory)
	return fmt.Sprintf(`
  <section id="metrics">
    <h2>DORA Metrics</h2>
    <div class="metric-card">
      <h3>Lead Time</h3>
      <span class="metric-value">%.1f days</span>
      %s
    </div>
    <div class="metric-card">
      <h3>Deploy Frequency</h3>
      <span class="metric-value">%.1f/sprint</span>
    </div>
    <div class="metric-card">
      <h3>Change Fail Rate</h3>
      <span class="metric-value">%.0f%%</span>
    </div>
    <div class="metric-card">
      <h3>MTTR</h3>
      <span class="metric-value">%.1f days</span>
    </div>
  </section>
`, e.tracker.LeadTime, leadSparkline, e.tracker.DeployFrequency, e.tracker.ChangeFailRate*100, e.tracker.MTTR)
}

func (e HTMLExporter) bufferSection() string {
	timeline := bufferTimelineHTML(e.tracker.FeverHistory)
	return fmt.Sprintf(`
  <section id="buffer">
    <h2>Buffer Consumption</h2>
    %s
  </section>
`, timeline)
}

func (e HTMLExporter) lessonsSection() string {
	triggered := triggeredLessons(e.lessonsSeen)
	if len(triggered) == 0 {
		return `
  <section id="lessons">
    <h2>Lessons Learned</h2>
    <p class="placeholder">No lessons triggered — run simulation to see insights</p>
  </section>
`
	}

	var b strings.Builder
	b.WriteString(`
  <section id="lessons">
    <h2>Lessons Learned</h2>
`)
	for _, lesson := range triggered {
		b.WriteString(fmt.Sprintf(`    <article class="lesson">
      <h3>%s</h3>
      <p>%s</p>
    </article>
`, lesson.Title, lesson.Content))
	}
	b.WriteString("  </section>\n")
	return b.String()
}

func (e HTMLExporter) questionsSection() string {
	questions := transferQuestions(e.lessonsSeen)
	if len(questions) == 0 {
		return `
  <section id="questions">
    <h2>Monday Morning Questions</h2>
    <p class="placeholder">Complete more lessons to unlock transfer questions</p>
  </section>
`
	}

	var b strings.Builder
	b.WriteString(`
  <section id="questions">
    <h2>Monday Morning Questions</h2>
    <ul>
`)
	for _, q := range questions {
		b.WriteString(fmt.Sprintf(`      <li class="question-item">%s</li>
`, q))
	}
	b.WriteString("    </ul>\n  </section>\n")
	return b.String()
}

func (e HTMLExporter) comparisonSection() string {
	if e.comparison == nil {
		return ""
	}
	return fmt.Sprintf(`
  <section id="comparison">
    <h2>Policy Comparison</h2>
    <table>
      <tr><th>Metric</th><th>%s</th><th>%s</th><th>Winner</th></tr>
      <tr>
        <td>Lead Time</td>
        <td>%.1f days</td>
        <td>%.1f days</td>
        <td class="winner">%s</td>
      </tr>
      <tr>
        <td>Deploy Frequency</td>
        <td>%.1f/sprint</td>
        <td>%.1f/sprint</td>
        <td class="winner">%s</td>
      </tr>
    </table>
    <p><strong>Overall Winner:</strong> <span class="winner">%s</span></p>
  </section>
`, e.comparison.PolicyA, e.comparison.PolicyB,
		e.comparison.LeadTimeA, e.comparison.LeadTimeB, leadTimeWinner(e.comparison),
		e.comparison.DeployFreqA, e.comparison.DeployFreqB, deployFreqWinner(e.comparison),
		e.comparison.Winner)
}

// Calculation: determines lead time winner (lower is better).
func leadTimeWinner(c *ComparisonSummary) string {
	if c.LeadTimeA < c.LeadTimeB {
		return c.PolicyA
	}
	return c.PolicyB
}

// Calculation: determines deploy frequency winner (higher is better).
func deployFreqWinner(c *ComparisonSummary) string {
	if c.DeployFreqA > c.DeployFreqB {
		return c.PolicyA
	}
	return c.PolicyB
}

// =============================================================================
// Constants (no magic strings per Go Dev Guide)
// =============================================================================

const (
	// CSS class names
	CSSClassBufferGreen  = "buffer-green"
	CSSClassBufferYellow = "buffer-yellow"
	CSSClassBufferRed    = "buffer-red"

	// Color hex values
	ColorBufferGreen  = "#22c55e"
	ColorBufferYellow = "#eab308"
	ColorBufferRed    = "#ef4444"
	ColorTrendStable  = "#6b7280"

	// SVG dimensions
	SparklineWidth  = 80
	SparklineHeight = 20

	// SVG namespace
	SVGNamespace = "http://www.w3.org/2000/svg"
)

// =============================================================================
// Triggered Lessons
// =============================================================================

// lessonGetters maps lesson IDs to their constructor functions.
// Used by triggeredLessons to retrieve lesson content.
var lessonGetters = map[lessons.LessonID]func() lessons.Lesson{
	lessons.Orientation:           lessons.OrientationLesson,
	lessons.Understanding:         lessons.UnderstandingLesson,
	lessons.FeverChart:            lessons.FeverChartLesson,
	lessons.DORAMetrics:           lessons.DORAMetricsLesson,
	lessons.PolicyComparison:      lessons.PolicyComparisonLesson,
	lessons.VarianceExpected:      lessons.VarianceExpectedLesson,
	lessons.PhaseProgress:         lessons.PhaseProgressLesson,
	lessons.VarianceAnalysis:      lessons.VarianceAnalysisLesson,
	lessons.UncertaintyConstraint: lessons.UncertaintyConstraintLesson,
	lessons.ConstraintHunt:        lessons.ConstraintHuntLesson,
	lessons.ExploitFirst:          lessons.ExploitFirstLesson,
	lessons.FiveFocusing:          lessons.FiveFocusingLesson,
	// ManagerTakeaways requires ComparisonSummary, handled separately
}

// Calculation: returns lessons that were triggered during session.
// Boundary: nil/empty map → empty slice (nil-safe).
func triggeredLessons(seen map[lessons.LessonID]bool) []lessons.Lesson {
	if len(seen) == 0 {
		return nil
	}

	var result []lessons.Lesson
	for id, wasSeen := range seen {
		if !wasSeen {
			continue
		}
		if getter, ok := lessonGetters[id]; ok {
			result = append(result, getter())
		}
		// Note: ManagerTakeaways is excluded since it requires ComparisonSummary
	}
	return result
}

// =============================================================================
// Buffer Timeline Generation
// =============================================================================

// Calculation: generates color-coded buffer timeline HTML.
// Boundary: empty/nil history → placeholder message.
// Values are interpreted as percentage consumed (0-100).
// Colors: green (<33%), yellow (33-66%), red (>66%).
func bufferTimelineHTML(history []float64) string {
	if len(history) == 0 {
		return `<p class="placeholder">No buffer data — run a sprint first</p>`
	}

	var bars []string
	for _, pct := range history {
		class := classForBufferPercent(pct)
		bars = append(bars, fmt.Sprintf(`<div class="bar %s"></div>`, class))
	}

	return fmt.Sprintf(`<div class="timeline">%s</div>`, strings.Join(bars, ""))
}

// Calculation: returns CSS class for buffer consumption percentage.
// Boundary: values outside 0-100 are clamped to nearest threshold.
func classForBufferPercent(pct float64) string {
	switch {
	case pct < 33.0:
		return CSSClassBufferGreen
	case pct < 67.0:
		return CSSClassBufferYellow
	default:
		return CSSClassBufferRed
	}
}

// =============================================================================
// Transfer Questions
// =============================================================================

// transferQuestionMap maps lesson IDs to their transfer questions.
// These are "Monday morning" questions to help managers apply insights.
var transferQuestionMap = map[lessons.LessonID][]string{
	lessons.UncertaintyConstraint: {
		"Which tickets on your backlog have LOW understanding? What would decomposition reveal?",
	},
	lessons.ConstraintHunt: {
		"Where are queues building up in your workflow? Is that actually the constraint?",
	},
	lessons.ExploitFirst: {
		"Are you protecting your constraint from interruptions and context switches?",
	},
	lessons.FiveFocusing: {
		"Have you identified and exploited the constraint before trying to elevate it?",
	},
	lessons.ManagerTakeaways: {
		"Which work items have the highest uncertainty? Can they be decomposed?",
		"Where is work piling up? Is that your actual constraint?",
		"What would happen if you stopped starting and started finishing?",
	},
}

// Calculation: returns dynamic questions based on lessons seen.
// Boundary: nil/empty map → empty slice (nil-safe).
func transferQuestions(seen map[lessons.LessonID]bool) []string {
	if len(seen) == 0 {
		return nil
	}

	var questions []string
	for id, wasSeen := range seen {
		if !wasSeen {
			continue
		}
		if qs, ok := transferQuestionMap[id]; ok {
			questions = append(questions, qs...)
		}
	}
	return questions
}

// =============================================================================
// SVG Sparkline Generation
// =============================================================================

// Calculation: generates inline SVG sparkline from data points.
// Boundary: len < 2 → empty string, all identical → flat line at midpoint.
// Returns empty string if insufficient data for a meaningful trend line.
func generateSparklineSVG(data []float64) string {
	if len(data) < 2 {
		return ""
	}

	// Find min/max for normalization
	minVal, maxVal := data[0], data[0]
	for _, v := range data {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	// Boundary defense: avoid div-by-zero when all values identical
	rangeVal := maxVal - minVal
	if rangeVal == 0 {
		rangeVal = 1 // Flat line at midpoint
	}

	// Calculate X spacing
	xStep := float64(SparklineWidth) / float64(len(data)-1)

	// Generate points string: "x1,y1 x2,y2 ..."
	var points []string
	for i, v := range data {
		x := float64(i) * xStep
		// Normalize Y: 0 at top, height at bottom (SVG coordinates)
		// y = height - (v - min) / range * height
		normalizedY := (v - minVal) / rangeVal
		y := float64(SparklineHeight) - (normalizedY * float64(SparklineHeight))

		// Handle flat line case: center vertically
		if maxVal == minVal {
			y = float64(SparklineHeight) / 2
		}

		points = append(points, fmt.Sprintf("%.1f,%.1f", x, y))
	}

	// Build SVG element
	return fmt.Sprintf(
		`<svg width="%d" height="%d" xmlns="%s">`+
			`<polyline points="%s" stroke="%s" stroke-width="2" fill="none"/>`+
			`</svg>`,
		SparklineWidth, SparklineHeight, SVGNamespace,
		strings.Join(points, " "),
		ColorTrendStable,
	)
}
