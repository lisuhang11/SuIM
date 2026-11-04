package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
	"github.com/lisuhang11/SuIM/services/group/internal/store"
)

// GroupApplicationResponse 由群主或管理员同意、拒绝一条未处理的入群申请。
func (s *Server) GroupApplicationResponse(ctx context.Context, req *grouppb.GroupApplicationResponseReq) (*grouppb.GroupApplicationResponseResp, error) {
	groupID, err := normalizeGroupID(req.GetGroupId())
	if err != nil {
		return nil, err
	}
	op, err := bindActor(ctx, req.GetOpUserId())
	if err != nil {
		return nil, err
	}
	from, err := normalizeUserID(req.GetFromUserId())
	if err != nil {
		return nil, err
	}
	handledMsg, err := normalizeLimited(req.GetHandledMsg(), 1024)
	if err != nil {
		return nil, err
	}
	if err := normalizeHandleResult(req.GetHandleResult()); err != nil {
		return nil, err
	}

	profiles, err := s.requireUsers(ctx, []string{from})
	if err != nil {
		return nil, err
	}
	group, opMember, err := s.requireManager(ctx, groupID, op)
	if err != nil {
		return nil, err
	}
	request, err := s.store.TakeRequest(ctx, groupID, from)
	if err != nil {
		return nil, mapStore(err)
	}
	if request.HandleResult != model.HandlePending {
		return nil, errorcode.ErrGroupRequestHandled
	}

	now := s.now()
	var member *model.Member
	if req.GetHandleResult() == model.HandleAgree {
		if _, err := s.store.TakeMember(ctx, groupID, from); err == nil {
			// 人已经在群里，只结束申请。
		} else if !errors.Is(err, store.ErrNotMember) {
			return nil, mapStore(err)
		} else {
			member = newMember(groupID, from, profiles[from], model.RoleOrdinary, op, now)
			member.JoinSource = request.JoinSource
			member.InviterUserID = request.InviterUserID
			member.Ex = request.Ex
			if err := s.beforeMembersJoin(ctx, groupID, group.Ex, []*model.Member{member}); err != nil {
				return nil, err
			}
		}
	}
	if err := s.store.HandleRequest(ctx, groupID, from, handledMsg, req.GetHandleResult(), op, now, member, s.memberCap(group.GroupType)); err != nil {
		return nil, mapStore(err)
	}
	if member != nil {
		if err := s.reopenHistory(ctx, group, []string{from}); err != nil {
			return nil, err
		}
		s.createGroupConversations(ctx, group, []string{from})
	}
	request.HandleResult = req.GetHandleResult()
	request.HandledMsg = handledMsg
	request.HandleUserID = op
	request.HandledTime = now
	s.notifyApplicationHandled(ctx, op, group, opMember, profiles[from], request)
	if member != nil {
		if latest, err := s.store.TakeGroup(ctx, groupID); err == nil {
			group = latest
		}
		s.notifyAgreedEnter(ctx, op, group, opMember, member)
		s.afterJoin(ctx, groupID, request.ReqMsg, member.InviterUserID, member.JoinSource)
	}
	return &grouppb.GroupApplicationResponseResp{}, nil
}
