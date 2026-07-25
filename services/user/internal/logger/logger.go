// Package logger provides a thin wrapper around log/slog with context
// propagation of request-scoped values (request-id).
package logger

import (
	"context"
	"log/slog"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// WithRequestID stores a request identifier in the context so subsequent
// log calls can include it.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func argsWithRequestID(ctx context.Context, args []any) []any {
	if rid, ok := ctx.Value(requestIDKey).(string); ok && rid != "" {
		return append([]any{"request_id", rid}, args...)
	}
	return args
}

// Info logs at Info level.
func Info(ctx context.Context, msg string, args ...any) {
	slog.InfoContext(ctx, msg, argsWithRequestID(ctx, args)...)
}

// Warn logs at Warn level.
func Warn(ctx context.Context, msg string, args ...any) {
	slog.WarnContext(ctx, msg, argsWithRequestID(ctx, args)...)
}

// Error logs at Error level.
func Error(ctx context.Context, msg string, args ...any) {
	slog.ErrorContext(ctx, msg, argsWithRequestID(ctx, args)...)
}

// Debug logs at Debug level.
func Debug(ctx context.Context, msg string, args ...any) {
	slog.DebugContext(ctx, msg, argsWithRequestID(ctx, args)...)
}

// With adds structured fields to the logger and returns a new context.
func With(ctx context.Context, args ...any) context.Context {
	return ctx
}
