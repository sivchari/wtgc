// Package worktree defines the Worktree type representing a git worktree.
package worktree

import "time"

// Worktree holds metadata for a single git worktree entry.
type Worktree struct {
	// Path is the filesystem path to the worktree.
	Path string
	// Branch is the branch name checked out in this worktree.
	Branch string
	// Head is the HEAD commit hash.
	Head string
	// HeadDate is the committer date of HEAD.
	HeadDate time.Time
	// Bare reports whether this is a bare repository entry.
	Bare bool
	// Detached reports whether HEAD is detached.
	Detached bool
	// Locked reports whether the worktree is locked via git worktree lock.
	Locked bool
	// Prunable reports whether the worktree is prunable.
	Prunable bool
	// MainWorktree reports whether this is the main worktree (never deleted).
	MainWorktree bool
}
