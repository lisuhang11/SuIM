// Package logger 封装 log/slog，提供带 request-id 上下文传播的日志工具。
package logger

import (
	"context"
	"log/slog"
)

// contextKey 用于上下文键类型。
type contextKey string

const requestIDKey contextKey = "request_id"

// WithRequestID 将请求标识存入上下文，以便后续日志自动携带。
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// argsWithRequestID 在日志参数前自动附加 request_id 字段。
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

// With 向上下文中添加结构化字段（当前为占位实现，直接返回原上下文）。
func With(ctx context.Context, args ...any) context.Context {
	return ctx
}
