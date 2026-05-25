package model

import (
	"errors"
	"testing"
)

// UC39 (#15445): table-driven coverage for ParseReleaseMode happy +
// ErrInvalidReleaseMode cases per Decision A (Domain heavy unit-test).

func TestParseReleaseMode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ReleaseMode
		wantErr error // sentinel; nil for happy
	}{
		{"empty string defaults to push", "", ReleaseModePush, nil},
		{"explicit push", "push", ReleaseModePush, nil},
		{"explicit demand", "demand", ReleaseModeDemand, nil},
		{"case-insensitive PUSH", "PUSH", ReleaseModePush, nil},
		{"case-insensitive Demand", "Demand", ReleaseModeDemand, nil},
		{"case-insensitive DEMAND", "DEMAND", ReleaseModeDemand, nil},
		{"unknown garbage returns ErrInvalidReleaseMode", "garbage", ReleaseModePush, ErrInvalidReleaseMode},
		{"unknown pull (common typo) returns sentinel", "pull", ReleaseModePush, ErrInvalidReleaseMode},
		{"whitespace not stripped → invalid", " push ", ReleaseModePush, ErrInvalidReleaseMode},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseReleaseMode(tc.input)
			if tc.wantErr == nil {
				if err != nil {
					t.Errorf("got err %v; want nil", err)
				}
				if got != tc.want {
					t.Errorf("got %v; want %v", got, tc.want)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got err %v; want errors.Is(err, %v)", err, tc.wantErr)
			}
		})
	}
}

func TestReleaseMode_String(t *testing.T) {
	tests := []struct {
		mode ReleaseMode
		want string
	}{
		{ReleaseModePush, "push"},
		{ReleaseModeDemand, "demand"},
	}
	for _, tc := range tests {
		if got := tc.mode.String(); got != tc.want {
			t.Errorf("ReleaseMode(%d).String() = %q; want %q", tc.mode, got, tc.want)
		}
	}
}

// Guard that the unwrap chain is intact — if a future refactor forgets
// %w, this test catches it.
func TestParseReleaseMode_ErrorsIsChain(t *testing.T) {
	_, err := ParseReleaseMode("nonsense")
	if !errors.Is(err, ErrInvalidReleaseMode) {
		t.Fatalf("err must unwrap to ErrInvalidReleaseMode; got %v", err)
	}
}
