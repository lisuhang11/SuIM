package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
	"github.com/lisuhang11/SuIM/services/group/internal/store"
)

// GroupApplicationResponse 由群主或管理员同意、拒绝一条未处理的入群申请。
func (s *Server) GroupApplicationResponse(ctx context.Context, req *grouppb.GroupApplicationResponseReq) (*grouppb.GroupApplicationResponseResp, error) {
	groupID, err := normalizeGroupID(req.GetGroupId())
	if err != nil {
		return nil, err
	}
	op, err := normalizeUserID(req.GetOpUserId())
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
		}
	}
	if err := s.store.HandleRequest(ctx, groupID, from, handledMsg, req.GetHandleResult(), op, now, member); err != nil {
		return nil, mapStore(err)
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
	}
	return &grouppb.GroupApplicationResponseResp{}, nil
}

// GetGroupApplicationList 返回操作者作为群主或管理员能看到的入群申请。
func (s *Server) GetGroupApplicationList(ctx context.Context, req *grouppb.GetGroupApplicationListReq) (*grouppb.GetGroupApplicationListResp, error) {
	userID, err := normalizeUserID(req.GetFromUserId())
	if err != nil {
		return nil, err
	}
	groupIDs, err := normalizeGroupIDs(req.GetGroupIds())
	if err != nil {
		return nil, err
	}
	results, err := normalizeHandleResults(req.GetHandleResults())
	if err != nil {
		return nil, err
	}
	offset, limit, err := page(req.GetOffset(), req.GetLimit())
	if err != nil {
		return nil, err
	}
	if len(groupIDs) == 0 {
		groupIDs, err = s.store.ManagedGroupIDs(ctx, userID)
		if err != nil {
			return nil, err
		}
	} else if err := s.requireManagerGroups(ctx, userID, groupIDs); err != nil {
		return nil, err
	}
	total, requests, err := s.store.PageGroupRequests(ctx, groupIDs, results, offset, limit)
	if err != nil {
		return nil, err
	}
	return &grouppb.GetGroupApplicationListResp{Total: int32(total), Requests: requestListPB(requests)}, nil
}

// GetUserReqApplicationList 返回用户自己发出的入群申请。
func (s *Server) GetUserReqApplicationList(ctx context.Context, req *grouppb.GetUserReqApplicationListReq) (*grouppb.GetUserReqApplicationListResp, error) {
	userID, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	groupIDs, err := normalizeGroupIDs(req.GetGroupIds())
	if err != nil {
		return nil, err
	}
	results, err := normalizeHandleResults(req.GetHandleResults())
	if err != nil {
		return nil, err
	}
	offset, limit, err := page(req.GetOffset(), req.GetLimit())
	if err != nil {
		return nil, err
	}
	if _, err := s.requireUsers(ctx, []string{userID}); err != nil {
		return nil, err
	}
	total, requests, err := s.store.PageUserRequests(ctx, userID, groupIDs, results, offset, limit)
	if err != nil {
		return nil, err
	}
	return &grouppb.GetUserReqApplicationListResp{Total: int32(total), Requests: requestListPB(requests)}, nil
}

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
