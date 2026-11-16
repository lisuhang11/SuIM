// Package ratelimit exposes SuIM's limiter contracts and entry-point middleware.
package ratelimit

import "github.com/lisuhang11/SuIM/pkg/ratelimit/types"

var ErrLimitExceeded = types.ErrLimitExceeded

type DoneInfo = types.DoneInfo
type DoneFunc = types.DoneFunc
type Stat = types.Stat
type Limiter = types.Limiter
