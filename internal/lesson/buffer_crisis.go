package lesson

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/engine"
	"github.com/binaryphile/sofdevsim-2026/internal/events"
	"github.com/binaryphile/sofdevsim-2026/internal/model"
	"github.com/binaryphile/sofdevsim-2026/internal/zkproof"
)

// lessonSeed produces a deterministic crisis-recovery sequence.
const lessonSeed int64 = 42

// zoneChange tracks a buffer zone transition for proof generation.
type zoneChange struct {
	tick    int
	oldZone model.FeverStatus
	newZone model.FeverStatus
}

// userChoice tracks decisions made during the lesson.
type userChoice struct {
	tick   int
	choice string
}

// BufferCrisisLesson teaches crisis-recovery through guided simulation.
type BufferCrisisLesson struct{}

// Name returns the lesson identifier.
func (l BufferCrisisLesson) Name() string {
	return "buffer-crisis"
}

// Description returns a brief description of the lesson.
func (l BufferCrisisLesson) Description() string {
	return "Learn how buffer crises develop and resolve in software projects"
}

// Run executes the interactive lesson.
func (l BufferCrisisLesson) Run(proverPath string) error {
	display := NewDisplay()
	display.Header("Lesson: Buffer Crisis Management")

	// Explain what the user is about to see
	display.Text("THE BUFFER")
	display.Text("──────────")
	display.Text("In project management, a 'buffer' is extra time built into the schedule")
	display.Text("to absorb uncertainty. When work takes longer than expected, buffer is consumed.")
	display.Text("")
	display.Text("THE FEVER CHART (TameFlow)")
	display.Text("──────────────────────────")
	display.Text("The fever chart compares buffer consumption to work progress:")
	display.Text("")
	display.Text("  Ratio = Used% ÷ Work%")
	display.Text("")
	display.Text("  🟩🟩🟩🟩🟩🟩🟨🟨🟨🟨🟨🟨🟨🟥🟥🟥🟥🟥🟥🟥🟥🟥🟥🟥🟥")
	display.Text("  0     0.66     1.0            1.5          2.0+")
	display.Text("    AHEAD      ON TRACK            BEHIND")
	display.Text("")
	display.Text("Examples:")
	display.Text("  Work 40%, Used 20%  →  0.5 🟢  (used half as much buffer as progress)")
	display.Text("  Work 40%, Used 40%  →  1.0 🟡  (used same buffer as progress)")
	display.Text("  Work 20%, Used 40%  →  2.0 🔴  (used 2x as much buffer as progress)")
	display.Text("")
	display.Text("Using all your buffer isn't failure - it's what buffer is FOR.")
	display.Text("Failure is using all buffer before finishing the work.")
	display.Text("")
	display.Text("Your goal: Navigate the team through a crisis using TameFlow principles.")
	display.Text("")

	if !display.PromptYN("Ready to begin?") {
		return nil
	}

	// Create engine with event store for tracking
	store := events.NewMemoryStore()
	eng := engine.NewEngineWithStore(lessonSeed, store)

	// Initialize simulation
	simID := "lesson-sim"
	// 14-day sprint with 10% buffer = only 1.4 buffer days
	// This makes buffer consumption hit Red faster
	config := events.SimConfig{
		TeamSize:     1,
		SprintLength: 14,
		Seed:         lessonSeed,
		Policy:       model.PolicyDORAStrict,
		BufferPct:    0.10, // Tight buffer forces crisis
	}
	var err error
	eng, err = eng.EmitCreated(simID, 0, config)
	if err != nil {
		return fmt.Errorf("create simulation: %w", err)
	}

	// Add multiple developers to maximize active tickets
	for _, dev := range []struct{ id, name string }{
		{"dev-1", "Alex"},
		{"dev-2", "Blake"},
		{"dev-3", "Casey"},
	} {
		eng, err = eng.AddDeveloper(dev.id, dev.name, 1.0)
		if err != nil {
			return fmt.Errorf("add developer: %w", err)
		}
	}

	// 3 large tickets that can't complete in 14-day sprint
	// With 10% buffer (1.4 days), falling behind quickly hits Red
	// Recovery requires completing work, not just waiting for new sprint
	tickets := []model.Ticket{
		model.NewTicket("TKT-001", "Major refactoring", 25, model.LowUnderstanding),
		model.NewTicket("TKT-002", "New feature", 25, model.LowUnderstanding),
		model.NewTicket("TKT-003", "Technical debt", 25, model.LowUnderstanding),
	}
	for _, t := range tickets {
		eng, err = eng.AddTicket(t)
		if err != nil {
			return fmt.Errorf("add ticket: %w", err)
		}
	}

	// Track zone transitions by monitoring sprint state directly
	// (The engine doesn't emit BufferZoneChanged events automatically)
	var crisisTick, recoveryTick int
	lastZone := model.FeverYellow // Start balanced (progress ≈ buffer when both are 0)
	var collectedZoneChanges []zoneChange

	// Track user decisions and accumulated debt
	var userChoices []userChoice
	accumulatedDebt := 0.0 // Bad choices accumulate debt that carries to next sprint

	// Run simulation with interactive decision points
	for sprintNum := 0; sprintNum < 4 && recoveryTick == 0; sprintNum++ {
		// Start sprint
		eng, err = eng.StartSprint()
		if err != nil {
			return fmt.Errorf("start sprint %d: %w", sprintNum+1, err)
		}

		// Apply accumulated debt from previous sprint's bad choices
		// This prevents "free reset" at sprint boundary
		if accumulatedDebt > 0 && sprintNum > 0 {
			sprint, _ := eng.Sim().CurrentSprintOption.Get()
			debtDays := sprint.BufferDays * accumulatedDebt
			if debtDays > 0 {
				display.Text(fmt.Sprintf("\n⚠️  Debt carried forward: %.0f%% of buffer already consumed", accumulatedDebt*100))
				display.Text("   (Consequence of previous choices)\n")
				// Emit buffer consumption for the debt
				eng, _ = eng.EmitBufferConsumed(debtDays)
			}
		}

		// Assign all backlog tickets to idle developers
		eng = l.assignAvailableWork(eng)

		// Run through the sprint, showing live simulation state
		sprint, _ := eng.Sim().CurrentSprintOption.Get()
		fmt.Printf("\n── Sprint %d (days %d-%d) ──\n", sprintNum+1, sprint.StartDay+1, sprint.EndDay)

		for eng.Sim().CurrentTick < sprint.EndDay && recoveryTick == 0 {
			eng, _, err = eng.Tick()
			if err != nil {
				return fmt.Errorf("tick: %w", err)
			}

			// Show live simulation state every tick
			if sp, ok := eng.Sim().CurrentSprintOption.Get(); ok {
				sim := eng.Sim()

				// Calculate progress: completed points / total points
				var totalPoints, completedPoints float64
				for _, t := range sim.Backlog {
					totalPoints += t.EstimatedDays
				}
				for _, t := range sim.ActiveTickets {
					totalPoints += t.EstimatedDays
				}
				for _, t := range sim.CompletedTickets {
					totalPoints += t.EstimatedDays
					completedPoints += t.EstimatedDays
				}
				var progress float64
				if totalPoints > 0 {
					progress = completedPoints / totalPoints
				}

				bufferPct := sp.BufferPctUsed()

				// TameFlow fever chart uses diagonal zone boundaries:
				// - Green-Yellow boundary: line from (0,0) to (100%, 66%)
				//   At any progress%, buffer < progress * 0.66 is GREEN
				// - Yellow-Red boundary: line from (0, 33%) to (100%, 100%)
				//   At any progress%, buffer > 0.33 + progress * 0.67 is RED
				var currentZone model.FeverStatus
				var zoneLabel string

				greenThreshold := progress * 0.66           // below this = GREEN
				redThreshold := 0.33 + progress*0.67        // above this = RED

				// Expected progress based on time elapsed in sprint
				expected := sp.ProgressPct(sim.CurrentTick)

				// Calculate ratio = buffer% / progress%
				// Ratio < 1 means buffer use is less than progress (good)
				// Ratio > 1 means buffer use exceeds progress (bad)
				// Only show ratio once we have meaningful progress (>5%)
				var ratioStr string
				if progress > 0.05 {
					ratio := bufferPct / progress
					ratioStr = fmt.Sprintf("%.1f", ratio)
				} else {
					ratioStr = "-" // not enough progress for meaningful ratio
				}

				switch {
				case bufferPct <= greenThreshold:
					currentZone = model.FeverGreen
					zoneLabel = "🟢"
				case bufferPct >= redThreshold:
					currentZone = model.FeverRed
					zoneLabel = "🔴"
				default:
					currentZone = model.FeverYellow
					zoneLabel = "🟡"
				}

				// Live status: Day, work (expected), buffer used, ratio, zone
				fmt.Printf("\r  Day %2d: Work %3.0f%% (exp %2.0f%%) | Used %3.0f%% | Ratio %s %s     ",
					sim.CurrentTick, progress*100, expected*100, bufferPct*100, ratioStr, zoneLabel)
				if currentZone != lastZone {
					tick := eng.Sim().CurrentTick
					fmt.Println()

					// Display the zone change
					l.displayZoneChangeFromState(display, tick, lastZone, currentZone)

					// Interactive decision points
					if currentZone == model.FeverYellow && lastZone == model.FeverGreen {
						choice := l.promptYellowDecision(display, tick)
						userChoices = append(userChoices, userChoice{tick: tick, choice: choice})

						switch choice {
						case "overtime":
							display.Text("\n→ Overtime initiated. Watch what happens to the buffer...\n")
							// Overtime causes bugs - adds rework tickets AND debt
							eng, _ = eng.AddTicket(model.NewTicket(
								fmt.Sprintf("TKT-BUG-%d", tick), "Bug from fatigue", 4, model.LowUnderstanding))
							eng, _ = eng.AddTicket(model.NewTicket(
								fmt.Sprintf("TKT-BUG2-%d", tick), "Another bug", 3, model.LowUnderstanding))
							eng = l.assignAvailableWork(eng)
							accumulatedDebt += 0.3 // 30% buffer penalty carries forward
						case "wait":
							display.Text("\n→ Waiting to see if things improve...\n")
							// Waiting while yellow = mild debt (should have acted)
							accumulatedDebt += 0.1
							eng = l.assignAvailableWork(eng) // Keep assigning backlog
						default:
							display.Text("\n→ No new work. Focusing on current items...\n")
							// Correct choice - no debt, no new assignments
						}
					}

					if currentZone == model.FeverRed {
						choice := l.promptRedDecision(display, tick)
						userChoices = append(userChoices, userChoice{tick: tick, choice: choice})

						switch choice {
						case "meeting":
							display.Text("\n→ Meeting called. Team stops coding for 2 hours...\n")
							// Meeting adds action items AND significant debt
							eng, _ = eng.AddTicket(model.NewTicket(
								fmt.Sprintf("TKT-ACTIONS-%d", tick), "Meeting action items", 4, model.MediumUnderstanding))
							eng = l.assignAvailableWork(eng)
							accumulatedDebt += 0.25
						case "add-people":
							display.Text("\n→ Contractors joining. They need onboarding...\n")
							// Brooks's Law - severe debt penalty
							eng, _ = eng.AddTicket(model.NewTicket(
								fmt.Sprintf("TKT-ONBOARD-%d", tick), "Contractor onboarding", 6, model.LowUnderstanding))
							eng, _ = eng.AddTicket(model.NewTicket(
								fmt.Sprintf("TKT-DOCS-%d", tick), "Write setup docs", 3, model.LowUnderstanding))
							eng = l.assignAvailableWork(eng)
							accumulatedDebt += 0.5 // 50% buffer penalty - Brooks's Law hits hard
						default:
							display.Text("\n→ Meetings cancelled. Pure focus on finishing...\n")
							// Correct choice - no debt, no new work
						}
					}

					// Track for ZK proof
					collectedZoneChanges = append(collectedZoneChanges, zoneChange{
						tick:    tick,
						oldZone: lastZone,
						newZone: currentZone,
					})

					// Track crisis and recovery
					if currentZone == model.FeverRed && crisisTick == 0 {
						crisisTick = tick
					}
					if currentZone == model.FeverGreen && crisisTick > 0 && recoveryTick == 0 {
						recoveryTick = tick
					}

					lastZone = currentZone
				}
			}

			// Delay so user can see each day's progress
			time.Sleep(1 * time.Second)
		}
		fmt.Println() // Final newline after sprint
	}

	// Handle case where no crisis occurred (shouldn't happen with seed 42)
	if crisisTick == 0 {
		display.Text("\nNo crisis occurred in this simulation run.")
		display.Text("Try running again or adjusting the workload.")
		return nil
	}

	// Check if lesson failed (never recovered)
	// This happens when bad choices added too much work for recovery
	if recoveryTick == 0 {
		display.Header("Lesson Failed")
		display.Text("The project never recovered from the crisis.")
		display.Text("")
		display.Text("Your choices affected the simulation:")
		for _, uc := range userChoices {
			display.Text(fmt.Sprintf("  Tick %d: %s", uc.tick, uc.choice))
		}
		display.Text("")
		display.Text("The simulation shows what happens when you add work during a crisis.")
		display.Text("Run the lesson again to try different approaches.")
		return nil
	}

	// Lesson complete
	display.Header("Lesson Complete")
	display.Text("You experienced a crisis-recovery cycle.")
	display.Text("This pattern is common in software projects.")
	display.Text("")
	display.Text(fmt.Sprintf("Crisis started at tick %d, resolved at tick %d.", crisisTick, recoveryTick))
	display.Text("")
	display.Text("This proof cryptographically verifies you completed the lesson")
	display.Text("without revealing the simulation details.")

	// Offer proof generation
	if !display.PromptYN("Generate a ZK proof of this learning?") {
		return nil
	}

	return l.generateProof(display, simID, proverPath, collectedZoneChanges, crisisTick, recoveryTick)
}

// assignAvailableWork assigns backlog tickets to idle developers.
func (l BufferCrisisLesson) assignAvailableWork(eng engine.Engine) engine.Engine {
	for {
		sim := eng.Sim()
		if len(sim.Backlog) == 0 {
			break
		}
		ticketID := sim.Backlog[0].ID

		assigned := false
		for _, dev := range sim.Developers {
			if dev.IsIdle() {
				eng, _ = eng.AssignTicket(ticketID, dev.ID)
				assigned = true
				break
			}
		}
		if !assigned {
			break
		}
	}
	return eng
}

// promptYellowDecision asks the user what to do when entering yellow zone.
func (l BufferCrisisLesson) promptYellowDecision(d Display, tick int) string {
	choice := d.PromptChoice(
		"⚠️  The buffer is shrinking. What do you do?",
		[]string{
			"Have the team work overtime to catch up",
			"Wait and see if things improve",
			"Stop starting new work until current items finish",
		})

	switch choice {
	case 0:
		return "overtime"
	case 1:
		return "wait"
	default:
		return "stop-starting"
	}
}

// promptRedDecision asks the user what to do when entering red zone.
func (l BufferCrisisLesson) promptRedDecision(d Display, tick int) string {
	choice := d.PromptChoice(
		"🔴 CRISIS! The buffer is nearly exhausted. What do you do?",
		[]string{
			"Call an all-hands meeting to discuss the situation",
			"Bring in contractors to help with the workload",
			"Cancel all meetings, everyone focuses on their current task until done",
		})

	switch choice {
	case 0:
		return "meeting"
	case 1:
		return "add-people"
	default:
		return "focus"
	}
}

// displayZoneChangeFromState shows educational narration for zone transitions.
// Content based on TameFlow's Fever Chart methodology.
func (l BufferCrisisLesson) displayZoneChangeFromState(d Display, tick int, oldZone, newZone model.FeverStatus) {
	switch {
	case oldZone == model.FeverGreen && newZone == model.FeverYellow:
		d.Event(tick, "Buffer: GREEN → YELLOW",
			"⚠️  Work is accumulating faster than completing.",
			"",
			"The buffer is your safety margin against uncertainty.",
			"When it shrinks, you have less room for surprises.",
			"",
			"TameFlow insight: Yellow means 'pay attention'.",
			"Review work-in-progress. Are items stuck? Blocked?",
			"This is the time to intervene—not when it's red.")

	case oldZone == model.FeverYellow && newZone == model.FeverRed:
		d.Event(tick, "Buffer: YELLOW → RED",
			"🔴 CRISIS! Buffer nearly exhausted.",
			"",
			"In real projects, this means:",
			"- Too much work in progress (WIP)",
			"- Team is overloaded and context-switching",
			"- Quality suffers, deadlines slip",
			"- Technical debt accumulates silently",
			"",
			"The remedy: STOP STARTING, START FINISHING.",
			"",
			"TameFlow insight: Red is not failure—it's a signal.",
			"Swarm on blocked items. Reduce WIP ruthlessly.",
			"Every new task you start makes recovery harder.")

	case oldZone == model.FeverGreen && newZone == model.FeverRed:
		d.Event(tick, "Buffer: GREEN → RED",
			"🔴 CRISIS! Buffer jumped directly to red.",
			"",
			"Something unexpected consumed the buffer rapidly.",
			"Common causes:",
			"- Production incident requiring all hands",
			"- Major scope change mid-sprint",
			"- Key team member unavailable",
			"- Discovered complexity (unknown unknowns)",
			"",
			"This is why buffers exist—to absorb shocks.")

	case newZone == model.FeverGreen && oldZone != model.FeverGreen:
		d.Event(tick, "Buffer: "+oldZone.String()+" → GREEN",
			"✅ RECOVERED! Buffer restored to healthy levels.",
			"",
			"The team focused on completing work rather than",
			"starting new tasks. Flow was restored.",
			"",
			"TameFlow insight: Recovery happens when you",
			"finish more than you start. This is sustainable pace.",
			"",
			"Key learning: The crisis wasn't caused by one thing.",
			"It was caused by accumulated small decisions.",
			"Recovery works the same way—many small completions.")

	case oldZone == model.FeverRed && newZone == model.FeverYellow:
		d.Event(tick, "Buffer: RED → YELLOW",
			"📈 Improving! Buffer consumption decreasing.",
			"",
			"The crisis is easing. Your interventions are working.",
			"",
			"Keep the discipline:",
			"- No new work until current items complete",
			"- Continue swarming on blockers",
			"- Protect the team from interruptions",
			"",
			"You're not out of the woods yet. Stay focused.")
	}
}

// generateProof creates the ZK proof and saves it to a file.
func (l BufferCrisisLesson) generateProof(d Display, simID, proverPath string, zoneChanges []zoneChange, crisisTick, recoveryTick int) error {
	// Build events from zone changes for the prover
	var proofEvents []events.Event
	for _, zc := range zoneChanges {
		proofEvents = append(proofEvents, events.NewBufferZoneChanged(
			simID,
			zc.tick,
			zc.oldZone,
			zc.newZone,
			0.5, // Penetration ratio not critical for proof
		))
	}

	// Detect crisis sequences
	sequences := zkproof.DetectBufferCrisis(proofEvents)
	if len(sequences) == 0 {
		d.Error("No crisis-recovery sequence detected in events")
		return fmt.Errorf("no crisis-recovery sequence detected")
	}

	// Use the first sequence
	seq := sequences[0]

	// Resolve prover path
	if proverPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get home dir: %w", err)
		}
		proverPath = filepath.Join(home, "projects", "zk-event-proofs")
	}

	// Invoke prover with spinner
	var result zkproof.ProofResult
	err := d.Spinner("Generating proof...", func() error {
		var err error
		result, err = zkproof.InvokeProver(proverPath, simID, seq)
		return err
	})
	if err != nil {
		d.Error(fmt.Sprintf("Proof generation failed: %v", err))
		return err
	}

	if !result.Success {
		d.Error(fmt.Sprintf("Proof failed: %s", result.Error))
		return fmt.Errorf("proof failed: %s", result.Error)
	}

	// Save proof to file
	if err := os.MkdirAll("./proofs", 0755); err != nil {
		return fmt.Errorf("create proofs directory: %w", err)
	}

	filename := fmt.Sprintf("./proofs/lesson-%s.json", time.Now().Format("2006-01-02"))
	if err := os.WriteFile(filename, []byte(result.Proof), 0644); err != nil {
		return fmt.Errorf("save proof: %w", err)
	}

	d.Success("Proof generated!",
		fmt.Sprintf("Crisis:   tick %d", result.PublicOutput.CrisisTimestamp),
		fmt.Sprintf("Recovery: tick %d", result.PublicOutput.RecoveryTimestamp),
		fmt.Sprintf("File:     %s", filename))

	return nil
}
