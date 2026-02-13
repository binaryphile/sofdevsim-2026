package office

import "testing"

func TestShouldBecomeFrustrated(t *testing.T) {
	tests := []struct {
		name      string
		state     AnimationState
		actual    float64
		estimated float64
		want      bool
	}{
		{"idle, over estimate", StateIdle, 3.0, 2.0, true},
		{"working, over estimate", StateWorking, 3.0, 2.0, true},
		{"already frustrated", StateFrustrated, 3.0, 2.0, false}, // key case: don't re-trigger
		{"under estimate", StateWorking, 1.0, 2.0, false},
		{"at estimate", StateWorking, 2.0, 2.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anim := DeveloperAnimation{State: tt.state}
			got := anim.ShouldBecomeFrustrated(tt.actual, tt.estimated)
			if got != tt.want {
				t.Errorf("ShouldBecomeFrustrated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldStartWorking(t *testing.T) {
	tests := []struct {
		name  string
		state AnimationState
		want  bool
	}{
		{"idle", StateIdle, true},
		{"conference", StateConference, true},
		{"already working", StateWorking, false},    // key case: don't re-trigger
		{"moving", StateMoving, false},              // still in transit
		{"frustrated", StateFrustrated, false},      // don't downgrade from frustrated
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anim := DeveloperAnimation{State: tt.state}
			got := anim.ShouldStartWorking()
			if got != tt.want {
				t.Errorf("ShouldStartWorking() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsActive(t *testing.T) {
	tests := []struct {
		name  string
		state AnimationState
		want  bool
	}{
		{"working", StateWorking, true},
		{"frustrated", StateFrustrated, true},
		{"idle", StateIdle, false},
		{"conference", StateConference, false},
		{"moving", StateMoving, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anim := DeveloperAnimation{State: tt.state}
			got := anim.IsActive()
			if got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}
