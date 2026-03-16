package daemon_test

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/sivchari/wtgc/internal/config"
	"github.com/sivchari/wtgc/internal/daemon"
	"github.com/sivchari/wtgc/internal/provider"
)

// initRepo creates a new git repository in dir with an initial commit and
// configures user.email and user.name so commits succeed in CI.
func initRepo(t *testing.T, dir string) {
	t.Helper()

	run := func(args ...string) {
		t.Helper()

		//nolint:gosec // G204: args are fixed test-only strings; no user input involved
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir

		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")

	file := filepath.Join(dir, "README.md")

	if writeErr := os.WriteFile(file, []byte("# test\n"), 0o600); writeErr != nil {
		t.Fatalf("write README: %v", writeErr)
	}

	run("add", ".")
	run("commit", "-m", "initial commit")
}

// addWorktree adds a linked worktree at wtDir on a new branch named branch,
// then backdates the branch's HEAD commit so it appears stale.
func addWorktree(t *testing.T, repoDir, wtDir, branch string, stale bool) {
	t.Helper()

	run := func(dir string, args ...string) {
		t.Helper()

		//nolint:gosec // G204: args are fixed test-only strings; no user input involved
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir

		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run(repoDir, "worktree", "add", "-b", branch, wtDir)

	if stale {
		// Amend the commit date to be far in the past so IsStale returns true.
		pastDate := "2000-01-01T00:00:00+00:00"
		run(wtDir, "commit", "--allow-empty", "--amend", "--no-edit",
			"--date", pastDate,
		)

		// Also override the committer date via env.
		env := os.Environ()
		env = append(env, "GIT_COMMITTER_DATE="+pastDate, "GIT_AUTHOR_DATE="+pastDate)

		cmd := exec.CommandContext(t.Context(), "git", "commit", "--allow-empty", "--amend", "--no-edit")
		cmd.Dir = wtDir
		cmd.Env = env

		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git amend committer date: %v\n%s", err, out)
		}
	}
}

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestRunOnce_StaleWorktreeRemoved(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)

	wtDir := filepath.Join(t.TempDir(), "stale-wt")
	addWorktree(t, repoDir, wtDir, "stale-branch", true)

	cfg := config.Default()
	cfg.MaxAge = time.Millisecond // everything is stale
	cfg.Directories = []string{repoDir}

	p := provider.NewGitProvider(repoDir)
	d := daemon.New(cfg, p, newLogger())

	if err := d.RunOnce(t.Context()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	// The linked worktree directory should be gone.
	if _, err := os.Stat(wtDir); !os.IsNotExist(err) {
		t.Errorf("expected worktree directory %q to be removed", wtDir)
	}
}

func TestRunOnce_FreshWorktreeKept(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)

	wtDir := filepath.Join(t.TempDir(), "fresh-wt")
	addWorktree(t, repoDir, wtDir, "fresh-branch", false)

	cfg := config.Default()
	cfg.MaxAge = 365 * 24 * time.Hour // nothing is stale
	cfg.Directories = []string{repoDir}

	p := provider.NewGitProvider(repoDir)
	d := daemon.New(cfg, p, newLogger())

	if err := d.RunOnce(t.Context()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if _, err := os.Stat(wtDir); err != nil {
		t.Errorf("expected worktree directory %q to still exist, got: %v", wtDir, err)
	}
}

func TestRunOnce_DryRun(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)

	wtDir := filepath.Join(t.TempDir(), "dryrun-wt")
	addWorktree(t, repoDir, wtDir, "dryrun-branch", true)

	cfg := config.Default()
	cfg.MaxAge = time.Millisecond
	cfg.DryRun = true
	cfg.Directories = []string{repoDir}

	p := provider.NewGitProvider(repoDir)
	d := daemon.New(cfg, p, newLogger())

	if err := d.RunOnce(t.Context()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	// Dry-run must NOT remove the worktree.
	if _, err := os.Stat(wtDir); err != nil {
		t.Errorf("dry-run: worktree directory %q should still exist, got: %v", wtDir, err)
	}
}

func TestRunOnce_ExcludedBranchKept(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)

	wtDir := filepath.Join(t.TempDir(), "excluded-wt")
	addWorktree(t, repoDir, wtDir, "release/1.0", true)

	cfg := config.Default()
	cfg.MaxAge = time.Millisecond
	cfg.Exclude = []string{"release/*"}
	cfg.Directories = []string{repoDir}

	p := provider.NewGitProvider(repoDir)
	d := daemon.New(cfg, p, newLogger())

	if err := d.RunOnce(t.Context()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if _, err := os.Stat(wtDir); err != nil {
		t.Errorf("excluded branch: worktree %q should still exist, got: %v", wtDir, err)
	}
}

func TestRunOnce_LockedWorktreeKept(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)

	wtDir := filepath.Join(t.TempDir(), "locked-wt")
	addWorktree(t, repoDir, wtDir, "locked-branch", true)

	// Lock the worktree so CheckSafety returns unsafe.
	lockWorktree(t, repoDir, wtDir)

	cfg := config.Default()
	cfg.MaxAge = time.Millisecond
	cfg.Directories = []string{repoDir}

	p := provider.NewGitProvider(repoDir)
	d := daemon.New(cfg, p, newLogger())

	if err := d.RunOnce(t.Context()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if _, err := os.Stat(wtDir); err != nil {
		t.Errorf("locked worktree %q should still exist, got: %v", wtDir, err)
	}
}

func TestRunOnce_ForceRemovesDirtyWorktree(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)

	wtDir := filepath.Join(t.TempDir(), "dirty-wt")
	addWorktree(t, repoDir, wtDir, "dirty-branch", true)

	// Write an uncommitted file to make the worktree dirty.
	dirtyFile := filepath.Join(wtDir, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("dirty\n"), 0o600); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	cfg := config.Default()
	cfg.MaxAge = time.Millisecond
	cfg.Force = true
	cfg.Directories = []string{repoDir}

	p := provider.NewGitProvider(repoDir)
	d := daemon.New(cfg, p, newLogger())

	if err := d.RunOnce(t.Context()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	// Force mode must remove even a dirty worktree.
	if _, err := os.Stat(wtDir); !os.IsNotExist(err) {
		t.Errorf("force: expected dirty worktree %q to be removed", wtDir)
	}
}

func TestRunOnce_DirtyWorktreeKept(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)

	wtDir := filepath.Join(t.TempDir(), "dirty-kept-wt")
	addWorktree(t, repoDir, wtDir, "dirty-kept-branch", true)

	// Write an uncommitted file so HasUncommittedChanges returns true.
	dirtyFile := filepath.Join(wtDir, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("dirty\n"), 0o600); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	cfg := config.Default()
	cfg.MaxAge = time.Millisecond
	cfg.Force = false
	cfg.Directories = []string{repoDir}

	p := provider.NewGitProvider(repoDir)
	d := daemon.New(cfg, p, newLogger())

	if err := d.RunOnce(t.Context()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if _, err := os.Stat(wtDir); err != nil {
		t.Errorf("dirty worktree %q should still exist, got: %v", wtDir, err)
	}
}

func TestRun_StopsOnContextCancel(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)

	cfg := config.Default()
	cfg.MaxAge = 365 * 24 * time.Hour // nothing stale; fast path
	cfg.Interval = 20 * time.Millisecond
	cfg.Directories = []string{repoDir}

	p := provider.NewGitProvider(repoDir)
	d := daemon.New(cfg, p, newLogger())

	// Allow at least two ticks so the ticker branch is exercised.
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	if err := d.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRunOnce_InvalidExcludePatternSkipped(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRepo(t, repoDir)

	wtDir := filepath.Join(t.TempDir(), "invalid-exclude-wt")
	addWorktree(t, repoDir, wtDir, "feat/myfeature", true)

	cfg := config.Default()
	cfg.MaxAge = time.Millisecond
	// "[" is an invalid glob pattern; isExcluded should log the error and not match.
	cfg.Exclude = []string{"[invalid"}
	cfg.Directories = []string{repoDir}

	p := provider.NewGitProvider(repoDir)
	d := daemon.New(cfg, p, newLogger())

	if err := d.RunOnce(t.Context()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	// Invalid pattern must not prevent removal; worktree should be gone.
	if _, err := os.Stat(wtDir); !os.IsNotExist(err) {
		t.Errorf("expected worktree %q to be removed when exclude pattern is invalid", wtDir)
	}
}

// lockWorktree runs `git worktree lock` to lock the worktree at path.
func lockWorktree(t *testing.T, repoDir, wtDir string) {
	t.Helper()

	//nolint:gosec // G204: args are fixed test-only strings; no user input involved
	cmd := exec.CommandContext(t.Context(), "git", "-C", repoDir, "worktree", "lock", wtDir)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git worktree lock: %v\n%s", err, out)
	}
}
