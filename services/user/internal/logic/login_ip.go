package logic

import (
	"context"
	"strings"
)

func clipClientIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if len(ip) > 64 {
		return ip[:64]
	}
	return ip
}

func (s *Server) recordLoginIP(ctx context.Context, userID, ip string) {
	ip = clipClientIP(ip)
	if ip == "" {
		return
	}
	_ = s.store.TouchLastLoginIP(ctx, userID, ip)
}
