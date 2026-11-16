// Package types holds the shared contracts of SuIM rate limiters. It is kept
// separate so policy implementations can depend on it without importing the
// middleware package that constructs them.
package types

import "errors"

// ErrLimitExceeded is returned when a request must be shed.
var ErrLimitExceeded = errors.New("rate limit exceeded")

// DoneInfo carries the outcome of a request back to the limiter. BBR ignores
// Err today, but the hook keeps room for error-aware adaptive policies.
type DoneInfo struct {
	Err error
}

// DoneFunc must be called exactly once when an allowed request finishes.
type DoneFunc func(DoneInfo)

// Stat is a snapshot of the signals used by the BBR policy.
type Stat struct {
	CPU         int64
	InFlight    int64
	MaxInFlight int64
	MinRT       int64
	MaxPass     int64
}

// Limiter is the runtime contract shared by gRPC and HTTP entry points.
type Limiter interface {
	Allow() (DoneFunc, error)
	Stat() Stat
}
