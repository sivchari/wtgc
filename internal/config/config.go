// Package config provides configuration types and defaults for wtgc.
package config

import (
	"log/slog"
	"time"
)

// Config holds all configuration for the wtgc daemon and CLI.
type Config struct {
	// Interval is the check interval for daemon mode.
	Interval time.Duration
	// MaxAge is the age threshold; worktrees older than this are considered stale.
	MaxAge time.Duration
	// Provider is the provider name (e.g., "git-native").
	Provider string
	// Directories is the list of directories to scan for git repositories.
	Directories []string
	// DryRun, if true, only logs what would be deleted without actually deleting.
	DryRun bool
	// Force, if true, deletes worktrees even when they have uncommitted changes.
	Force bool
	// Exclude is a list of glob patterns for branch names to exclude from deletion.
	Exclude []string
	// LogLevel is the log level: "debug", "info", "warn", or "error".
	LogLevel string
}

// Default returns a Config with sensible default values.
func Default() Config {
	return Config{
		Interval: time.Hour,
		MaxAge:   7 * 24 * time.Hour,
		Provider: "git-native",
		LogLevel: "info",
	}
}

// ParseSlogLevel converts the LogLevel string to a slog.Level value.
// Unknown values fall back to slog.LevelInfo.
//
//nolint:gocritic // hugeParam: value receiver is intentional for the public API contract
func (c Config) ParseSlogLevel() slog.Level {
	switch c.LogLevel {
	case "debug":
		return slog.LevelDebug

	case "warn":
		return slog.LevelWarn

	case "error":
		return slog.LevelError

	default:
		return slog.LevelInfo
	}
}
