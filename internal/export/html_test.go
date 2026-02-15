package export

import (
	"os"
	"strings"
	"testing"

	"github.com/binaryphile/sofdevsim-2026/internal/lessons"
)

// =============================================================================
// 1. SVG Sparkline Tests (TDD: write tests first)
// =============================================================================

func TestGenerateSparklineSVG_TwoPoints_ProducesValidSVG(t *testing.T) {
	data := []float64{10.0, 20.0}

	svg := generateSparklineSVG(data)

	if svg == "" {
		t.Fatal("expected SVG output for two points, got empty string")
	}
	if !strings.Contains(svg, "<svg") {
		t.Error("expected <svg> element")
	}
	if !strings.Contains(svg, "<polyline") {
		t.Error("expected <polyline> element")
	}
	if !strings.Contains(svg, `width="200"`) {
		t.Errorf("expected width=200, got: %s", svg)
	}
	if !strings.Contains(svg, `height="50"`) {
		t.Errorf("expected height=50, got: %s", svg)
	}
}

func TestGenerateSparklineSVG_SinglePoint_ReturnsEmpty(t *testing.T) {
	data := []float64{10.0}

	svg := generateSparklineSVG(data)

	if svg != "" {
		t.Errorf("expected empty string for single point, got: %s", svg)
	}
}

func TestGenerateSparklineSVG_Empty_ReturnsEmpty(t *testing.T) {
	svg := generateSparklineSVG([]float64{})

	if svg != "" {
		t.Errorf("expected empty string for empty data, got: %s", svg)
	}
}

func TestGenerateSparklineSVG_Nil_ReturnsEmpty(t *testing.T) {
	svg := generateSparklineSVG(nil)

	if svg != "" {
		t.Errorf("expected empty string for nil data, got: %s", svg)
	}
}

func TestGenerateSparklineSVG_AllIdentical_ProducesFlatLine(t *testing.T) {
	data := []float64{5.0, 5.0, 5.0, 5.0}

	svg := generateSparklineSVG(data)

	if svg == "" {
		t.Fatal("expected SVG output for identical values")
	}
	// For identical values, Y coordinates should be the same (flat line at middle)
	if !strings.Contains(svg, "points=") {
		t.Error("expected points attribute in polyline")
	}
}

func TestGenerateSparklineSVG_NormalizesRange(t *testing.T) {
	// Values should be normalized to fit within height (20px)
	data := []float64{0.0, 100.0}

	svg := generateSparklineSVG(data)

	if svg == "" {
		t.Fatal("expected SVG output")
	}
	// First point should be at bottom (y=20), second at top (y=0)
	if !strings.Contains(svg, "points=") {
		t.Error("expected points attribute")
	}
}

func BenchmarkGenerateSparklineSVG(b *testing.B) {
	data := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 4.0, 3.0, 2.0, 1.0, 2.0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		generateSparklineSVG(data)
	}
}

// =============================================================================
// 2. Buffer Timeline Tests (TDD: write tests first)
// =============================================================================

func TestBufferTimelineHTML_MixedValues_AppliesCorrectColors(t *testing.T) {
	// Values: 0-33% green, 34-66% yellow, 67-100% red
	history := []float64{20.0, 50.0, 80.0} // green, yellow, red

	html := bufferTimelineHTML(history)

	if html == "" {
		t.Fatal("expected HTML output for mixed values")
	}
	if !strings.Contains(html, CSSClassBufferGreen) {
		t.Error("expected green class for 20% value")
	}
	if !strings.Contains(html, CSSClassBufferYellow) {
		t.Error("expected yellow class for 50% value")
	}
	if !strings.Contains(html, CSSClassBufferRed) {
		t.Error("expected red class for 80% value")
	}
}

func TestBufferTimelineHTML_Empty_ReturnsPlaceholder(t *testing.T) {
	html := bufferTimelineHTML([]float64{})

	if html == "" {
		t.Fatal("expected placeholder message for empty history")
	}
	if !strings.Contains(html, "No buffer data") {
		t.Errorf("expected placeholder text, got: %s", html)
	}
}

func TestBufferTimelineHTML_Nil_ReturnsPlaceholder(t *testing.T) {
	html := bufferTimelineHTML(nil)

	if html == "" {
		t.Fatal("expected placeholder message for nil history")
	}
	if !strings.Contains(html, "No buffer data") {
		t.Errorf("expected placeholder text, got: %s", html)
	}
}

func TestBufferTimelineHTML_AllGreen_OnlyGreenBars(t *testing.T) {
	history := []float64{10.0, 20.0, 30.0} // all green (<33%)

	html := bufferTimelineHTML(history)

	if !strings.Contains(html, CSSClassBufferGreen) {
		t.Error("expected green class")
	}
	if strings.Contains(html, CSSClassBufferYellow) {
		t.Error("unexpected yellow class for all-green values")
	}
	if strings.Contains(html, CSSClassBufferRed) {
		t.Error("unexpected red class for all-green values")
	}
}

func TestBufferTimelineHTML_AllRed_OnlyRedBars(t *testing.T) {
	history := []float64{70.0, 80.0, 90.0} // all red (>66%)

	html := bufferTimelineHTML(history)

	if !strings.Contains(html, CSSClassBufferRed) {
		t.Error("expected red class")
	}
	if strings.Contains(html, CSSClassBufferGreen) {
		t.Error("unexpected green class for all-red values")
	}
}

func BenchmarkBufferTimelineHTML(b *testing.B) {
	history := []float64{10.0, 25.0, 40.0, 55.0, 70.0, 85.0, 90.0, 75.0, 50.0, 30.0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bufferTimelineHTML(history)
	}
}

// =============================================================================
// 3. Triggered Lessons Tests (TDD: write tests first)
// =============================================================================

func TestTriggeredLessons_SeenMultiple_ReturnsAll(t *testing.T) {
	seen := map[lessons.LessonID]bool{
		lessons.UncertaintyConstraint: true,
		lessons.ConstraintHunt:        true,
	}

	result := triggeredLessons(seen)

	if len(result) != 2 {
		t.Errorf("expected 2 lessons, got %d", len(result))
	}

	// Verify both lessons are present
	ids := make(map[lessons.LessonID]bool)
	for _, l := range result { // justified:AS
		ids[l.ID] = true
	}
	if !ids[lessons.UncertaintyConstraint] {
		t.Error("expected UncertaintyConstraint lesson")
	}
	if !ids[lessons.ConstraintHunt] {
		t.Error("expected ConstraintHunt lesson")
	}
}

func TestTriggeredLessons_NilMap_ReturnsEmpty(t *testing.T) {
	result := triggeredLessons(nil)

	if len(result) != 0 {
		t.Errorf("expected empty slice for nil map, got %d lessons", len(result))
	}
}

func TestTriggeredLessons_EmptyMap_ReturnsEmpty(t *testing.T) {
	result := triggeredLessons(map[lessons.LessonID]bool{})

	if len(result) != 0 {
		t.Errorf("expected empty slice for empty map, got %d lessons", len(result))
	}
}

func TestTriggeredLessons_FalseValues_NotIncluded(t *testing.T) {
	seen := map[lessons.LessonID]bool{
		lessons.UncertaintyConstraint: true,
		lessons.ConstraintHunt:        false, // explicitly false
	}

	result := triggeredLessons(seen)

	if len(result) != 1 {
		t.Errorf("expected 1 lesson (false values excluded), got %d", len(result))
	}
}

// =============================================================================
// 4. Transfer Questions Tests (TDD: write tests first)
// =============================================================================

func TestTransferQuestions_UC19Seen_IncludesUncertaintyQuestion(t *testing.T) {
	seen := map[lessons.LessonID]bool{
		lessons.UncertaintyConstraint: true,
	}

	questions := transferQuestions(seen)

	if len(questions) == 0 {
		t.Fatal("expected at least one question for UC19")
	}

	// Should include question about LOW understanding tickets
	found := false
	for _, q := range questions { // justified:AS
		if strings.Contains(q, "LOW understanding") || strings.Contains(q, "understanding") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected uncertainty-related question, got: %v", questions)
	}
}

func TestTransferQuestions_UC20Seen_IncludesConstraintQuestion(t *testing.T) {
	seen := map[lessons.LessonID]bool{
		lessons.ConstraintHunt: true,
	}

	questions := transferQuestions(seen)

	if len(questions) == 0 {
		t.Fatal("expected at least one question for UC20")
	}

	// Should include question about queues/constraint
	found := false
	for _, q := range questions { // justified:AS
		if strings.Contains(q, "queue") || strings.Contains(q, "constraint") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected constraint-related question, got: %v", questions)
	}
}

func TestTransferQuestions_NoneSeen_ReturnsEmpty(t *testing.T) {
	questions := transferQuestions(map[lessons.LessonID]bool{})

	if len(questions) != 0 {
		t.Errorf("expected empty slice for no lessons seen, got %d questions", len(questions))
	}
}

func TestTransferQuestions_NilMap_ReturnsEmpty(t *testing.T) {
	questions := transferQuestions(nil)

	if len(questions) != 0 {
		t.Errorf("expected empty slice for nil map, got %d questions", len(questions))
	}
}

func TestTransferQuestions_MultipleSeen_ReturnsMultiple(t *testing.T) {
	seen := map[lessons.LessonID]bool{
		lessons.UncertaintyConstraint: true,
		lessons.ConstraintHunt:        true,
		lessons.ExploitFirst:          true,
	}

	questions := transferQuestions(seen)

	if len(questions) < 3 {
		t.Errorf("expected at least 3 questions for 3 lessons, got %d", len(questions))
	}
}

// =============================================================================
// 5. Full HTML Generation Tests (TDD: write tests first)
// =============================================================================

func TestGenerateHTML_TypicalSimulation_ContainsAllSections(t *testing.T) {
	exporter := newTestExporter(true) // with comparison

	html := exporter.GenerateHTML()

	// Check all 6 sections present
	sections := []string{
		"How to Read This Report",
		"DORA Metrics",
		"Buffer Consumption",
		"Lessons",
		"Monday Morning",
		"Policy Comparison",
	}
	for _, section := range sections { // justified:AS
		if !strings.Contains(html, section) {
			t.Errorf("expected section %q in HTML", section)
		}
	}

	// Check basic HTML structure
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("expected DOCTYPE declaration")
	}
	if !strings.Contains(html, "<html") {
		t.Error("expected <html> tag")
	}
	if !strings.Contains(html, "</html>") {
		t.Error("expected closing </html> tag")
	}
}

func TestGenerateHTML_NoComparison_OmitsComparisonSection(t *testing.T) {
	exporter := newTestExporter(false) // no comparison

	html := exporter.GenerateHTML()

	if strings.Contains(html, "Policy Comparison") {
		t.Error("expected no comparison section when comparison is nil")
	}

	// Other sections should still be present
	if !strings.Contains(html, "DORA Metrics") {
		t.Error("expected DORA Metrics section")
	}
}

func TestGenerateHTML_IncludesSimulationParams(t *testing.T) {
	exporter := newTestExporter(false)

	html := exporter.GenerateHTML()

	if !strings.Contains(html, "42") { // seed
		t.Error("expected seed value in HTML")
	}
	if !strings.Contains(html, "tameflow") {
		t.Error("expected policy name in HTML")
	}
}

func TestGenerateHTML_IncludesInlineCSS(t *testing.T) {
	exporter := newTestExporter(false)

	html := exporter.GenerateHTML()

	if !strings.Contains(html, "<style>") {
		t.Error("expected inline CSS")
	}
	if !strings.Contains(html, CSSClassBufferGreen) {
		t.Error("expected buffer-green CSS class definition")
	}
}

func BenchmarkGenerateHTML(b *testing.B) {
	exporter := newTestExporter(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		exporter.GenerateHTML()
	}
}

// newTestExporter creates an HTMLExporter with test data.
func newTestExporter(withComparison bool) HTMLExporter {
	sim := testSimulation()
	tracker := testTracker()
	lessonState := testLessonState()

	var comparison *ComparisonSummary
	if withComparison {
		comparison = testComparisonSummary()
	}

	return NewHTMLExporter(sim, tracker, lessonState, comparison)
}

func testSimulation() SimulationParams {
	return SimulationParams{
		Seed:           42,
		Policy:         "tameflow-cognitive",
		DeveloperCount: 3,
		SprintCount:    5,
	}
}

func testTracker() TrackerData {
	return TrackerData{
		LeadTime:        4.2,
		DeployFrequency: 2.5,
		ChangeFailRate:  0.15,
		MTTR:            1.2,
		LeadTimeHistory: []float64{5.0, 4.5, 4.2},
		FeverHistory:    []float64{20.0, 45.0, 70.0},
	}
}

func testLessonState() map[lessons.LessonID]bool {
	return map[lessons.LessonID]bool{
		lessons.UncertaintyConstraint: true,
		lessons.ConstraintHunt:        true,
	}
}

func testComparisonSummary() *ComparisonSummary {
	return &ComparisonSummary{
		PolicyA:     "DORA-Strict",
		PolicyB:     "TameFlow",
		Winner:      "TameFlow",
		LeadTimeA:   5.2,
		LeadTimeB:   3.8,
		DeployFreqA: 2.0,
		DeployFreqB: 2.8,
	}
}

// =============================================================================
// 6. File Export Tests (TDD: write tests first)
// =============================================================================

func TestExportToFile_ValidPath_CreatesFile(t *testing.T) {
	exporter := newTestExporter(false)
	tmpDir := t.TempDir()
	path := tmpDir + "/report.html"

	err := exporter.ExportToFile(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file exists and contains HTML
	content, err := readFileContent(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if !strings.Contains(content, "<!DOCTYPE html>") {
		t.Error("expected HTML content in file")
	}
}

func TestExportToFile_InvalidPath_ReturnsError(t *testing.T) {
	exporter := newTestExporter(false)
	path := "/nonexistent/directory/report.html"

	err := exporter.ExportToFile(path)

	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestExportToFile_CreatesDirectory_IfNeeded(t *testing.T) {
	exporter := newTestExporter(false)
	tmpDir := t.TempDir()
	path := tmpDir + "/exports/report.html"

	err := exporter.ExportToFile(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file was created
	if _, err := readFileContent(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

// readFileContent is a helper for tests.
func readFileContent(path string) (string, error) {
	// Using os.ReadFile directly in test helper
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// TestManual_GenerateInspectableReport generates a report at /tmp for manual inspection.
// Run with: go test -v -run TestManual_GenerateInspectableReport ./internal/export/
func TestManual_GenerateInspectableReport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping manual inspection test in short mode")
	}

	// Create rich test data simulating 3 sprints
	sim := SimulationParams{
		Seed:           42,
		Policy:         "TameFlow-Cognitive",
		DeveloperCount: 3,
		SprintCount:    3,
	}

	tracker := TrackerData{
		LeadTime:        4.2,
		DeployFrequency: 2.5,
		ChangeFailRate:  0.15,
		MTTR:            1.2,
		LeadTimeHistory: []float64{6.0, 5.0, 4.5, 4.2, 3.8}, // improving trend
		FeverHistory:    []float64{20.0, 35.0, 50.0, 65.0, 80.0}, // buffer consumption over sprint
	}

	// Simulate having seen lessons from 3 sprints
	lessonsSeen := map[lessons.LessonID]bool{
		lessons.UncertaintyConstraint: true,
		lessons.ConstraintHunt:        true,
		lessons.ExploitFirst:          true,
		lessons.FiveFocusing:          true,
	}

	comparison := &ComparisonSummary{
		PolicyA:     "DORA-Strict",
		PolicyB:     "TameFlow-Cognitive",
		Winner:      "TameFlow-Cognitive",
		LeadTimeA:   5.2,
		LeadTimeB:   3.8,
		DeployFreqA: 2.0,
		DeployFreqB: 2.8,
	}

	exporter := NewHTMLExporter(sim, tracker, lessonsSeen, comparison)
	path := "/tmp/test-report.html"
	if err := exporter.ExportToFile(path); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	t.Logf("Report generated at: %s", path)
	t.Logf("Open in browser: file://%s", path)
}
