// Package main is the entry point for the wtgc CLI.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/sivchari/wtgc/internal/config"
	"github.com/sivchari/wtgc/internal/daemon"
	"github.com/sivchari/wtgc/internal/provider"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

// newRootCmd builds the root cobra command and all subcommands.
func newRootCmd() *cobra.Command {
	cfg := config.Default()

	root := &cobra.Command{
		Use:           "wtgc",
		Short:         "Git worktree garbage collector",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Persistent flags shared by all subcommands.
	pf := root.PersistentFlags()
	pf.DurationVar(&cfg.Interval, "interval", cfg.Interval, "Check interval for daemon mode.")
	pf.DurationVar(&cfg.MaxAge, "max-age", cfg.MaxAge, "Worktrees older than this are considered stale.")
	pf.StringVar(&cfg.Provider, "provider", cfg.Provider, "Provider name.")
	pf.StringSliceVar(&cfg.Directories, "dir", nil, "Directories to scan for git repositories (repeatable).")
	pf.BoolVar(&cfg.DryRun, "dry-run", false, "Only log what would be deleted.")
	pf.BoolVar(&cfg.Force, "force", false, "Delete even with uncommitted changes.")
	pf.StringSliceVar(&cfg.Exclude, "exclude", nil, "Glob patterns for branches to exclude (repeatable).")
	pf.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level: debug, info, warn, error.")

	root.AddCommand(
		newDaemonCmd(&cfg),
		newRunCmd(&cfg),
		newListCmd(&cfg),
		newVersionCmd(),
	)

	return root
}

func newDaemonCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run as a background daemon",
		RunE: func(_ *cobra.Command, _ []string) error {
			return executeDaemon(cfg)
		},
	}
}

func newRunCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Run cleanup once and exit",
		RunE: func(_ *cobra.Command, _ []string) error {
			return executeRunOnce(cfg)
		},
	}
}

func newListCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List stale worktrees without removing them",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg.DryRun = true

			return executeRunOnce(cfg)
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("wtgc %s (commit: %s, built at: %s)\n", version, commit, date)
		},
	}
}

// buildLogger constructs a slog.Logger from the config's log level.
func buildLogger(cfg *config.Config) *slog.Logger {
	level := cfg.ParseSlogLevel()

	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

// buildProviders creates one provider per configured directory.
func buildProviders(cfg *config.Config) []provider.Provider {
	providers := make([]provider.Provider, 0, len(cfg.Directories))

	for _, dir := range cfg.Directories {
		providers = append(providers, provider.NewGitProvider(dir))
	}

	return providers
}

// executeDaemon runs the daemon loop for all configured providers.
func executeDaemon(cfg *config.Config) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger := buildLogger(cfg)
	providers := buildProviders(cfg)

	for _, p := range providers {
		d := daemon.New(*cfg, p, logger)

		if err := d.Run(ctx); err != nil {
			return fmt.Errorf("daemon run: %w", err)
		}
	}

	return nil
}

// executeRunOnce performs a single cleanup pass for all configured providers.
func executeRunOnce(cfg *config.Config) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger := buildLogger(cfg)
	providers := buildProviders(cfg)

	for _, p := range providers {
		d := daemon.New(*cfg, p, logger)

		if err := d.RunOnce(ctx); err != nil {
			return fmt.Errorf("run once: %w", err)
		}
	}

	return nil
}
