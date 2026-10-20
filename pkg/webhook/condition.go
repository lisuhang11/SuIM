package webhook

import "context"

// WithCondition runs callback only when the before-webhook is enabled.
func WithCondition(ctx context.Context, before *BeforeConfig, callback func(context.Context) error) error {
	if before == nil || !before.Enable || callback == nil {
		return nil
	}
	return callback(ctx)
}
