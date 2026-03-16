package worktree

import "time"

// IsStale reports whether the worktree's HEAD commit is older than the given max age.
func (wt *Worktree) IsStale(maxAge time.Duration) bool {
	if wt.HeadDate.IsZero() {
		return false
	}

	return time.Since(wt.HeadDate) > maxAge
}
