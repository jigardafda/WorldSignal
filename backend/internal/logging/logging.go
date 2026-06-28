// Package logging ports backend/src/lib/logger.ts: a tiny scoped logger.
// Log output is not part of any parity contract.
package logging

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Logger writes scoped, leveled lines.
type Logger struct {
	scope string
	out   io.Writer
	errw  io.Writer
}

// New returns a logger for the given scope.
func New(scope string) *Logger {
	return &Logger{scope: scope, out: os.Stdout, errw: os.Stderr}
}

func (l *Logger) emit(level, msg string, extra ...any) {
	ts := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	line := fmt.Sprintf("%s %-5s [%s] %s", ts, strings.ToUpper(level), l.scope, msg)
	w := l.out
	if level == "error" || level == "warn" {
		w = l.errw
	}
	if len(extra) > 0 {
		_, _ = fmt.Fprintln(w, append([]any{line}, extra...)...)
	} else {
		_, _ = fmt.Fprintln(w, line)
	}
}

// Info logs at info level.
func (l *Logger) Info(msg string, extra ...any) { l.emit("info", msg, extra...) }

// Warn logs at warn level.
func (l *Logger) Warn(msg string, extra ...any) { l.emit("warn", msg, extra...) }

// Error logs at error level.
func (l *Logger) Error(msg string, extra ...any) { l.emit("error", msg, extra...) }

// Debug logs at debug level.
func (l *Logger) Debug(msg string, extra ...any) { l.emit("debug", msg, extra...) }
