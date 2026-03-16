// Package main is the entry point for the wtgc CLI.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/sivchari/wtgc/internal/config"
	"github.com/sivchari/wtgc/internal/daemon"
	"github.com/sivchari/wtgc/internal/provider"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// stringSlice is a flag.Value that accumulates repeated --flag values.
type stringSlice []string

func (s *stringSlice) String() string {
	if s == nil {
		return ""
	}

	return strings.Join(*s, ",")
}

func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)

	return nil
}

func main() {
	if len(os.Args) < 2 { //nolint:mnd // 2 = program + subcommand
		printUsage()
		os.Exit(1)
	}

	sub := os.Args[1]

	switch sub {
	case "daemon":
		runSubcommand(sub, runDaemon)

	case "run":
		runSubcommand(sub, runOnce)

	case "list":
		runSubcommand(sub, runList)

	case "version":
		fmt.Printf("wtgc %s (commit: %s, built at: %s)\n", version, commit, date)

	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %q\n", sub)
		printUsage()
		os.Exit(1)
	}
}

// printUsage prints the top-level usage message.
func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: wtgc <subcommand> [flags]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Subcommands:")
	fmt.Fprintln(os.Stderr, "  daemon   Run as a background daemon")
	fmt.Fprintln(os.Stderr, "  run      Run cleanup once and exit")
	fmt.Fprintln(os.Stderr, "  list     List stale worktrees")
	fmt.Fprintln(os.Stderr, "  version  Print version information")
}

// runSubcommand parses flags for a subcommand and calls fn.
// It returns an error so the caller can decide how to exit, avoiding os.Exit after defer.
func runSubcommand(name string, fn func(context.Context, config.Config) error) {
	if err := execSubcommand(name, fn); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// execSubcommand is the testable core of runSubcommand.
func execSubcommand(name string, fn func(context.Context, config.Config) error) error {
	fs, cfg, dirs, exclude := newFlagSet(name)

	if err := fs.Parse(os.Args[2:]); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	cfg.Directories = []string(*dirs)
	cfg.Exclude = []string(*exclude)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := fn(ctx, cfg); err != nil {
		return fmt.Errorf("run: %w", err)
	}

	return nil
}

// newFlagSet creates a FlagSet pre-populated with common flags bound to a Config.
func newFlagSet(name string) (*flag.FlagSet, config.Config, *stringSlice, *stringSlice) {
	defaults := config.Default()
	fs := flag.NewFlagSet(name, flag.ContinueOnError)

	cfg := config.Config{}

	fs.DurationVar(&cfg.Interval, "interval", defaults.Interval, "Check interval for daemon mode.")
	fs.DurationVar(&cfg.MaxAge, "max-age", defaults.MaxAge, "Worktrees older than this are considered stale.")
	fs.StringVar(&cfg.Provider, "provider", defaults.Provider, "Provider name.")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "Only log what would be deleted.")
	fs.BoolVar(&cfg.Force, "force", false, "Delete even with uncommitted changes.")
	fs.StringVar(&cfg.LogLevel, "log-level", defaults.LogLevel, "Log level: debug, info, warn, error.")

	var dirs stringSlice

	fs.Var(&dirs, "dir", "Directory to scan for git repositories (repeatable).")

	var exclude stringSlice

	fs.Var(&exclude, "exclude", "Glob pattern for branches to exclude (repeatable).")

	return fs, cfg, &dirs, &exclude
}

// buildLogger constructs a slog.Logger from the config's log level.
//
//nolint:gocritic // hugeParam: cfg is passed by value intentionally; callers own the Config
func buildLogger(cfg config.Config) *slog.Logger {
	level := cfg.ParseSlogLevel()

	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

// buildProviders creates one provider per configured directory.
//
//nolint:gocritic // hugeParam: cfg is passed by value intentionally; callers own the Config
func buildProviders(cfg config.Config) []provider.Provider {
	providers := make([]provider.Provider, 0, len(cfg.Directories))

	for _, dir := range cfg.Directories {
		providers = append(providers, provider.NewGitProvider(dir))
	}

	return providers
}

// runDaemon runs the daemon loop, cycling through all providers.
//
//nolint:gocritic // hugeParam: cfg matches the subcommand function signature; callers own the Config value
func runDaemon(ctx context.Context, cfg config.Config) error {
	logger := buildLogger(cfg)
	providers := buildProviders(cfg)

	for _, p := range providers {
		d := daemon.New(cfg, p, logger)

		if err := d.Run(ctx); err != nil {
			return fmt.Errorf("daemon run: %w", err)
		}
	}

	return nil
}

// runOnce performs a single cleanup pass across all providers.
//
//nolint:gocritic // hugeParam: cfg matches the subcommand function signature; callers own the Config value
func runOnce(ctx context.Context, cfg config.Config) error {
	logger := buildLogger(cfg)
	providers := buildProviders(cfg)

	for _, p := range providers {
		d := daemon.New(cfg, p, logger)

		if err := d.RunOnce(ctx); err != nil {
			return fmt.Errorf("run once: %w", err)
		}
	}

	return nil
}

// runList prints stale worktrees without removing them.
//
//nolint:gocritic // hugeParam: cfg matches the subcommand function signature; callers own the Config value
func runList(ctx context.Context, cfg config.Config) error {
	cfg.DryRun = true

	return runOnce(ctx, cfg)
}
