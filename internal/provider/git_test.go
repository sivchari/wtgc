package provider

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sivchari/wtgc/internal/worktree"
)

// setupTestRepo initialises a bare-minimum git repo in dir.
// It creates an initial commit and returns the commit hash.
func setupTestRepo(t *testing.T, dir string) string {
	t.Helper()

	run := func(args ...string) {
		t.Helper()

		env := append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@example.com",
			"GIT_AUTHOR_DATE=2024-01-01T00:00:00+00:00",
			"GIT_COMMITTER_DATE=2024-01-01T00:00:00+00:00",
		)

		//nolint:gosec // git is a trusted binary used only in tests
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir
		cmd.Env = env

		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")

	readme := filepath.Join(dir, "README.md")

	if err := os.WriteFile(readme, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}

	run("add", ".")
	run("commit", "-m", "init")

	//nolint:gosec // git is a trusted binary used only in tests
	cmd := exec.CommandContext(t.Context(), "git", "-C", dir, "rev-parse", "HEAD")

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}

	return string(out[:len(out)-1])
}

// addWorktree creates a new worktree in a subdirectory of dir.
func addWorktree(t *testing.T, repoDir, branch, wtPath string) {
	t.Helper()

	//nolint:gosec // git is a trusted binary used only in tests
	cmd := exec.CommandContext(t.Context(), "git", "-C", repoDir, "worktree", "add", "-b", branch, wtPath)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git worktree add: %v\n%s", err, out)
	}
}

// realPath resolves symlinks in path to get the canonical filesystem path.
func realPath(t *testing.T, path string) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", path, err)
	}

	return resolved
}

func TestGitProvider_Name(t *testing.T) {
	t.Parallel()

	p := NewGitProvider("/any")

	if p.Name() != "git-native" {
		t.Errorf("Name() = %q, want %q", p.Name(), "git-native")
	}
}

func TestGitProvider_List_main(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	hash := setupTestRepo(t, dir)

	p := NewGitProvider(dir)

	worktrees, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(worktrees) != 1 {
		t.Fatalf("List() returned %d worktrees, want 1", len(worktrees))
	}

	wt := worktrees[0]

	if !wt.MainWorktree {
		t.Error("first worktree should have MainWorktree=true")
	}

	if wt.Path != realPath(t, dir) {
		t.Errorf("Path = %q, want %q", wt.Path, realPath(t, dir))
	}

	if wt.Head != hash {
		t.Errorf("Head = %q, want %q", wt.Head, hash)
	}

	if wt.Branch != "main" {
		t.Errorf("Branch = %q, want %q", wt.Branch, "main")
	}

	if wt.HeadDate.IsZero() {
		t.Error("HeadDate should not be zero")
	}
}

func TestGitProvider_List_withWorktree(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	setupTestRepo(t, dir)

	wtDir := t.TempDir()
	addWorktree(t, dir, "feature", wtDir)

	p := NewGitProvider(dir)

	worktrees, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(worktrees) != 2 {
		t.Fatalf("List() returned %d worktrees, want 2", len(worktrees))
	}

	main := worktrees[0]

	if !main.MainWorktree {
		t.Error("first worktree should have MainWorktree=true")
	}

	wt := worktrees[1]

	if wt.MainWorktree {
		t.Error("second worktree should have MainWorktree=false")
	}

	if wt.Branch != "feature" {
		t.Errorf("Branch = %q, want %q", wt.Branch, "feature")
	}

	if wt.Path != realPath(t, wtDir) {
		t.Errorf("Path = %q, want %q", wt.Path, realPath(t, wtDir))
	}
}

func TestGitProvider_Remove(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	setupTestRepo(t, dir)

	wtDir := t.TempDir()
	addWorktree(t, dir, "to-remove", wtDir)

	p := NewGitProvider(dir)

	realWtDir := realPath(t, wtDir)
	wt := worktree.Worktree{Path: wtDir}

	if err := p.Remove(context.Background(), wt, false); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	worktrees, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List() after Remove() error: %v", err)
	}

	for _, w := range worktrees {
		if w.Path == realWtDir {
			t.Errorf("worktree %q still present after Remove()", realWtDir)
		}
	}
}

func TestGitProvider_Remove_force(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	setupTestRepo(t, dir)

	wtDir := t.TempDir()
	addWorktree(t, dir, "to-remove-force", wtDir)

	dirtyFile := filepath.Join(wtDir, "dirty.txt")

	if err := os.WriteFile(dirtyFile, []byte("dirty"), 0o600); err != nil {
		t.Fatal(err)
	}

	p := NewGitProvider(dir)

	realWtDir := realPath(t, wtDir)
	wt := worktree.Worktree{Path: wtDir}

	if err := p.Remove(context.Background(), wt, true); err != nil {
		t.Fatalf("Remove(force=true) error: %v", err)
	}

	worktrees, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List() after Remove(force) error: %v", err)
	}

	for _, w := range worktrees {
		if w.Path == realWtDir {
			t.Errorf("worktree %q still present after Remove(force)", realWtDir)
		}
	}
}
