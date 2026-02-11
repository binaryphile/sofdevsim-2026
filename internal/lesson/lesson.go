// Package lesson provides interactive educational lessons for sofdevsim.
package lesson

// Lesson defines the interface for interactive lessons.
type Lesson interface {
	Name() string
	Description() string
	Run(proverPath string) error
}

// Lessons is the registry of available lessons.
var Lessons = map[string]Lesson{
	"buffer-crisis": BufferCrisisLesson{},
}
