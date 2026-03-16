package worktree_test

import (
	"testing"
	"time"

	"github.com/sivchari/wtgc/internal/worktree"
)

func TestWorktree_IsStale(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name     string
		headDate time.Time
		maxAge   time.Duration
		want     bool
	}{
		{
			name:     "stale when HeadDate is older than maxAge",
			headDate: now.Add(-48 * time.Hour),
			maxAge:   24 * time.Hour,
			want:     true,
		},
		{
			name:     "not stale when HeadDate is within maxAge",
			headDate: now.Add(-1 * time.Hour),
			maxAge:   24 * time.Hour,
			want:     false,
		},
		{
			name:     "not stale when HeadDate is zero",
			headDate: time.Time{},
			maxAge:   24 * time.Hour,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wt := &worktree.Worktree{HeadDate: tt.headDate}
			got := wt.IsStale(tt.maxAge)

			if got != tt.want {
				t.Errorf("IsStale() = %v, want %v", got, tt.want)
			}
		})
	}
}
