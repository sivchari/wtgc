package worktree

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// Reason represents a reason why a worktree should not be removed.
type Reason int

const (
	// ReasonMainWorktree indicates the worktree is the main worktree.
	ReasonMainWorktree Reason = iota + 1
	// ReasonLocked indicates the worktree is locked.
	ReasonLocked
	// ReasonUncommittedChanges indicates the worktree has uncommitted changes.
	ReasonUncommittedChanges
)

// String returns a human-readable description of the reason.
func (r Reason) String() string {
	switch r {
	case ReasonMainWorktree:
		return "main worktree"
	case ReasonLocked:
		return "locked"
	case ReasonUncommittedChanges:
		return "uncommitted changes"
	default:
		return fmt.Sprintf("unknown reason (%d)", int(r))
	}
}

// GuardResult holds the result of a safety check.
type GuardResult struct {
	Safe    bool
	Reasons []Reason
}

// CheckSafety checks whether the worktree is safe to remove.
// It checks: MainWorktree flag, Locked flag.
// Uncommitted changes detection requires running git status, so provide a separate function for that.
func (wt *Worktree) CheckSafety() GuardResult {
	var reasons []Reason

	if wt.MainWorktree {
		reasons = append(reasons, ReasonMainWorktree)
	}

	if wt.Locked {
		reasons = append(reasons, ReasonLocked)
	}

	if len(reasons) == 0 {
		return GuardResult{Safe: true}
	}

	return GuardResult{
		Safe:    false,
		Reasons: reasons,
	}
}

// HasUncommittedChanges runs `git status --porcelain` in the worktree directory
// and returns true if there are any uncommitted changes.
func HasUncommittedChanges(ctx context.Context, path string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", path, "status", "--porcelain") //nolint:gosec // arguments are fixed strings except path

	var out bytes.Buffer

	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("git status in %s: %w", path, err)
	}

	return out.Len() > 0, nil
}
