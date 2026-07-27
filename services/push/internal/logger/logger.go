// Package logger 封装 log/slog，自动注入 request-id。
package logger

import (
	"context"
	"log/slog"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// WithRequestID 将请求标识存入 context。
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func argsWithRequestID(ctx context.Context, args []any) []any {
	if rid, ok := ctx.Value(requestIDKey).(string); ok && rid != "" {
		return append([]any{"request_id", rid}, args...)
	}
	return args
}

// Info 输出 Info 级别日志。
func Info(ctx context.Context, msg string, args ...any) {
	slog.InfoContext(ctx, msg, argsWithRequestID(ctx, args)...)
}

// Warn 输出 Warn 级别日志。
func Warn(ctx context.Context, msg string, args ...any) {
	slog.WarnContext(ctx, msg, argsWithRequestID(ctx, args)...)
}

// Error 输出 Error 级别日志。
func Error(ctx context.Context, msg string, args ...any) {
	slog.ErrorContext(ctx, msg, argsWithRequestID(ctx, args)...)
}

// Debug 输出 Debug 级别日志。
func Debug(ctx context.Context, msg string, args ...any) {
	slog.DebugContext(ctx, msg, argsWithRequestID(ctx, args)...)
}
