package provider

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/sivchari/wtgc/internal/worktree"
)

// GitProvider implements Provider using the git CLI.
type GitProvider struct {
	repoDir string
}

// NewGitProvider creates a new GitProvider rooted at repoDir.
func NewGitProvider(repoDir string) *GitProvider {
	return &GitProvider{repoDir: repoDir}
}

// Name returns the provider name.
func (g *GitProvider) Name() string {
	return "git-native"
}

// List returns all worktrees by parsing `git worktree list --porcelain` output.
func (g *GitProvider) List(ctx context.Context) ([]worktree.Worktree, error) {
	//nolint:gosec // repoDir is set by the caller; git is a trusted binary
	cmd := exec.CommandContext(ctx, "git", "-C", g.repoDir, "worktree", "list", "--porcelain")

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	worktrees, err := g.parsePorcelain(ctx, out)
	if err != nil {
		return nil, fmt.Errorf("parse porcelain output: %w", err)
	}

	return worktrees, nil
}

// Remove removes the worktree at wt.Path, optionally with --force.
//
//nolint:gocritic // hugeParam: wt satisfies the Provider interface signature; cannot use a pointer here
func (g *GitProvider) Remove(ctx context.Context, wt worktree.Worktree, force bool) error {
	args := []string{"-C", g.repoDir, "worktree", "remove"}

	if force {
		args = append(args, "--force")
	}

	args = append(args, wt.Path)

	//nolint:gosec // args are constructed from trusted, application-controlled inputs only
	cmd := exec.CommandContext(ctx, "git", args...)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree remove %s: %w", wt.Path, err)
	}

	return nil
}

// applyLine applies a single non-blank porcelain line to the current worktree entry.
func applyLine(line string, current *worktree.Worktree, first bool) {
	switch {
	case strings.HasPrefix(line, "worktree "):
		current.Path = strings.TrimPrefix(line, "worktree ")
		current.MainWorktree = first

	case strings.HasPrefix(line, "HEAD "):
		current.Head = strings.TrimPrefix(line, "HEAD ")

	case strings.HasPrefix(line, "branch "):
		branch := strings.TrimPrefix(line, "branch ")
		current.Branch = strings.TrimPrefix(branch, "refs/heads/")

	case line == "bare":
		current.Bare = true

	case line == "detached":
		current.Detached = true

	case strings.HasPrefix(line, "locked"):
		current.Locked = true

	case strings.HasPrefix(line, "prunable"):
		current.Prunable = true
	}
}

// parsePorcelain parses the output of `git worktree list --porcelain`.
func (g *GitProvider) parsePorcelain(ctx context.Context, data []byte) ([]worktree.Worktree, error) {
	var (
		worktrees []worktree.Worktree
		current   worktree.Worktree
		first     = true
	)

	scanner := bufio.NewScanner(bytes.NewReader(data))

	for scanner.Scan() {
		line := scanner.Text()

		if line != "" {
			applyLine(line, &current, first)

			continue
		}

		if current.Path == "" {
			continue
		}

		var err error

		worktrees, err = g.flushEntry(ctx, worktrees, &current)
		if err != nil {
			return nil, err
		}

		current = worktree.Worktree{}
		first = false
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan porcelain output: %w", err)
	}

	if current.Path == "" {
		return worktrees, nil
	}

	worktrees, err := g.flushEntry(ctx, worktrees, &current)
	if err != nil {
		return nil, err
	}

	return worktrees, nil
}

// flushEntry resolves the HEAD date for current and appends it to worktrees.
func (g *GitProvider) flushEntry(
	ctx context.Context,
	worktrees []worktree.Worktree,
	current *worktree.Worktree,
) ([]worktree.Worktree, error) {
	headDate, err := g.fetchHeadDate(ctx, current.Head)
	if err != nil {
		return nil, fmt.Errorf("fetch HEAD date for %s: %w", current.Path, err)
	}

	current.HeadDate = headDate

	return append(worktrees, *current), nil
}

// fetchHeadDate retrieves the committer date of the given commit hash.
func (g *GitProvider) fetchHeadDate(ctx context.Context, hash string) (time.Time, error) {
	if hash == "" {
		return time.Time{}, nil
	}

	//nolint:gosec // hash comes from trusted git porcelain output
	cmd := exec.CommandContext(ctx, "git", "-C", g.repoDir, "log", "-1", "--format=%cI", hash)

	out, err := cmd.Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("git log for hash %s: %w", hash, err)
	}

	raw := strings.TrimSpace(string(out))

	if raw == "" {
		return time.Time{}, nil
	}

	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse commit date %q: %w", raw, err)
	}

	return t, nil
}
