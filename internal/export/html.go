// Package export provides CSV and HTML export of simulation data.
package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/binaryphile/fluentfp/slice"
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
	SprintCount    int
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
	b.WriteString(e.interpretationGuide())
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

// Calculation: generates interpretation guide section.
func (e HTMLExporter) interpretationGuide() string {
	return `
  <section id="guide">
    <h2>How to Read This Report</h2>
    <div class="card">
      <div class="guide-grid">
        <div class="guide-item">
          <div class="guide-icon">📊</div>
          <div class="guide-content">
            <div class="guide-label">DORA Metrics</div>
            <div class="guide-desc">Industry-standard software delivery measures. Lower lead time + higher deploy frequency = better flow. <a href="https://dora.dev/guides/dora-metrics-four-keys/" target="_blank" class="learn-more">Learn more →</a></div>
          </div>
        </div>
        <div class="guide-item">
          <div class="guide-icon">🌡️</div>
          <div class="guide-content">
            <div class="guide-label">Buffer Timeline</div>
            <div class="guide-desc">Shows schedule buffer consumption (Critical Chain). <span style="color:#22c55e">●</span> Safe <span style="color:#eab308">●</span> Caution <span style="color:#ef4444">●</span> At risk. <a href="https://en.wikipedia.org/wiki/Critical_chain_project_management#Buffer_management" target="_blank" class="learn-more">Learn more →</a></div>
          </div>
        </div>
        <div class="guide-item">
          <div class="guide-icon">💡</div>
          <div class="guide-content">
            <div class="guide-label">Lessons</div>
            <div class="guide-desc">Theory of Constraints insights about flow and uncertainty. <a href="https://www.tocico.org/page/WhatisTOCoverview" target="_blank" class="learn-more">Learn more →</a></div>
          </div>
        </div>
        <div class="guide-item">
          <div class="guide-icon">❓</div>
          <div class="guide-content">
            <div class="guide-label">Monday Questions</div>
            <div class="guide-desc">Apply lessons to your real work with these transfer questions. <a href="https://tameflow.com" target="_blank" class="learn-more">Learn more →</a></div>
          </div>
        </div>
      </div>
    </div>
  </section>
`
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
    :root {
      --green: %s;
      --yellow: %s;
      --red: %s;
      --primary: #0f172a;
      --secondary: #475569;
      --accent: #0ea5e9;
      --bg: #f8fafc;
      --card-bg: #ffffff;
      --border: #e2e8f0;
    }
    * { box-sizing: border-box; }
    body {
      font-family: 'Inter', system-ui, -apple-system, sans-serif;
      max-width: 900px;
      margin: 0 auto;
      padding: 2rem;
      background: var(--bg);
      color: var(--primary);
      line-height: 1.6;
    }
    h1 {
      font-size: 2.5rem;
      font-weight: 700;
      margin-bottom: 0.5rem;
      background: linear-gradient(135deg, var(--primary), var(--accent));
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
      background-clip: text;
    }
    .subtitle { color: var(--secondary); margin-bottom: 2rem; }
    h2 {
      font-size: 1.25rem;
      font-weight: 600;
      color: var(--primary);
      margin: 2rem 0 1rem;
      display: flex;
      align-items: center;
      gap: 0.5rem;
    }
    h2 .section-icon { margin-right: 0.5rem; }

    /* Cards */
    .card {
      background: var(--card-bg);
      border-radius: 12px;
      padding: 1.5rem;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1), 0 1px 2px rgba(0,0,0,0.06);
      margin-bottom: 1rem;
    }

    /* Metrics Grid */
    .metrics-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 1rem; }
    .metric-card { text-align: center; }
    .metric-label { font-size: 0.875rem; color: var(--secondary); text-transform: uppercase; letter-spacing: 0.05em; }
    .metric-value { font-size: 2.5rem; font-weight: 700; color: var(--primary); margin: 0.25rem 0; }
    .metric-unit { font-size: 1rem; color: var(--secondary); font-weight: 400; }
    .sparkline { margin-top: 0.5rem; }

    /* Buffer Timeline */
    .buffer-chart { text-align: center; }
    .buffer-legend { display: flex; gap: 1.5rem; justify-content: center; margin-bottom: 1rem; font-size: 0.8rem; color: var(--secondary); }
    .legend-dot { display: inline-block; width: 12px; height: 12px; border-radius: 3px; margin-right: 0.25rem; vertical-align: middle; }
    .timeline { display: flex; gap: 8px; justify-content: center; }
    .bar-container { display: flex; flex-direction: column; align-items: center; }
    .bar { width: 50px; height: 80px; border-radius: 6px; transition: transform 0.2s; }
    .bar:hover { transform: scaleY(1.05); }
    .bar-label { font-size: 0.75rem; font-weight: 600; color: var(--primary); margin-top: 0.5rem; }
    .bar-time { font-size: 0.7rem; color: var(--secondary); height: 1em; }
    .buffer-explanation { font-size: 0.85rem; color: var(--secondary); margin-top: 1.5rem; background: var(--bg); padding: 1rem; border-radius: 8px; line-height: 1.6; }
    .buffer-explanation p { margin: 0 0 0.75rem 0; }
    .buffer-explanation p:last-child { margin-bottom: 0; }
    .%s { background: linear-gradient(180deg, %s, %s); }
    .%s { background: linear-gradient(180deg, %s, %s); }
    .%s { background: linear-gradient(180deg, %s, %s); }

    /* Lessons */
    .lesson {
      background: var(--card-bg);
      border-left: 4px solid var(--accent);
      border-radius: 0 12px 12px 0;
      padding: 1.5rem;
      margin-bottom: 1rem;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1);
    }
    .lesson h3 { margin: 0 0 1rem; font-size: 1.1rem; color: var(--primary); }
    .lesson p, .lesson pre {
      margin: 0;
      font-size: 0.9rem;
      color: var(--secondary);
      line-height: 1.7;
    }
    .lesson pre.lesson-content {
      white-space: pre-wrap;
      font-family: inherit;
      background: var(--bg);
      padding: 1rem;
      border-radius: 8px;
      margin-top: 1rem;
    }
    .key-insight {
      background: linear-gradient(135deg, #fef3c7, #fde68a);
      padding: 1rem 1.25rem;
      border-radius: 8px;
      margin: 1rem 0;
      font-weight: 500;
      color: #92400e;
      font-size: 0.95rem;
      line-height: 1.5;
    }

    /* Questions */
    .question-item {
      padding: 1rem 1.25rem;
      background: var(--card-bg);
      margin-bottom: 0.75rem;
      border-radius: 8px;
      border-left: 3px solid var(--accent);
      box-shadow: 0 1px 2px rgba(0,0,0,0.05);
      font-size: 0.95rem;
    }

    /* Comparison Table */
    .comparison-table {
      width: 100%%;
      border-collapse: separate;
      border-spacing: 0;
      background: var(--card-bg);
      border-radius: 12px;
      overflow: hidden;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1);
    }
    .comparison-table th {
      background: var(--primary);
      color: white;
      padding: 1rem;
      text-align: left;
      font-weight: 600;
    }
    .comparison-table td { padding: 1rem; border-bottom: 1px solid var(--border); }
    .comparison-table tr:last-child td { border-bottom: none; }
    .winner { color: var(--green); font-weight: 600; }
    .winner-badge {
      display: inline-block;
      background: linear-gradient(135deg, var(--green), #059669);
      color: white;
      padding: 0.5rem 1rem;
      border-radius: 999px;
      font-weight: 600;
      margin-top: 1rem;
    }

    /* Parameters */
    .params-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 1rem; text-align: center; }
    .param-label { font-size: 0.75rem; color: var(--secondary); text-transform: uppercase; letter-spacing: 0.1em; }
    .param-value { font-size: 1.25rem; font-weight: 600; color: var(--primary); }

    /* Guide Grid */
    .guide-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 1.5rem; }
    .guide-item { display: flex; gap: 1rem; align-items: flex-start; }
    .guide-icon { font-size: 1.5rem; flex-shrink: 0; }
    .guide-content { flex: 1; }
    .guide-label { font-weight: 600; color: var(--primary); margin-bottom: 0.25rem; }
    .guide-desc { font-size: 0.875rem; color: var(--secondary); line-height: 1.5; }
    .learn-more { color: var(--accent); text-decoration: none; font-size: 0.8rem; }
    .learn-more:hover { text-decoration: underline; }

    /* Related sections grouping (Gestalt closure) */
    .section-group { background: linear-gradient(180deg, #f0f9ff 0%%, var(--bg) 100%%); border-radius: 16px; padding: 1rem; margin: 2rem 0; }
    .section-group h2 { margin-top: 1rem; }
    .section-connector { text-align: center; color: var(--accent); font-size: 1.5rem; margin: 0.5rem 0; }
    .section-hint { font-size: 0.9rem; color: var(--secondary); margin-bottom: 1rem; font-style: italic; }

    /* Info tooltips */
    .info-link { display: inline-block; width: 16px; height: 16px; background: var(--accent); color: white; border-radius: 50%%; font-size: 0.7rem; text-align: center; line-height: 16px; text-decoration: none; margin-left: 0.25rem; vertical-align: middle; }
    .info-link:hover { background: var(--primary); }

    .placeholder { color: var(--secondary); font-style: italic; text-align: center; padding: 2rem; }

    footer {
      margin-top: 3rem;
      padding-top: 1.5rem;
      border-top: 1px solid var(--border);
      color: var(--secondary);
      font-size: 0.875rem;
      text-align: center;
    }
  </style>
</head>
<body>
  <h1>Simulation Report</h1>
  <p class="subtitle">Software Development Flow Analysis</p>
`, time.Now().Format("2006-01-02"),
		ColorBufferGreen, ColorBufferYellow, ColorBufferRed,
		CSSClassBufferGreen, ColorBufferGreen, "#16a34a",
		CSSClassBufferYellow, ColorBufferYellow, "#ca8a04",
		CSSClassBufferRed, ColorBufferRed, "#dc2626")
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
    <div class="card">
      <div class="params-grid">
        <div><div class="param-label">Policy</div><div class="param-value">%s</div></div>
        <div><div class="param-label">Sprints</div><div class="param-value">%d</div></div>
        <div><div class="param-label">Developers</div><div class="param-value">%d</div></div>
        <div><div class="param-label">Seed</div><div class="param-value">%d</div></div>
      </div>
    </div>
  </section>
`, e.sim.Policy, e.sim.SprintCount, e.sim.DeveloperCount, e.sim.Seed)
}

func (e HTMLExporter) metricsSection() string {
	leadSparkline := generateSparklineSVG(e.tracker.LeadTimeHistory)
	return fmt.Sprintf(`
  <section id="metrics">
    <h2><span class="section-icon">📊</span>DORA Metrics</h2>
    <div class="metrics-grid">
      <div class="card metric-card">
        <div class="metric-label">Lead Time <a href="https://dora.dev/guides/dora-metrics-four-keys/#lead-time-for-changes" target="_blank" class="info-link" title="Time from commit to production">?</a></div>
        <div class="metric-value">%.1f<span class="metric-unit"> days</span></div>
        <div class="sparkline">%s</div>
      </div>
      <div class="card metric-card">
        <div class="metric-label">Deploy Frequency <a href="https://dora.dev/guides/dora-metrics-four-keys/#deployment-frequency" target="_blank" class="info-link" title="How often code is deployed">?</a></div>
        <div class="metric-value">%.1f<span class="metric-unit">/sprint</span></div>
      </div>
      <div class="card metric-card">
        <div class="metric-label">Change Fail Rate <a href="https://dora.dev/guides/dora-metrics-four-keys/#change-failure-rate" target="_blank" class="info-link" title="Percentage of deployments causing failures">?</a></div>
        <div class="metric-value">%.0f<span class="metric-unit">%%</span></div>
      </div>
      <div class="card metric-card">
        <div class="metric-label">MTTR <a href="https://dora.dev/guides/dora-metrics-four-keys/#time-to-restore-service" target="_blank" class="info-link" title="Mean Time To Restore service after failure">?</a></div>
        <div class="metric-value">%.1f<span class="metric-unit"> days</span></div>
      </div>
    </div>
  </section>
`, e.tracker.LeadTime, leadSparkline, e.tracker.DeployFrequency, e.tracker.ChangeFailRate*100, e.tracker.MTTR)
}

func (e HTMLExporter) bufferSection() string {
	timeline := bufferTimelineHTML(e.tracker.FeverHistory)
	return fmt.Sprintf(`
  <section id="buffer">
    <h2><span class="section-icon">🌡️</span>Buffer Consumption</h2>
    %s
  </section>
`, timeline)
}

func (e HTMLExporter) lessonsSection() string {
	triggered := triggeredLessons(e.lessonsSeen)
	if len(triggered) == 0 {
		return `
  <div class="section-group">
  <section id="lessons">
    <h2><span class="section-icon">💡</span>Lessons Learned</h2>
    <p class="placeholder">No lessons triggered — run simulation to see insights</p>
  </section>
`
		// Note: questionsSection() will close the section-group div
	}

	var b strings.Builder
	b.WriteString(`
  <div class="section-group">
  <section id="lessons">
    <h2><span class="section-icon">💡</span>Lessons Learned</h2>
`)
	for _, lesson := range triggered { // justified:CF
		formattedContent := formatLessonContent(lesson.Content)
		// Use first tip as tooltip TL;DR if available
		tooltip := ""
		if len(lesson.Tips) > 0 {
			tooltip = fmt.Sprintf(` title="%s"`, lesson.Tips[0])
		}
		b.WriteString(fmt.Sprintf(`    <article class="lesson"%s>
      <h3>%s</h3>
      %s
    </article>
`, tooltip, lesson.Title, formattedContent))
	}
	b.WriteString("  </section>\n")
	return b.String()
}

// Calculation: formats lesson content, extracting KEY INSIGHT into highlight box.
func formatLessonContent(content string) string {
	// Check for KEY INSIGHT pattern
	if strings.Contains(content, "KEY INSIGHT:") {
		parts := strings.SplitN(content, "KEY INSIGHT:", 2)
		before := strings.TrimSpace(parts[0])
		after := parts[1]

		// Find end of insight (next double newline or end)
		insightEnd := strings.Index(after, "\n\n")
		var insight, rest string
		if insightEnd == -1 {
			insight = strings.TrimSpace(after)
			rest = ""
		} else {
			insight = strings.TrimSpace(after[:insightEnd])
			rest = strings.TrimSpace(after[insightEnd:])
		}

		var b strings.Builder
		if before != "" {
			b.WriteString(fmt.Sprintf(`<p class="lesson-content">%s</p>`, before))
		}
		b.WriteString(fmt.Sprintf(`<div class="key-insight"><strong>Key Insight:</strong> %s</div>`, insight))
		if rest != "" {
			b.WriteString(fmt.Sprintf(`<pre class="lesson-content">%s</pre>`, rest))
		}
		return b.String()
	}

	// No KEY INSIGHT, just use pre for formatting
	return fmt.Sprintf(`<pre class="lesson-content">%s</pre>`, content)
}

func (e HTMLExporter) questionsSection() string {
	questions := transferQuestions(e.lessonsSeen)
	if len(questions) == 0 {
		return `
  <div class="section-connector">↓</div>
  <section id="questions">
    <h2><span class="section-icon">❓</span>Monday Morning Questions</h2>
    <p class="placeholder">Complete more lessons to unlock transfer questions</p>
  </section>
  </div>
`
	}

	var b strings.Builder
	b.WriteString(`
  <div class="section-connector">↓</div>
  <section id="questions">
    <h2><span class="section-icon">❓</span>Monday Morning Questions</h2>
    <p class="section-hint">Apply the lessons above to your real work:</p>
    <div class="questions-list">
`)
	// writeQuestion writes a question item HTML element.
	writeQuestion := func(q string) {
		b.WriteString(fmt.Sprintf(`      <div class="question-item">%s</div>
`, q))
	}
	slice.From(questions).Each(writeQuestion)
	b.WriteString("    </div>\n  </section>\n  </div>\n")
	return b.String()
}

func (e HTMLExporter) comparisonSection() string {
	if e.comparison == nil {
		return ""
	}
	return fmt.Sprintf(`
  <section id="comparison">
    <h2><span class="section-icon">⚖️</span>Policy Comparison</h2>
    <table class="comparison-table">
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
    <p style="text-align:center;margin-top:1.5rem;"><span class="winner-badge">Winner: %s</span></p>
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
	SparklineWidth  = 200
	SparklineHeight = 50

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
	for id, wasSeen := range seen { // justified:IX
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
	for i, pct := range history { // justified:IX
		class := classForBufferPercent(pct)
		label := fmt.Sprintf("%.0f%%", pct)
		// Add time labels: Start for first, End for last
		timeLabel := ""
		if i == 0 {
			timeLabel = "Start"
		} else if i == len(history)-1 {
			timeLabel = "End"
		}
		bars = append(bars, fmt.Sprintf(
			`<div class="bar-container"><div class="bar %s" title="%s consumed"></div><div class="bar-label">%s</div><div class="bar-time">%s</div></div>`,
			class, label, label, timeLabel))
	}

	return fmt.Sprintf(`
    <div class="buffer-chart">
      <div class="buffer-legend">
        <span><span class="legend-dot" style="background:%s"></span> Safe (&lt;33%%)</span>
        <span><span class="legend-dot" style="background:%s"></span> Caution (33-66%%)</span>
        <span><span class="legend-dot" style="background:%s"></span> At Risk (&gt;66%%)</span>
      </div>
      <div class="timeline">%s</div>
      <div class="buffer-explanation">
        <p><strong>How to read:</strong> Each bar is a checkpoint during sprint execution, showing cumulative buffer consumption at that moment. Time flows left→right from sprint start to end.</p>
        <p><strong>The concept:</strong> Instead of padding each task estimate, Critical Chain pools safety margin into one <em>project buffer</em> and tracks how fast work consumes it. When buffer turns red, act now—cut scope, add help, or reset expectations.</p>
        <p><strong>Key insight:</strong> Uncertainty causes variance. Variance consumes buffer. A single LOW understanding ticket can burn through your entire buffer. <a href="https://en.wikipedia.org/wiki/Critical_chain_project_management" target="_blank" class="learn-more">Learn more about Critical Chain →</a></p>
      </div>
    </div>`, ColorBufferGreen, ColorBufferYellow, ColorBufferRed, strings.Join(bars, ""))
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
		"Which tickets on your backlog have LOW understanding? Are you accounting for their variance in your commitments?",
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
	for id, wasSeen := range seen { // justified:IX
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
	for _, v := range data { // justified:MB
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
	for i, v := range data { // justified:IX
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

	// Determine trend color: for lead time, lower is better
	// Green = improving (going down), Red = worsening (going up), Gray = stable
	trendColor := sparklineTrendColor(data[0], data[len(data)-1])

	// Build SVG element
	return fmt.Sprintf(
		`<svg width="%d" height="%d" xmlns="%s">`+
			`<polyline points="%s" stroke="%s" stroke-width="3" fill="none"/>`+
			`</svg>`,
		SparklineWidth, SparklineHeight, SVGNamespace,
		strings.Join(points, " "),
		trendColor,
	)
}

// Calculation: determines sparkline color based on trend direction.
// For metrics where lower is better (lead time), decreasing = green, increasing = red.
func sparklineTrendColor(first, last float64) string {
	threshold := first * 0.05 // 5% change threshold for "stable"
	if last < first-threshold {
		return ColorBufferGreen // Improving (going down)
	}
	if last > first+threshold {
		return ColorBufferRed // Worsening (going up)
	}
	return ColorTrendStable // Stable
}
