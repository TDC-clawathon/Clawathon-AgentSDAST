// Package logging configures structured, leveled logging for the whole tool via
// the standard library's log/slog. A single call to Setup installs the default
// logger so any package can use slog.Info/Debug/... with consistent formatting.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Setup installs a process-wide structured logger and returns it.
//
//	level  : debug | info | warn | error   (default info)
//	format : text | json                    (default text)
//
// Logs go to stderr so stdout stays clean for command output (reports, JSON).
func Setup(level, format string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}

	var handler slog.Handler
	if strings.EqualFold(format, "json") {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
