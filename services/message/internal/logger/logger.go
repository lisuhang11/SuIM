// Package logger 封装 log/slog，自动将 request-id 注入上下文日志。
package logger

import (
	"context"
	"log/slog"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// WithRequestID 将请求标识存入 context，后续日志调用会自动携带。
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// argsWithRequestID 从 context 提取 request_id 并附加到日志参数。
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

// With 向日志附加结构化字段并返回新 context。
func With(ctx context.Context, args ...any) context.Context {
	return ctx
}
