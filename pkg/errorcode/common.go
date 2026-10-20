package errorcode

import "google.golang.org/grpc/codes"

const (
	// Internal means the service failed while doing its own work.
	Internal Code = 1001
	// WebhookNetwork means the callback HTTP call failed.
	WebhookNetwork Code = 1002
	// WebhookData means the callback response could not be decoded.
	WebhookData Code = 1003
	// WebhookContinue means the callback asked the business flow to go on.
	WebhookContinue Code = 1004
	// WebhookRejected means the callback forbade the business action.
	WebhookRejected Code = 1005
	// InvalidJSON means the HTTP body is not usable JSON.
	InvalidJSON Code = 1006
	// TokenRequired means the request did not carry a token.
	TokenRequired Code = 1007
	// NoPermission means the token user cannot perform this action.
	NoPermission Code = 1008
)

var (
	ErrInternal        = Define(Internal, codes.Internal, "internal error")
	ErrWebhookNetwork  = Define(WebhookNetwork, codes.Unavailable, "webhook request failed")
	ErrWebhookData     = Define(WebhookData, codes.Internal, "webhook response is invalid")
	ErrWebhookContinue = Define(WebhookContinue, codes.Aborted, "callback continue")
	ErrWebhookRejected = Define(WebhookRejected, codes.PermissionDenied, "callback rejected")
	ErrInvalidJSON     = Define(InvalidJSON, codes.InvalidArgument, "invalid json")
	ErrTokenRequired   = Define(TokenRequired, codes.Unauthenticated, "token is required")
	ErrNoPermission    = Define(NoPermission, codes.PermissionDenied, "no permission")
)
