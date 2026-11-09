package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
	"github.com/lisuhang11/SuIM/services/group/internal/store"
)

type memberEdit struct {
	userID  string
	patch   model.MemberPatch
	role    bool
	profile bool
}

// 修改群成员的昵称、头像、角色和扩展字段。 不能把任何人设成群主。任命或调整他人角色只留给群主，管理员只能把自己降为普通成员。
func (s *Server) SetGroupMemberInfo(ctx context.Context, req *grouppb.SetGroupMemberInfoReq) (*grouppb.SetGroupMemberInfoResp, error) {
	op, err := bindActor(ctx, req.GetOpUserId())
	if err != nil {
		return nil, err
	}
	if len(req.GetMembers()) == 0 {
		return nil, errorcode.ErrGroupInvalidArgs.WithMsg("members are empty")
	}
	grouped := map[string][]memberEdit{}
	for _, member := range req.GetMembers() {
		groupID, err := normalizeGroupID(member.GetGroupId())
		if err != nil {
			return nil, err
		}
		userID, err := normalizeUserID(member.GetUserId())
		if err != nil {
			return nil, err
		}
		patch, err := memberPatch(member)
		if err != nil {
			return nil, err
		}
		if patch.Empty() {
			return nil, errorcode.ErrGroupNothingToUpdate
		}
		edits := grouped[groupID]
		for _, edit := range edits {
			if edit.userID == userID {
				return nil, errorcode.ErrGroupMemberRepeated
			}
		}
		grouped[groupID] = append(edits, memberEdit{
			userID:  userID,
			patch:   patch,
			role:    patch.RoleLevel != nil,
			profile: patch.ProfileChanged(),
		})
	}

	// 修改成员前 webhook。回调可以改昵称、头像、角色和扩展字段，拒绝则中断。
	// 回调改过的角色必须仍是这个操作者本来就能改的，否则整次请求不写入。
	updates := make(map[string][]store.MemberUpdate, len(grouped))
	for groupID, edits := range grouped {
		if _, err := s.activeGroup(ctx, groupID); err != nil {
			return nil, err
		}
		if err := s.authorizeMemberEdits(ctx, groupID, op, edits); err != nil {
			return nil, err
		}
		opMember, err := s.memberOf(ctx, groupID, op)
		if err != nil {
			return nil, err
		}
		groupUpdates := make([]store.MemberUpdate, 0, len(edits))
		for i := range edits {
			if err := s.beforeSetMember(ctx, groupID, edits[i].userID, &edits[i].patch); err != nil {
				return nil, err
			}
			target, err := s.memberOf(ctx, groupID, edits[i].userID)
			if err != nil {
				return nil, err
			}
			if err := allowRoleChange(opMember, target, edits[i].patch.RoleLevel); err != nil {
				return nil, err
			}
			edits[i].role = edits[i].patch.RoleLevel != nil && *edits[i].patch.RoleLevel != target.RoleLevel
			edits[i].profile = edits[i].patch.ProfileChanged()
			groupUpdates = append(groupUpdates, store.MemberUpdate{UserID: edits[i].userID, Patch: edits[i].patch})
		}
		updates[groupID] = groupUpdates
	}
	now := s.now()
	if err := s.store.UpdateGroupMembers(ctx, updates, now); err != nil {
		return nil, mapStore(err)
	}

	// 角色和资料变更分开通知。
	for groupID, edits := range grouped {
		group, err := s.store.TakeGroup(ctx, groupID)
		if err != nil {
			return nil, mapStore(err)
		}
		opMember, err := s.memberOf(ctx, groupID, op)
		if err != nil {
			return nil, err
		}
		for _, edit := range edits {
			changed, err := s.memberOf(ctx, groupID, edit.userID)
			if err != nil {
				return nil, err
			}
			s.notifyMemberInfo(ctx, op, group, opMember, changed, edit.role, edit.profile)
			s.afterSetMember(ctx, changed)
		}
	}
	return &grouppb.SetGroupMemberInfoResp{}, nil
}

func (s *Server) authorizeMemberEdits(ctx context.Context, groupID, op string, edits []memberEdit) error {
	ids := make([]string, 0, len(edits)+1)
	ids = append(ids, op)
	for _, edit := range edits {
		ids = append(ids, edit.userID)
	}
	members, err := s.store.FindMembers(ctx, groupID, ids)
	if err != nil {
		return err
	}
	found := make(map[string]*model.Member, len(members))
	for _, member := range members {
		found[member.UserID] = member
	}
	opMember, ok := found[op]
	if !ok {
		return errorcode.ErrGroupNoPermission
	}
	for _, edit := range edits {
		target, ok := found[edit.userID]
		if !ok {
			return errorcode.ErrGroupNotMember
		}
		if err := allowMemberEdit(opMember, target, edit.patch); err != nil {
			return err
		}
	}
	return nil
}

func allowMemberEdit(op, target *model.Member, patch model.MemberPatch) error {
	self := op.UserID == target.UserID
	if err := allowRoleChange(op, target, patch.RoleLevel); err != nil {
		return err
	}
	switch op.RoleLevel {
	case model.RoleOwner:
		return nil
	case model.RoleAdmin:
		if target.RoleLevel == model.RoleOwner || (target.RoleLevel == model.RoleAdmin && !self) {
			return errorcode.ErrGroupNoPermission
		}
		return nil
	case model.RoleOrdinary:
		if !self {
			return errorcode.ErrGroupNoPermission
		}
		return nil
	default:
		return errorcode.ErrGroupNoPermission
	}
}

// 只让群主改变别人的角色。角色没变时不拦截。 管理员不能任命管理员，只能把自己降为普通成员。群主不能改自己的角色，转让走单独的接口。
func allowRoleChange(op, target *model.Member, role *int32) error {
	if role == nil || *role == target.RoleLevel {
		return nil
	}
	if op.RoleLevel == model.RoleOwner {
		if op.UserID == target.UserID {
			return errorcode.ErrGroupNoPermission.WithMsg("group owner cannot change own role")
		}
		return nil
	}
	if op.UserID == target.UserID && op.RoleLevel == model.RoleAdmin && *role == model.RoleOrdinary {
		return nil
	}
	return errorcode.ErrGroupNoPermission.WithMsg("only the group owner can change roles")
}

func memberPatch(member *grouppb.SetGroupMemberInfo) (model.MemberPatch, error) {
	var patch model.MemberPatch
	if member.Nickname != nil {
		name, err := normalizeName(member.GetNickname(), false)
		if err != nil {
			return patch, err
		}
		patch.Nickname = &name
	}
	if member.FaceUrl != nil {
		face, err := normalizeLimited(member.GetFaceUrl(), 512)
		if err != nil {
			return patch, err
		}
		patch.FaceURL = &face
	}
	if member.RoleLevel != nil {
		role := member.GetRoleLevel()
		if role != model.RoleAdmin && role != model.RoleOrdinary {
			return patch, errorcode.ErrGroupInvalidArgs.WithMsg("role_level is invalid")
		}
		patch.RoleLevel = &role
	}
	if member.Ex != nil {
		ex, err := normalizeLimited(member.GetEx(), 1024)
		if err != nil {
			return patch, err
		}
		patch.Ex = &ex
	}
	return patch, nil
}
