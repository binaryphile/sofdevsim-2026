package lesson

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// Display handles terminal output for lessons.
type Display struct {
	reader *bufio.Reader
}

// NewDisplay creates a new Display for terminal output.
func NewDisplay() Display {
	return Display{reader: bufio.NewReader(os.Stdin)}
}

// Header displays a boxed header.
func (d Display) Header(text string) {
	width := len(text) + 4
	border := strings.Repeat("─", width)
	fmt.Printf("\n╭%s╮\n", border)
	fmt.Printf("│  %s  │\n", text)
	fmt.Printf("╰%s╯\n\n", border)
}

// Event displays a tick event with narration lines.
func (d Display) Event(tick int, title string, lines ...string) {
	fmt.Printf("[Tick %d] %s\n", tick, title)
	for _, line := range lines {
		if line == "" {
			fmt.Println()
		} else {
			fmt.Printf("         %s\n", line)
		}
	}
	fmt.Println()
}

// Text displays a plain text line.
func (d Display) Text(text string) {
	fmt.Println(text)
}

// Success displays a success message with details.
func (d Display) Success(title string, lines ...string) {
	fmt.Printf("\n✓ %s\n", title)
	for _, line := range lines {
		fmt.Printf("  %s\n", line)
	}
}

// Error displays an error message.
func (d Display) Error(message string) {
	fmt.Printf("\n✗ %s\n", message)
}

// PromptYN prompts for a yes/no response. Returns true for yes (default).
func (d Display) PromptYN(question string) bool {
	fmt.Printf("\n%s [Y/n] ", question)
	response, err := d.reader.ReadString('\n')
	if err != nil {
		return true // Default to yes on error
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "" || response == "y" || response == "yes"
}

// PromptChoice presents numbered options and returns the selected index (0-based).
// Loops until a valid choice is made.
func (d Display) PromptChoice(question string, options []string) int {
	fmt.Printf("\n%s\n", question)
	for i, opt := range options {
		fmt.Printf("  %d. %s\n", i+1, opt)
	}

	// Fresh reader for each prompt to avoid buffering issues with live output
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("\nYour choice: ")

		response, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("  Please enter a number.")
			continue
		}
		response = strings.TrimSpace(response)

		if response == "" {
			fmt.Println("  Please enter a number.")
			continue
		}

		choice := 0
		_, err = fmt.Sscanf(response, "%d", &choice)
		if err != nil || choice < 1 || choice > len(options) {
			fmt.Printf("  Please enter 1-%d.\n", len(options))
			continue
		}
		return choice - 1
	}
}

// Spinner runs a spinner with message while work happens.
func (d Display) Spinner(message string, work func() error) error {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	done := make(chan error)

	go func() {
		done <- work()
	}()

	i := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			fmt.Printf("\r%s done\n", message)
			return err
		case <-ticker.C:
			fmt.Printf("\r%s %s", frames[i%len(frames)], message)
			i++
		}
	}
}
