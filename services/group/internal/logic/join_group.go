package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
	"github.com/lisuhang11/SuIM/services/group/internal/store"
)

// JoinGroup 用户主动入群。群设置为直接加入时写入成员，否则只写入申请。
func (s *Server) JoinGroup(ctx context.Context, req *grouppb.JoinGroupReq) (*grouppb.JoinGroupResp, error) {
	groupID, err := normalizeGroupID(req.GetGroupId())
	if err != nil {
		return nil, err
	}
	userID, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	reqMsg, err := normalizeLimited(req.GetReqMsg(), 1024)
	if err != nil {
		return nil, err
	}
	ex, err := normalizeLimited(req.GetEx(), 1024)
	if err != nil {
		return nil, err
	}
	if err := normalizeJoinSource(req.GetJoinSource()); err != nil {
		return nil, err
	}
	if err := s.reserveJoin(ctx, userID); err != nil {
		return nil, err
	}

	profiles, err := s.requireUsers(ctx, []string{userID})
	if err != nil {
		return nil, err
	}
	group, err := s.activeGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if _, err := s.store.TakeMember(ctx, groupID, userID); err == nil {
		return nil, errorcode.ErrGroupAlreadyMember
	} else if !errors.Is(err, store.ErrNotMember) {
		return nil, mapStore(err)
	}

	// 入群前 webhook。回调拒绝则中断。
	if err := s.beforeJoin(ctx, group, userID, reqMsg, ex); err != nil {
		return nil, err
	}
	now := s.now()
	if group.NeedVerification == model.NeedDirectly {
		member := newMember(groupID, userID, profiles[userID], model.RoleOrdinary, userID, now)
		member.JoinSource = req.GetJoinSource()
		member.InviterUserID = ""
		member.Ex = ex
		if err := s.beforeMembersJoin(ctx, groupID, group.Ex, []*model.Member{member}); err != nil {
			return nil, err
		}
		if err := s.store.AddMembers(ctx, groupID, []*model.Member{member}, now, s.memberCap(group.GroupType)); err != nil {
			return nil, mapStore(err)
		}
		// 会话 seq 尚未接入。进入通知带上新的成员版本。
		if latest, err := s.store.TakeGroup(ctx, groupID); err == nil {
			group = latest
		}
		s.notifyEntered(ctx, userID, group, member)
		s.afterJoin(ctx, groupID, reqMsg, "", member.JoinSource)
		return &grouppb.JoinGroupResp{}, nil
	}

	request := &model.Request{
		GroupID:    groupID,
		UserID:     userID,
		ReqMsg:     reqMsg,
		JoinSource: req.GetJoinSource(),
		ReqTime:    now,
		Ex:         ex,
	}
	if err := s.store.CreateRequests(ctx, []*model.Request{request}); err != nil {
		return nil, mapStore(err)
	}
	s.notifyJoinApplication(ctx, userID, group, profiles[userID], request)
	return &grouppb.JoinGroupResp{}, nil
}
