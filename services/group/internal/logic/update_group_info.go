package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/grouppb"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

// UpdateGroupInfo 群主或管理员修改群名、公告、简介、头像和入群设置。
func (s *Server) UpdateGroupInfo(ctx context.Context, req *grouppb.UpdateGroupInfoReq) (*grouppb.UpdateGroupInfoResp, error) {
	// 调用方身份尚未从上下文读取，这里使用请求中的 op_user_id。
	groupID, err := normalizeGroupID(req.GetGroupId())
	if err != nil {
		return nil, err
	}
	op, err := normalizeUserID(req.GetOpUserId())
	if err != nil {
		return nil, err
	}
	patch, err := groupPatch(req)
	if err != nil {
		return nil, err
	}
	if patchEmpty(patch) {
		return nil, errorcode.ErrGroupNothingToUpdate
	}

	// 1. 群必须存在且未解散，操作者必须是群主或管理员。
	if _, err := s.activeGroup(ctx, groupID); err != nil {
		return nil, err
	}
	member, err := s.memberOf(ctx, groupID, op)
	if err != nil {
		if errors.Is(err, errorcode.ErrGroupNotMember) {
			return nil, errorcode.ErrGroupNoPermission
		}
		return nil, err
	}
	if member.RoleLevel != model.RoleOwner && member.RoleLevel != model.RoleAdmin {
		return nil, errorcode.ErrGroupNoPermission
	}

	// 2. 修改群资料前的 webhook 尚未接入。
	now := s.now()
	if patch.Notification != nil {
		patch.NotificationUserID = op
		patch.NotificationTime = now
	}
	if err := s.store.UpdateGroup(ctx, groupID, patch, now); err != nil {
		return nil, mapStore(err)
	}

	// 3. 群名、公告和资料变更通知，以及修改后 webhook 尚未接入。
	group, err := s.store.TakeGroup(ctx, groupID)
	if err != nil {
		return nil, mapStore(err)
	}
	return &grouppb.UpdateGroupInfoResp{Group: groupPB(group)}, nil
}

func groupPatch(req *grouppb.UpdateGroupInfoReq) (model.GroupPatch, error) {
	var patch model.GroupPatch
	if req.GroupName != nil {
		name, err := normalizeName(req.GetGroupName(), true)
		if err != nil {
			return patch, err
		}
		patch.Name = &name
	}
	if req.Notification != nil {
		value, err := normalizeLimited(req.GetNotification(), 8000)
		if err != nil {
			return patch, err
		}
		patch.Notification = &value
	}
	if req.Introduction != nil {
		value, err := normalizeLimited(req.GetIntroduction(), 1024)
		if err != nil {
			return patch, err
		}
		patch.Introduction = &value
	}
	if req.FaceUrl != nil {
		value, err := normalizeLimited(req.GetFaceUrl(), 512)
		if err != nil {
			return patch, err
		}
		patch.FaceURL = &value
	}
	if req.NeedVerification != nil {
		if err := normalizeVerification(req.GetNeedVerification()); err != nil {
			return patch, err
		}
		value := req.GetNeedVerification()
		patch.NeedVerification = &value
	}
	if req.LookMemberInfo != nil {
		if err := normalizeFlag(req.GetLookMemberInfo()); err != nil {
			return patch, err
		}
		value := req.GetLookMemberInfo()
		patch.LookMemberInfo = &value
	}
	if req.ApplyMemberFriend != nil {
		if err := normalizeFlag(req.GetApplyMemberFriend()); err != nil {
			return patch, err
		}
		value := req.GetApplyMemberFriend()
		patch.ApplyMemberFriend = &value
	}
	if req.Ex != nil {
		value, err := normalizeLimited(req.GetEx(), 1024)
		if err != nil {
			return patch, err
		}
		patch.Ex = &value
	}
	return patch, nil
}

func patchEmpty(patch model.GroupPatch) bool {
	return patch.Name == nil && patch.Notification == nil && patch.Introduction == nil && patch.FaceURL == nil &&
		patch.NeedVerification == nil && patch.LookMemberInfo == nil && patch.ApplyMemberFriend == nil && patch.Ex == nil
}
