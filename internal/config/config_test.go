package config_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/sivchari/wtgc/internal/config"
)

func TestDefault(t *testing.T) {
	t.Parallel()

	cfg := config.Default()

	if cfg.Interval != time.Hour {
		t.Errorf("Interval: got %v, want %v", cfg.Interval, time.Hour)
	}

	if cfg.MaxAge != 7*24*time.Hour {
		t.Errorf("MaxAge: got %v, want %v", cfg.MaxAge, 7*24*time.Hour)
	}

	if cfg.Provider != "git-native" {
		t.Errorf("Provider: got %q, want %q", cfg.Provider, "git-native")
	}

	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel: got %q, want %q", cfg.LogLevel, "info")
	}

	if cfg.DryRun {
		t.Error("DryRun: got true, want false")
	}

	if cfg.Force {
		t.Error("Force: got true, want false")
	}

	if len(cfg.Directories) != 0 {
		t.Errorf("Directories: got %v, want empty", cfg.Directories)
	}

	if len(cfg.Exclude) != 0 {
		t.Errorf("Exclude: got %v, want empty", cfg.Exclude)
	}
}

func TestParseSlogLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		logLevel string
		want     slog.Level
	}{
		{name: "debug", logLevel: "debug", want: slog.LevelDebug},
		{name: "info", logLevel: "info", want: slog.LevelInfo},
		{name: "warn", logLevel: "warn", want: slog.LevelWarn},
		{name: "error", logLevel: "error", want: slog.LevelError},
		{name: "unknown falls back to info", logLevel: "unknown", want: slog.LevelInfo},
		{name: "empty falls back to info", logLevel: "", want: slog.LevelInfo},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.Config{LogLevel: tc.logLevel}
			got := cfg.ParseSlogLevel()

			if got != tc.want {
				t.Errorf("ParseSlogLevel(%q): got %v, want %v", tc.logLevel, got, tc.want)
			}
		})
	}
}
