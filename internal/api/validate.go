package api

import "fmt"

// isValidSeed validates seed is non-negative. Pure calculation - no I/O.
func isValidSeed(seed int64) error {
	if seed < 0 {
		return fmt.Errorf("seed must be non-negative: %d", seed)
	}
	return nil
}
