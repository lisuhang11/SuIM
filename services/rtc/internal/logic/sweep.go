package logic

import (
	"context"
	"log/slog"
	"time"

	"github.com/lisuhang11/SuIM/pkg/call"
)

// 结束无人接听的通话，并通知双方。群通话超时通知仍在房间里的人。
func (s *Service) Sweep(ctx context.Context) {
	if s == nil || s.calls == nil {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ended, err := s.calls.Expire(ctx, time.Now())
			if err != nil {
				slog.Warn("expire calls", "err", err)
				continue
			}
			for _, sig := range ended {
				s.publish(ctx, sig, call.OpTimeout, []string{sig.CallerID, sig.CalleeID}, "")
			}
			groupEnded, err := s.calls.ExpireGroup(ctx, time.Now())
			if err != nil {
				slog.Warn("expire group calls", "err", err)
				continue
			}
			for _, step := range groupEnded {
				s.afterGroup(ctx, step, call.OpTimeout, "")
			}
		}
	}
}
