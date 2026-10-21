package logic

import (
	"context"
	"errors"
	"time"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/grouppb"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
	"github.com/lisuhang11/SuIM/services/group/internal/store"
)

func mapStore(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, store.ErrNotFound):
		return errorcode.ErrGroupNotFound
	case errors.Is(err, store.ErrNotMember):
		return errorcode.ErrGroupNotMember
	case errors.Is(err, store.ErrAlreadyMember):
		return errorcode.ErrGroupAlreadyMember
	case errors.Is(err, store.ErrRequestRepeated):
		return errorcode.ErrGroupRequestRepeated
	case errors.Is(err, store.ErrDismissed):
		return errorcode.ErrGroupDismissed
	default:
		return err
	}
}

func (s *Server) requireUsers(ctx context.Context, ids []string) (map[string]model.UserProfile, error) {
	profiles, err := s.users.GetUsers(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, ok := profiles[id]; !ok {
			return nil, errorcode.ErrGroupUserNotFound
		}
	}
	return profiles, nil
}

func (s *Server) activeGroup(ctx context.Context, groupID string) (*model.Group, error) {
	group, err := s.store.TakeGroup(ctx, groupID)
	if err != nil {
		return nil, mapStore(err)
	}
	if group.Status == model.GroupStatusDismissed {
		return nil, errorcode.ErrGroupDismissed
	}
	return group, nil
}

func (s *Server) memberOf(ctx context.Context, groupID, userID string) (*model.Member, error) {
	member, err := s.store.TakeMember(ctx, groupID, userID)
	if err != nil {
		return nil, mapStore(err)
	}
	return member, nil
}

func (s *Server) fillMembers(ctx context.Context, members []*model.Member) error {
	var missing []string
	seen := map[string]struct{}{}
	for _, member := range members {
		if member.Nickname != "" && member.FaceURL != "" {
			continue
		}
		if _, ok := seen[member.UserID]; ok {
			continue
		}
		seen[member.UserID] = struct{}{}
		missing = append(missing, member.UserID)
	}
	if len(missing) == 0 {
		return nil
	}
	profiles, err := s.users.GetUsers(ctx, missing)
	if err != nil {
		return err
	}
	for _, member := range members {
		profile, ok := profiles[member.UserID]
		if !ok {
			continue
		}
		if member.Nickname == "" {
			member.Nickname = profile.Nickname
		}
		if member.FaceURL == "" {
			member.FaceURL = profile.AvatarURL
		}
	}
	return nil
}

func groupPB(group *model.Group) *grouppb.GroupInfo {
	var updated int64
	if !group.NotificationUpdateTime.IsZero() {
		updated = group.NotificationUpdateTime.UnixMilli()
	}
	return &grouppb.GroupInfo{
		GroupId:                group.GroupID,
		GroupName:              group.GroupName,
		Notification:           group.Notification,
		Introduction:           group.Introduction,
		FaceUrl:                group.FaceURL,
		CreateTime:             group.CreateTime.UnixMilli(),
		Ex:                     group.Ex,
		Status:                 group.Status,
		CreatorUserId:          group.CreatorUserID,
		GroupType:              group.GroupType,
		NeedVerification:       group.NeedVerification,
		LookMemberInfo:         group.LookMemberInfo,
		ApplyMemberFriend:      group.ApplyMemberFriend,
		NotificationUpdateTime: updated,
		NotificationUserId:     group.NotificationUserID,
		MemberCount:            group.MemberCount,
		OwnerUserId:            group.OwnerUserID,
	}
}

func memberPB(member *model.Member) *grouppb.GroupMemberFullInfo {
	var mute int64
	if !member.MuteEndTime.IsZero() {
		mute = member.MuteEndTime.UnixMilli()
	}
	return &grouppb.GroupMemberFullInfo{
		GroupId:        member.GroupID,
		UserId:         member.UserID,
		Nickname:       member.Nickname,
		FaceUrl:        member.FaceURL,
		RoleLevel:      member.RoleLevel,
		JoinTime:       member.JoinTime.UnixMilli(),
		JoinSource:     member.JoinSource,
		InviterUserId:  member.InviterUserID,
		OperatorUserId: member.OperatorUserID,
		MuteEndTime:    mute,
		Ex:             member.Ex,
	}
}

func newMember(groupID, userID string, profile model.UserProfile, role int32, operator string, now time.Time) *model.Member {
	return &model.Member{
		GroupID:        groupID,
		UserID:         userID,
		Nickname:       profile.Nickname,
		FaceURL:        profile.AvatarURL,
		RoleLevel:      role,
		JoinTime:       now,
		JoinSource:     model.JoinByInvitation,
		InviterUserID:  operator,
		OperatorUserID: operator,
	}
}
