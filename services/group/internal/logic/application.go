package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

func (s *Server) requireManager(ctx context.Context, groupID, userID string) (*model.Group, *model.Member, error) {
	group, err := s.activeGroup(ctx, groupID)
	if err != nil {
		return nil, nil, err
	}
	member, err := s.managerMember(ctx, groupID, userID)
	if err != nil {
		return nil, nil, err
	}
	return group, member, nil
}

func (s *Server) requireManagerGroups(ctx context.Context, userID string, groupIDs []string) error {
	for _, groupID := range groupIDs {
		if _, err := s.store.TakeGroup(ctx, groupID); err != nil {
			return mapStore(err)
		}
		if _, err := s.managerMember(ctx, groupID, userID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) managerMember(ctx context.Context, groupID, userID string) (*model.Member, error) {
	member, err := s.memberOf(ctx, groupID, userID)
	if err != nil {
		if errors.Is(err, errorcode.ErrGroupNotMember) {
			return nil, errorcode.ErrGroupNoPermission
		}
		return nil, err
	}
	if member.RoleLevel != model.RoleOwner && member.RoleLevel != model.RoleAdmin {
		return nil, errorcode.ErrGroupNoPermission
	}
	return member, nil
}

func requestListPB(requests []*model.Request) []*grouppb.GroupRequestInfo {
	out := make([]*grouppb.GroupRequestInfo, 0, len(requests))
	for _, request := range requests {
		out = append(out, requestPB(request))
	}
	return out
}

func requestPB(request *model.Request) *grouppb.GroupRequestInfo {
	if request == nil {
		return nil
	}
	var handled int64
	if !request.HandledTime.IsZero() {
		handled = request.HandledTime.UnixMilli()
	}
	return &grouppb.GroupRequestInfo{
		UserId:        request.UserID,
		GroupId:       request.GroupID,
		HandleResult:  request.HandleResult,
		ReqMsg:        request.ReqMsg,
		HandledMsg:    request.HandledMsg,
		ReqTime:       request.ReqTime.UnixMilli(),
		HandleUserId:  request.HandleUserID,
		HandledTime:   handled,
		JoinSource:    request.JoinSource,
		InviterUserId: request.InviterUserID,
		Ex:            request.Ex,
	}
}
