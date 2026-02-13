package office

// Layout constants
const (
	CubicleWidth   = 10
	CubicleHeight  = 4
	CubicleSpacing = 2
	CubicleStartX  = 40 // Right side of screen
	CubicleStartY  = 2

	ConferenceX      = 5
	ConferenceY      = 3
	ConferenceWidth  = 30
	ConferenceHeight = 5
)

// Calculation: CubicleLayout returns positions for n developers (1-6)
// Pure function: int → []Position
func CubicleLayout(n int) []Position {
	positions := make([]Position, n)
	for i := 0; i < n; i++ {
		row := i / 3 // 0 or 1
		col := i % 3 // 0, 1, or 2
		positions[i] = Position{
			X: CubicleStartX + col*(CubicleWidth+CubicleSpacing),
			Y: CubicleStartY + row*(CubicleHeight+CubicleSpacing),
		}
	}
	return positions
}

// Calculation: ConferencePosition returns center position in conference room
// Pure function: (int, int) → Position
func ConferencePosition(devIndex, total int) Position {
	if total == 0 {
		return Position{X: ConferenceX + ConferenceWidth/2, Y: ConferenceY + 2}
	}
	// Evenly space developers horizontally in conference room
	spacing := ConferenceWidth / (total + 1)
	return Position{
		X: ConferenceX + spacing*(devIndex+1),
		Y: ConferenceY + 2,
	}
}

// Calculation: Lerp linearly interpolates between two positions
// Pure function: (Position, Position, float64) → Position
func Lerp(from, to Position, t float64) Position {
	return Position{
		X: from.X + int(float64(to.X-from.X)*t),
		Y: from.Y + int(float64(to.Y-from.Y)*t),
	}
}
