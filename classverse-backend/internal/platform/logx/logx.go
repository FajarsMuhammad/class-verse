// Package logx builds the application slog logger.
package logx

import (
	"io"
	"log/slog"
)

// New returns a JSON logger in production and a text logger otherwise.
func New(w io.Writer, production bool) *slog.Logger {
	if production {
		return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
