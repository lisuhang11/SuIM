package webhook

import "context"

type ctxKey int

const operationIDKey ctxKey = 1

// HeaderOperationID is the HTTP header that carries the operation id.
const HeaderOperationID = "operationID"

// WithOperationID stores an operation id that webhook posts send as a header.
func WithOperationID(ctx context.Context, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, operationIDKey, id)
}

func operationID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(operationIDKey).(string)
	return id
}
