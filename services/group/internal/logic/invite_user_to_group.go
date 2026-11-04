package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

// InviteUserToGroup 邀请用户入群。全员需要验证时，普通成员只写入申请。
func (s *Server) InviteUserToGroup(ctx context.Context, req *grouppb.InviteUserToGroupReq) (*grouppb.InviteUserToGroupResp, error) {
	groupID, err := normalizeGroupID(req.GetGroupId())
	if err != nil {
		return nil, err
	}
	op, err := bindActor(ctx, req.GetOpUserId())
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
	if len(userIDs) > maxInviteUsers {
		return nil, errorcode.ErrGroupInvalidArgs.WithMsg("too many users")
	}
	if err := s.reserveInvite(ctx, op, len(userIDs)); err != nil {
		return nil, err
	}

	// 1. 群未解散，操作者必须已在群内。操作者取上下文里的用户。
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

	// 2. 邀请前 webhook。回调可以拒绝其中一部分用户。
	refused, err := s.beforeInvite(ctx, groupID, reason, userIDs)
	if err != nil {
		return nil, err
	}
	userIDs = dropIDs(userIDs, refused)
	if len(userIDs) == 0 {
		return &grouppb.InviteUserToGroupResp{}, nil
	}
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
		for _, request := range requests {
			s.notifyJoinApplication(ctx, op, group, profiles[request.UserID], request)
		}
		return &grouppb.InviteUserToGroupResp{}, nil
	}

	now := s.now()
	members := make([]*model.Member, 0, len(userIDs))
	for _, id := range userIDs {
		members = append(members, newMember(groupID, id, profiles[id], model.RoleOrdinary, op, now))
	}
	// 3. 成员入群前 webhook。写入后清掉离开时留下的序号上限。
	if err := s.beforeMembersJoin(ctx, groupID, group.Ex, members); err != nil {
		return nil, err
	}
	if err := s.store.AddMembers(ctx, groupID, members, now, s.memberCap(group.GroupType)); err != nil {
		return nil, mapStore(err)
	}
	if err := s.reopenHistory(ctx, group, userIDs); err != nil {
		return nil, err
	}
	s.createGroupConversations(ctx, group, userIDs)
	// 4. 邀请通知带上新的成员版本。
	if latest, err := s.store.TakeGroup(ctx, groupID); err == nil {
		group = latest
	}
	s.notifyInvited(ctx, op, group, opMember, members)
	if len(members) > 0 {
		s.afterJoin(ctx, groupID, reason, members[0].InviterUserID, members[0].JoinSource)
	}
	return &grouppb.InviteUserToGroupResp{}, nil
}
