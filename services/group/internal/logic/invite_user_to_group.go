package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

// InviteUserToGroup 邀请用户入群。全员需要验证时，普通成员只写入申请。
func (s *Server) InviteUserToGroup(ctx context.Context, req *grouppb.InviteUserToGroupReq) (*grouppb.InviteUserToGroupResp, error) {
	groupID, err := normalizeGroupID(req.GetGroupId())
	if err != nil {
		return nil, err
	}
	op, err := normalizeUserID(req.GetOpUserId())
	if err != nil {
		return nil, err
	}
	userIDs, err := normalizeUserIDs(req.GetUserIds())
	if err != nil {
		return nil, err
	}
	reason, err := normalizeLimited(req.GetReason(), 1024)
	if err != nil {
		return nil, err
	}

	// 1. 群未解散，操作者必须已在群内。调用方身份尚未从上下文读取。
	group, err := s.activeGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	opMember, err := s.memberOf(ctx, groupID, op)
	if err != nil {
		if errors.Is(err, errorcode.ErrGroupNotMember) {
			return nil, errorcode.ErrGroupNoPermission
		}
		return nil, err
	}
	profiles, err := s.requireUsers(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	existing, err := s.store.FindMembers(ctx, groupID, userIDs)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return nil, errorcode.ErrGroupAlreadyMember
	}

	// 2. 邀请前 webhook 尚未接入。
	// 全员需要验证时，普通成员只能写入申请，不能直接拉人。
	if group.NeedVerification == model.NeedVerifyAll && opMember.RoleLevel != model.RoleOwner && opMember.RoleLevel != model.RoleAdmin {
		now := s.now()
		requests := make([]*model.Request, 0, len(userIDs))
		for _, id := range userIDs {
			requests = append(requests, &model.Request{
				GroupID:       groupID,
				UserID:        id,
				ReqMsg:        reason,
				JoinSource:    model.JoinByInvitation,
				InviterUserID: op,
				ReqTime:       now,
			})
		}
		if err := s.store.CreateRequests(ctx, requests); err != nil {
			return nil, mapStore(err)
		}
		// 入群申请通知尚未接入。
		return &grouppb.InviteUserToGroupResp{}, nil
	}

	now := s.now()
	members := make([]*model.Member, 0, len(userIDs))
	for _, id := range userIDs {
		members = append(members, newMember(groupID, id, profiles[id], model.RoleOrdinary, op, now))
	}
	// 3. 成员入群前 webhook 尚未接入。会话 seq 尚未接入。
	if err := s.store.AddMembers(ctx, groupID, members, now); err != nil {
		return nil, mapStore(err)
	}
	// 4. 入群通知和入群后 webhook 尚未接入。
	return &grouppb.InviteUserToGroupResp{}, nil
}
