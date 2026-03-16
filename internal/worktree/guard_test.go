package worktree_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sivchari/wtgc/internal/worktree"
)

func TestReason_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		reason worktree.Reason
		want   string
	}{
		{
			name:   "main worktree",
			reason: worktree.ReasonMainWorktree,
			want:   "main worktree",
		},
		{
			name:   "locked",
			reason: worktree.ReasonLocked,
			want:   "locked",
		},
		{
			name:   "uncommitted changes",
			reason: worktree.ReasonUncommittedChanges,
			want:   "uncommitted changes",
		},
		{
			name:   "unknown reason",
			reason: worktree.Reason(99),
			want:   "unknown reason (99)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.reason.String()

			if got != tt.want {
				t.Errorf("Reason.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// assertGuardResult checks that got matches the expected Safe flag and Reasons slice.
func assertGuardResult(t *testing.T, got worktree.GuardResult, wantSafe bool, wantReasons []worktree.Reason) {
	t.Helper()

	if got.Safe != wantSafe {
		t.Errorf("CheckSafety().Safe = %v, want %v", got.Safe, wantSafe)
	}

	if len(got.Reasons) != len(wantReasons) {
		t.Errorf("CheckSafety().Reasons = %v, want %v", got.Reasons, wantReasons)

		return
	}

	for i, r := range got.Reasons {
		if r != wantReasons[i] {
			t.Errorf("CheckSafety().Reasons[%d] = %v, want %v", i, r, wantReasons[i])
		}
	}
}

func TestWorktree_CheckSafety(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mainWorktree bool
		locked       bool
		wantSafe     bool
		wantReasons  []worktree.Reason
	}{
		{
			name:        "safe worktree",
			wantSafe:    true,
			wantReasons: nil,
		},
		{
			name:         "main worktree",
			mainWorktree: true,
			wantSafe:     false,
			wantReasons:  []worktree.Reason{worktree.ReasonMainWorktree},
		},
		{
			name:        "locked worktree",
			locked:      true,
			wantSafe:    false,
			wantReasons: []worktree.Reason{worktree.ReasonLocked},
		},
		{
			name:         "both main and locked",
			mainWorktree: true,
			locked:       true,
			wantSafe:     false,
			wantReasons:  []worktree.Reason{worktree.ReasonMainWorktree, worktree.ReasonLocked},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wt := &worktree.Worktree{
				MainWorktree: tt.mainWorktree,
				Locked:       tt.locked,
			}

			got := wt.CheckSafety()

			assertGuardResult(t, got, tt.wantSafe, tt.wantReasons)
		})
	}
}

// initGitRepo initializes a git repository in the given directory with an initial commit.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()

	run := func(args ...string) {
		t.Helper()

		cmd := exec.CommandContext(t.Context(), args[0], args[1:]...) //nolint:gosec // test helper with fixed git commands
		cmd.Dir = dir

		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	run("git", "init")
	run("git", "config", "user.email", "test@example.com")
	run("git", "config", "user.name", "Test User")

	initFile := filepath.Join(dir, "init.txt")
	if err := os.WriteFile(initFile, []byte("init\n"), 0o600); err != nil {
		t.Fatalf("failed to write init file: %v", err)
	}

	run("git", "add", ".")
	run("git", "commit", "-m", "initial commit")
}

func TestHasUncommittedChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  bool
	}{
		{
			name:  "clean repo has no uncommitted changes",
			setup: func(_ *testing.T, _ string) {},
			want:  false,
		},
		{
			name: "modified file causes uncommitted changes",
			setup: func(t *testing.T, dir string) {
				t.Helper()

				path := filepath.Join(dir, "init.txt")
				if err := os.WriteFile(path, []byte("modified\n"), 0o600); err != nil {
					t.Fatalf("failed to modify file: %v", err)
				}
			},
			want: true,
		},
		{
			name: "untracked file causes uncommitted changes",
			setup: func(t *testing.T, dir string) {
				t.Helper()

				path := filepath.Join(dir, "untracked.txt")
				if err := os.WriteFile(path, []byte("untracked\n"), 0o600); err != nil {
					t.Fatalf("failed to write untracked file: %v", err)
				}
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()

			initGitRepo(t, dir)
			tt.setup(t, dir)

			got, err := worktree.HasUncommittedChanges(t.Context(), dir)
			if err != nil {
				t.Fatalf("HasUncommittedChanges() error = %v", err)
			}

			if got != tt.want {
				t.Errorf("HasUncommittedChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}
