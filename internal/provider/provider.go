// Package provider defines the Provider interface for worktree management backends.
package provider

import (
	"context"

	"github.com/sivchari/wtgc/internal/worktree"
)

// Provider is the interface that wraps worktree management operations.
type Provider interface {
	// Name returns the provider name (e.g., "git-native", "git-wt", "gwq").
	Name() string
	// List returns all worktrees managed by this provider.
	List(ctx context.Context) ([]worktree.Worktree, error)
	// Remove removes the specified worktree.
	Remove(ctx context.Context, wt worktree.Worktree, force bool) error
}
