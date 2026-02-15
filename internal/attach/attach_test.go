package attach

import "testing"

func TestEstimateSteps(t *testing.T) {
	tests := []struct {
		action string
		want   int
	}{
		{"Rebase", 1},
		{"Rebase + Merge", 2},
		{"Rebase + Merge + Push", 3},
		{"Rebase + Merge + Remove worktree", 4},
		{"Rebase + Merge + Remove worktree + Push", 5},
		{"Pull + Rebase + Merge + Remove worktree + Push", 6},
		{"Pull + Rebase + Merge + Push", 4},
	}
	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			got := estimateSteps(tt.action)
			if got != tt.want {
				t.Errorf("estimateSteps(%q) = %d, want %d", tt.action, got, tt.want)
			}
		})
	}
}
