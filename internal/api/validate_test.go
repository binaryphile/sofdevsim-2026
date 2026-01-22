package api

import "testing"

// TestIsValidSeed verifies seed validation logic.
// Pure calculation - no HTTP mocking needed.
func TestIsValidSeed(t *testing.T) {
	tests := []struct {
		name    string
		seed    int64
		wantErr bool
	}{
		{"zero is valid", 0, false},
		{"positive is valid", 42, false},
		{"large positive is valid", 1<<62, false},
		{"negative is invalid", -1, true},
		{"large negative is invalid", -999999, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := isValidSeed(tt.seed)
			if (err != nil) != tt.wantErr {
				t.Errorf("isValidSeed(%d) error = %v, wantErr %v", tt.seed, err, tt.wantErr)
			}
		})
	}
}
