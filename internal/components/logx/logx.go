// Package logx builds the structured logger. It returns a logger instead of
// setting a global, so callers pass it down explicitly.
package logx

import (
	"io"
	"log/slog"
)

// New returns a logger writing to w: debug level when verbose, info otherwise;
// JSON handler when json is set, text otherwise.
func New(verbose, json bool, w io.Writer) *slog.Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{Level: level}
	if json {
		return slog.New(slog.NewJSONHandler(w, opts))
	}
	return slog.New(slog.NewTextHandler(w, opts))
}
