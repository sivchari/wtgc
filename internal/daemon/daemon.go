// Package daemon provides the periodic cleanup loop for wtgc.
package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/sivchari/wtgc/internal/config"
	"github.com/sivchari/wtgc/internal/provider"
	"github.com/sivchari/wtgc/internal/worktree"
)

// Daemon runs periodic worktree cleanup according to its configuration.
type Daemon struct {
	cfg      config.Config
	provider provider.Provider
	logger   *slog.Logger
}

// New creates a new Daemon with the given configuration, provider, and logger.
//
//nolint:gocritic // hugeParam: cfg satisfies the public constructor signature; callers own the Config value
func New(cfg config.Config, p provider.Provider, logger *slog.Logger) *Daemon {
	return &Daemon{
		cfg:      cfg,
		provider: p,
		logger:   logger,
	}
}

// Run starts the daemon loop. It performs an initial cleanup pass and then
// repeats at cfg.Interval until ctx is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	if err := d.RunOnce(ctx); err != nil {
		if ctx.Err() != nil {
			return nil
		}

		return err
	}

	ticker := time.NewTicker(d.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			if err := d.RunOnce(ctx); err != nil {
				if ctx.Err() != nil {
					return nil
				}

				d.logger.ErrorContext(ctx, "cleanup pass failed", slog.String("error", err.Error()))
			}
		}
	}
}

// RunOnce performs a single cleanup pass over all worktrees returned by the provider.
func (d *Daemon) RunOnce(ctx context.Context) error {
	worktrees, err := d.provider.List(ctx)
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}

	for i := range worktrees {
		d.processWorktree(ctx, &worktrees[i])
	}

	return nil
}

// processWorktree evaluates a single worktree and removes it when eligible.
func (d *Daemon) processWorktree(ctx context.Context, wt *worktree.Worktree) {
	if wt.MainWorktree {
		return
	}

	if !wt.IsStale(d.cfg.MaxAge) {
		return
	}

	if !d.isSafeToRemove(ctx, wt) {
		return
	}

	if d.isExcluded(wt.Branch) {
		d.logger.InfoContext(ctx, "skipping worktree: branch excluded",
			slog.String("path", wt.Path),
			slog.String("branch", wt.Branch),
		)

		return
	}

	if d.cfg.DryRun {
		d.logger.InfoContext(ctx, "dry-run: would remove worktree",
			slog.String("path", wt.Path),
			slog.String("branch", wt.Branch),
		)

		return
	}

	if removeErr := d.provider.Remove(ctx, *wt, d.cfg.Force); removeErr != nil {
		d.logger.ErrorContext(ctx, "failed to remove worktree",
			slog.String("path", wt.Path),
			slog.String("error", removeErr.Error()),
		)

		return
	}

	d.logger.InfoContext(ctx, "removed stale worktree",
		slog.String("path", wt.Path),
		slog.String("branch", wt.Branch),
	)
}

// isSafeToRemove returns true when the worktree passes all safety checks.
// It logs a reason for each check that fails.
func (d *Daemon) isSafeToRemove(ctx context.Context, wt *worktree.Worktree) bool {
	guard := wt.CheckSafety()
	if !guard.Safe {
		for _, reason := range guard.Reasons {
			d.logger.InfoContext(ctx, "skipping worktree",
				slog.String("path", wt.Path),
				slog.String("reason", reason.String()),
			)
		}

		return false
	}

	if d.cfg.Force {
		return true
	}

	dirty, checkErr := worktree.HasUncommittedChanges(ctx, wt.Path)
	if checkErr != nil {
		d.logger.WarnContext(ctx, "could not check uncommitted changes",
			slog.String("path", wt.Path),
			slog.String("error", checkErr.Error()),
		)

		return false
	}

	if dirty {
		d.logger.InfoContext(ctx, "skipping worktree",
			slog.String("path", wt.Path),
			slog.String("reason", worktree.ReasonUncommittedChanges.String()),
		)

		return false
	}

	return true
}

// isExcluded reports whether branch matches any pattern in cfg.Exclude.
func (d *Daemon) isExcluded(branch string) bool {
	for _, pattern := range d.cfg.Exclude {
		matched, err := filepath.Match(pattern, branch)
		if err != nil {
			d.logger.Warn("invalid exclude pattern",
				slog.String("pattern", pattern),
				slog.String("error", err.Error()),
			)

			continue
		}

		if matched {
			return true
		}
	}

	return false
}
