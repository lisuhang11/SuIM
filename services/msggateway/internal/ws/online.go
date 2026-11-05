package ws

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/online"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	gatewaypb "github.com/lisuhang11/SuIM/proto/msggateway"
)

func (s *Server) stillLocal(userID string, platformID int) bool {
	for _, id := range s.hub.Platforms(userID) {
		if int(id) == platformID {
			return true
		}
	}
	return false
}

func (s *Server) report(userID string, onlineIDs, offlineIDs []int32) {
	if s == nil || s.online == nil || userID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, _, err := s.online.Set(ctx, userID, onlineIDs, offlineIDs); err != nil {
		slog.Warn("set user online", "err", err, "userID", userID)
	}
}

func (s *Server) platforms(ctx context.Context, userID string) []int32 {
	if s.online != nil {
		platformIDs, err := s.online.Get(ctx, userID)
		if err == nil {
			return platformIDs
		}
		slog.Warn("get user online", "err", err, "userID", userID)
	}
	return s.hub.Platforms(userID)
}

// WatchOnline applies membership changes published by this process and by other gateways.
func (s *Server) WatchOnline(ctx context.Context) {
	if s == nil || s.online == nil {
		return
	}
	s.online.Subscribe(ctx, func(userID string, platformIDs []int32) {
		s.pushOnline(userID, platformIDs)
		s.keepLocal(userID, platformIDs)
	})
}

// RenewOnline pushes this process's platforms forward so a live connection does not expire.
func (s *Server) RenewOnline(ctx context.Context) {
	if s == nil || s.online == nil {
		return
	}
	ticker := time.NewTicker(online.Expire / 3)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.renewOnline(ctx)
		}
	}
}

// DropOnline removes the platforms still connected to this process.
func (s *Server) DropOnline(ctx context.Context) {
	if s == nil || s.online == nil {
		return
	}
	for _, row := range s.hub.Snapshot() {
		if ctx.Err() != nil {
			return
		}
		if _, _, err := s.online.Set(ctx, row.UserID, nil, row.Platforms); err != nil {
			slog.Warn("drop user online", "err", err, "userID", row.UserID)
		}
	}
}

func (s *Server) renewOnline(ctx context.Context) {
	for _, row := range s.hub.Snapshot() {
		if ctx.Err() != nil {
			return
		}
		platformIDs := s.hub.Platforms(row.UserID)
		if len(platformIDs) == 0 {
			continue
		}
		call, cancel := context.WithTimeout(ctx, 2*time.Second)
		_, _, err := s.online.Set(call, row.UserID, platformIDs, nil)
		cancel()
		if err != nil && ctx.Err() == nil {
			slog.Warn("renew user online", "err", err, "userID", row.UserID)
		}
	}
}

func (s *Server) pushOnline(userID string, platformIDs []int32) {
	body, err := proto.Marshal(&msgpb.SubUserOnlineStatusTips{Subscribers: []*msgpb.SubUserOnlineStatusElem{{
		UserId:            userID,
		OnlinePlatformIds: platformIDs,
	}}})
	if err != nil {
		return
	}
	for _, watcher := range s.hub.Watchers(userID) {
		_ = watcher.write(Resp{ReqIdentifier: reqSubUserOnline, Data: body})
	}
}

func (s *Server) keepLocal(userID string, published []int32) {
	seen := make(map[int32]struct{}, len(published))
	for _, platformID := range published {
		seen[platformID] = struct{}{}
	}
	var missing []int32
	for _, platformID := range s.hub.Platforms(userID) {
		if _, ok := seen[platformID]; !ok {
			missing = append(missing, platformID)
		}
	}
	if len(missing) == 0 {
		return
	}
	s.report(userID, missing, nil)
}

func (s *Server) onlineUsers(ctx context.Context, ids []string) ([]*gatewaypb.OnlineUser, error) {
	sets := map[string][]int32{}
	if len(ids) == 0 {
		listed, err := s.online.List(ctx)
		if err != nil {
			return nil, err
		}
		sets = listed
	} else {
		for _, userID := range ids {
			platformIDs, err := s.online.Get(ctx, userID)
			if err != nil {
				return nil, err
			}
			if len(platformIDs) > 0 {
				sets[userID] = platformIDs
			}
		}
	}
	users := make([]*gatewaypb.OnlineUser, 0, len(sets))
	for userID, platformIDs := range sets {
		if len(platformIDs) == 0 {
			continue
		}
		local := map[int32]*Client{}
		for _, client := range s.hub.Clients(userID) {
			local[int32(client.platformID)] = client
		}
		item := &gatewaypb.OnlineUser{UserId: userID, Status: onlineStatus}
		for _, platformID := range platformIDs {
			platform := &gatewaypb.Platform{PlatformId: platformID}
			if client := local[platformID]; client != nil {
				platform.ConnId = client.id
				platform.IsBackground = client.background
			}
			item.Platforms = append(item.Platforms, platform)
		}
		users = append(users, item)
	}
	return users, nil
}
