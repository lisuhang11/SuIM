package kafka

import (
	"context"

	"github.com/IBM/sarama"

	"github.com/lisuhang11/SuIM/pkg/rpcctx"
)

// Header keys carried on Kafka records (aligned with OpenIM naming).
const (
	HeaderOperationID = "operationID"
	HeaderOpUserID    = "opUserID"
)

type headerKey int

const operationIDKey headerKey = 1

// WithOperationID stores an operation id for producer headers.
func WithOperationID(ctx context.Context, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, operationIDKey, id)
}

// OperationID returns the id stored by WithOperationID.
func OperationID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(operationIDKey).(string)
	return id
}

// HeadersFromContext builds Kafka record headers from ctx (best-effort; never fails).
func HeadersFromContext(ctx context.Context) []sarama.RecordHeader {
	var out []sarama.RecordHeader
	if id := OperationID(ctx); id != "" {
		out = append(out, sarama.RecordHeader{Key: []byte(HeaderOperationID), Value: []byte(id)})
	}
	if user := rpcctx.UserID(ctx); user != "" {
		out = append(out, sarama.RecordHeader{Key: []byte(HeaderOpUserID), Value: []byte(user)})
	}
	return out
}

// ContextFromHeaders restores operator / operation id onto a background context.
func ContextFromHeaders(parent context.Context, headers []*sarama.RecordHeader) context.Context {
	if parent == nil {
		parent = context.Background()
	}
	ctx := parent
	for _, h := range headers {
		if h == nil {
			continue
		}
		switch string(h.Key) {
		case HeaderOperationID:
			if v := string(h.Value); v != "" {
				ctx = WithOperationID(ctx, v)
			}
		case HeaderOpUserID:
			if v := string(h.Value); v != "" {
				ctx = rpcctx.WithUser(ctx, v)
			}
		}
	}
	return ctx
}
