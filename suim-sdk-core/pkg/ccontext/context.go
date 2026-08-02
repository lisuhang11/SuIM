package ccontext

import "context"

type ctxKey int

const (
	keyInfo ctxKey = iota
	keyOperationID
)

// GlobalConfig holds SDK runtime config (mirrors OpenIM ccontext.GlobalConfig).
type GlobalConfig struct {
	ApiAddr     string
	DataDir     string
	UserID      string
	Token       string
	PlatformID  int32
}

type infoKey struct{}

func WithInfo(ctx context.Context, info *GlobalConfig) context.Context {
	return context.WithValue(ctx, keyInfo, info)
}

func Info(ctx context.Context) *GlobalConfig {
	v, _ := ctx.Value(keyInfo).(*GlobalConfig)
	return v
}

func WithOperationID(ctx context.Context, operationID string) context.Context {
	return context.WithValue(ctx, keyOperationID, operationID)
}

func OperationID(ctx context.Context) string {
	v, _ := ctx.Value(keyOperationID).(string)
	return v
}
