package logic

import (
	"context"
	"strconv"

	"github.com/lisuhang11/SuIM/pkg/webhook"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

type callbackMember struct {
	UserID    string `json:"userID"`
	RoleLevel int32  `json:"roleLevel"`
}

type callbackGroup struct {
	GroupID           string           `json:"groupID"`
	GroupName         string           `json:"groupName"`
	Notification      string           `json:"notification"`
	Introduction      string           `json:"introduction"`
	FaceURL           string           `json:"faceURL"`
	OwnerUserID       string           `json:"ownerUserID"`
	CreateTime        int64            `json:"createTime"`
	MemberCount       uint32           `json:"memberCount"`
	Ex                string           `json:"ex"`
	Status            int32            `json:"status"`
	CreatorUserID     string           `json:"creatorUserID"`
	GroupType         int32            `json:"groupType"`
	NeedVerification  int32            `json:"needVerification"`
	LookMemberInfo    int32            `json:"lookMemberInfo"`
	ApplyMemberFriend int32            `json:"applyMemberFriend"`
	InitMemberList    []callbackMember `json:"initMemberList"`
}

type beforeCreateGroupReq struct {
	webhook.CommonCallbackReq
	callbackGroup
}

type beforeCreateGroupResp struct {
	webhook.CommonCallbackResp
	GroupName         *string `json:"groupName"`
	Notification      *string `json:"notification"`
	Introduction      *string `json:"introduction"`
	FaceURL           *string `json:"faceURL"`
	Ex                *string `json:"ex"`
	GroupType         *int32  `json:"groupType"`
	NeedVerification  *int32  `json:"needVerification"`
	LookMemberInfo    *int32  `json:"lookMemberInfo"`
	ApplyMemberFriend *int32  `json:"applyMemberFriend"`
}

type joinMember struct {
	UserID string `json:"userID"`
	Ex     string `json:"ex"`
}

type beforeMembersJoinReq struct {
	webhook.CommonCallbackReq
	GroupID     string       `json:"groupID"`
	MembersList []joinMember `json:"memberList"`
	GroupEx     string       `json:"groupEx"`
}

type memberJoinCallback struct {
	UserID    *string `json:"userID"`
	Nickname  *string `json:"nickname"`
	FaceURL   *string `json:"faceURL"`
	RoleLevel *int32  `json:"roleLevel"`
	Ex        *string `json:"ex"`
}

type beforeMembersJoinResp struct {
	webhook.CommonCallbackResp
	MemberCallbackList []*memberJoinCallback `json:"memberCallbackList"`
}

type beforeInviteReq struct {
	webhook.CommonCallbackReq
	GroupID        string   `json:"groupID"`
	Reason         string   `json:"reason"`
	InvitedUserIDs []string `json:"invitedUserIDs"`
}

type beforeInviteResp struct {
	webhook.CommonCallbackResp
	RefusedMembersAccount []string `json:"refusedMembersAccount,omitempty"`
}

type beforeJoinReq struct {
	webhook.CommonCallbackReq
	GroupID    string `json:"groupID"`
	GroupType  string `json:"groupType"`
	ApplyID    string `json:"applyID"`
	ReqMessage string `json:"reqMessage"`
	Ex         string `json:"ex"`
}

type afterJoinReq struct {
	webhook.CommonCallbackReq
	GroupID       string `json:"groupID"`
	ReqMessage    string `json:"reqMessage"`
	JoinSource    int32  `json:"joinSource"`
	InviterUserID string `json:"inviterUserID"`
}

type afterQuitReq struct {
	webhook.CommonCallbackReq
	GroupID string `json:"groupID"`
	UserID  string `json:"userID"`
}

type afterKickReq struct {
	webhook.CommonCallbackReq
	GroupID       string   `json:"groupID"`
	KickedUserIDs []string `json:"kickedUserIDs"`
	Reason        string   `json:"reason"`
}

type afterDismissReq struct {
	webhook.CommonCallbackReq
	GroupID   string   `json:"groupID"`
	OwnerID   string   `json:"ownerID"`
	GroupType string   `json:"groupType"`
	MembersID []string `json:"membersID"`
}

type afterTransferReq struct {
	webhook.CommonCallbackReq
	GroupID        string `json:"groupID"`
	OldOwnerUserID string `json:"oldOwnerUserID"`
	NewOwnerUserID string `json:"newOwnerUserID"`
}

type setGroupInfoReq struct {
	webhook.CommonCallbackReq
	GroupID           string `json:"groupID"`
	GroupName         string `json:"groupName"`
	Notification      string `json:"notification"`
	Introduction      string `json:"introduction"`
	FaceURL           string `json:"faceURL"`
	Ex                string `json:"ex"`
	NeedVerification  int32  `json:"needVerification"`
	LookMemberInfo    int32  `json:"lookMemberInfo"`
	ApplyMemberFriend int32  `json:"applyMemberFriend"`
}

type setGroupInfoResp struct {
	webhook.CommonCallbackResp
	GroupName         *string `json:"groupName"`
	Notification      *string `json:"notification"`
	Introduction      *string `json:"introduction"`
	FaceURL           *string `json:"faceURL"`
	Ex                *string `json:"ex"`
	NeedVerification  *int32  `json:"needVerification"`
	LookMemberInfo    *int32  `json:"lookMemberInfo"`
	ApplyMemberFriend *int32  `json:"applyMemberFriend"`
}

type wrappedString struct {
	Value string `json:"value"`
}

type wrappedInt32 struct {
	Value int32 `json:"value"`
}

type setGroupInfoExReq struct {
	webhook.CommonCallbackReq
	GroupID           string         `json:"groupID"`
	GroupName         *wrappedString `json:"groupName"`
	Notification      *wrappedString `json:"notification"`
	Introduction      *wrappedString `json:"introduction"`
	FaceURL           *wrappedString `json:"faceURL"`
	Ex                *wrappedString `json:"ex"`
	NeedVerification  *wrappedInt32  `json:"needVerification"`
	LookMemberInfo    *wrappedInt32  `json:"lookMemberInfo"`
	ApplyMemberFriend *wrappedInt32  `json:"applyMemberFriend"`
}

type setGroupInfoExResp struct {
	webhook.CommonCallbackResp
	GroupName         *wrappedString `json:"groupName"`
	Notification      *wrappedString `json:"notification"`
	Introduction      *wrappedString `json:"introduction"`
	FaceURL           *wrappedString `json:"faceURL"`
	Ex                *wrappedString `json:"ex"`
	NeedVerification  *wrappedInt32  `json:"needVerification"`
	LookMemberInfo    *wrappedInt32  `json:"lookMemberInfo"`
	ApplyMemberFriend *wrappedInt32  `json:"applyMemberFriend"`
}

type setMemberReq struct {
	webhook.CommonCallbackReq
	GroupID   string  `json:"groupID"`
	UserID    string  `json:"userID"`
	Nickname  *string `json:"nickName"`
	FaceURL   *string `json:"faceURL"`
	RoleLevel *int32  `json:"roleLevel"`
	Ex        *string `json:"ex"`
}

type setMemberResp struct {
	webhook.CommonCallbackResp
	Nickname  *string `json:"nickName"`
	FaceURL   *string `json:"faceURL"`
	RoleLevel *int32  `json:"roleLevel"`
	Ex        *string `json:"ex"`
}

func (s *Server) beforeCreateGroup(ctx context.Context, group *model.Group, members []*model.Member) error {
	if s.hooks.Client == nil || !s.hooks.BeforeCreateGroup.Enable {
		return nil
	}
	req := beforeCreateGroupReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandBeforeCreateGroup},
		callbackGroup:     groupCallback(group, members),
	}
	var resp beforeCreateGroupResp
	if err := s.hooks.Client.SyncPost(ctx, webhook.CommandBeforeCreateGroup, req, &resp, &s.hooks.BeforeCreateGroup); err != nil {
		if err = webhook.Proceed(err); err != nil {
			return err
		}
	}
	return applyCreateGroup(group, &resp)
}

func (s *Server) afterCreateGroup(ctx context.Context, group *model.Group, members []*model.Member) {
	if s.hooks.Client == nil {
		return
	}
	req := beforeCreateGroupReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandAfterCreateGroup},
		callbackGroup:     groupCallback(group, members),
	}
	s.hooks.Client.AsyncPost(ctx, webhook.CommandAfterCreateGroup, req, &webhook.CommonCallbackResp{}, &s.hooks.AfterCreateGroup)
}

func (s *Server) beforeMembersJoin(ctx context.Context, groupID, groupEx string, members []*model.Member) error {
	if s.hooks.Client == nil || !s.hooks.BeforeMembersJoinGroup.Enable || len(members) == 0 {
		return nil
	}
	list := make([]joinMember, 0, len(members))
	for _, member := range members {
		list = append(list, joinMember{UserID: member.UserID, Ex: member.Ex})
	}
	req := beforeMembersJoinReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandBeforeMembersJoinGroup},
		GroupID:           groupID,
		MembersList:       list,
		GroupEx:           groupEx,
	}
	var resp beforeMembersJoinResp
	if err := webhook.Proceed(s.hooks.Client.SyncPost(ctx, webhook.CommandBeforeMembersJoinGroup, req, &resp, &s.hooks.BeforeMembersJoinGroup)); err != nil {
		return err
	}
	return applyMemberCallbacks(members, resp.MemberCallbackList)
}

func (s *Server) beforeInvite(ctx context.Context, groupID, reason string, userIDs []string) ([]string, error) {
	if s.hooks.Client == nil || !s.hooks.BeforeInviteJoinGroup.Enable {
		return nil, nil
	}
	req := beforeInviteReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandBeforeInviteJoinGroup},
		GroupID:           groupID,
		Reason:            reason,
		InvitedUserIDs:    userIDs,
	}
	var resp beforeInviteResp
	if err := webhook.Proceed(s.hooks.Client.SyncPost(ctx, webhook.CommandBeforeInviteJoinGroup, req, &resp, &s.hooks.BeforeInviteJoinGroup)); err != nil {
		return nil, err
	}
	return resp.RefusedMembersAccount, nil
}

func (s *Server) beforeJoin(ctx context.Context, group *model.Group, userID, message, ex string) error {
	if s.hooks.Client == nil || !s.hooks.BeforeJoinGroup.Enable {
		return nil
	}
	req := beforeJoinReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandBeforeJoinGroup},
		GroupID:           group.GroupID,
		GroupType:         strconv.FormatInt(int64(group.GroupType), 10),
		ApplyID:           userID,
		ReqMessage:        message,
		Ex:                ex,
	}
	var resp webhook.CommonCallbackResp
	return webhook.Proceed(s.hooks.Client.SyncPost(ctx, webhook.CommandBeforeJoinGroup, req, &resp, &s.hooks.BeforeJoinGroup))
}

func (s *Server) afterJoin(ctx context.Context, groupID, message, inviter string, joinSource int32) {
	if s.hooks.Client == nil {
		return
	}
	req := afterJoinReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandAfterJoinGroup},
		GroupID:           groupID,
		ReqMessage:        message,
		JoinSource:        joinSource,
		InviterUserID:     inviter,
	}
	s.hooks.Client.AsyncPost(ctx, webhook.CommandAfterJoinGroup, req, &webhook.CommonCallbackResp{}, &s.hooks.AfterJoinGroup)
}

func (s *Server) afterQuit(ctx context.Context, groupID, userID string) {
	if s.hooks.Client == nil {
		return
	}
	req := afterQuitReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandAfterQuitGroup},
		GroupID:           groupID,
		UserID:            userID,
	}
	s.hooks.Client.AsyncPost(ctx, webhook.CommandAfterQuitGroup, req, &webhook.CommonCallbackResp{}, &s.hooks.AfterQuitGroup)
}

func (s *Server) afterKick(ctx context.Context, groupID, userID string) {
	if s.hooks.Client == nil {
		return
	}
	req := afterKickReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandAfterKickGroup},
		GroupID:           groupID,
		KickedUserIDs:     []string{userID},
	}
	s.hooks.Client.AsyncPost(ctx, webhook.CommandAfterKickGroup, req, &webhook.CommonCallbackResp{}, &s.hooks.AfterKickGroup)
}

func (s *Server) afterDismiss(ctx context.Context, group *model.Group, members []string) {
	if s.hooks.Client == nil || group == nil {
		return
	}
	req := afterDismissReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandAfterDismissGroup},
		GroupID:           group.GroupID,
		OwnerID:           group.OwnerUserID,
		GroupType:         strconv.FormatInt(int64(group.GroupType), 10),
		MembersID:         members,
	}
	s.hooks.Client.AsyncPost(ctx, webhook.CommandAfterDismissGroup, req, &webhook.CommonCallbackResp{}, &s.hooks.AfterDismissGroup)
}

func (s *Server) afterTransfer(ctx context.Context, groupID, oldOwner, newOwner string) {
	if s.hooks.Client == nil {
		return
	}
	req := afterTransferReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandAfterTransferGroupOwner},
		GroupID:           groupID,
		OldOwnerUserID:    oldOwner,
		NewOwnerUserID:    newOwner,
	}
	s.hooks.Client.AsyncPost(ctx, webhook.CommandAfterTransferGroupOwner, req, &webhook.CommonCallbackResp{}, &s.hooks.AfterTransferGroupOwner)
}

func (s *Server) beforeSetGroupInfo(ctx context.Context, group *model.Group, patch *model.GroupPatch) error {
	if s.hooks.Client == nil || !s.hooks.BeforeSetGroupInfo.Enable {
		return nil
	}
	req := setGroupInfoBody(webhook.CommandBeforeSetGroupInfo, group, patch)
	var resp setGroupInfoResp
	if err := webhook.Proceed(s.hooks.Client.SyncPost(ctx, webhook.CommandBeforeSetGroupInfo, req, &resp, &s.hooks.BeforeSetGroupInfo)); err != nil {
		return err
	}
	return applyGroupInfo(patch, &resp)
}

func (s *Server) afterSetGroupInfo(ctx context.Context, group *model.Group) {
	if s.hooks.Client == nil || group == nil {
		return
	}
	req := setGroupInfoReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandAfterSetGroupInfo},
		GroupID:           group.GroupID,
		GroupName:         group.GroupName,
		Notification:      group.Notification,
		Introduction:      group.Introduction,
		FaceURL:           group.FaceURL,
		Ex:                group.Ex,
		NeedVerification:  group.NeedVerification,
		LookMemberInfo:    group.LookMemberInfo,
		ApplyMemberFriend: group.ApplyMemberFriend,
	}
	s.hooks.Client.AsyncPost(ctx, webhook.CommandAfterSetGroupInfo, req, &webhook.CommonCallbackResp{}, &s.hooks.AfterSetGroupInfo)
}

func (s *Server) beforeSetGroupInfoEx(ctx context.Context, group *model.Group, patch *model.GroupPatch) error {
	if s.hooks.Client == nil || !s.hooks.BeforeSetGroupInfoEx.Enable || patch.Ex == nil {
		return nil
	}
	req := setGroupInfoExBody(webhook.CommandBeforeSetGroupInfoEx, group, patch)
	var resp setGroupInfoExResp
	if err := webhook.Proceed(s.hooks.Client.SyncPost(ctx, webhook.CommandBeforeSetGroupInfoEx, req, &resp, &s.hooks.BeforeSetGroupInfoEx)); err != nil {
		return err
	}
	return applyGroupInfoEx(patch, &resp)
}

func (s *Server) afterSetGroupInfoEx(ctx context.Context, group *model.Group) {
	if s.hooks.Client == nil || group == nil {
		return
	}
	ex := group.Ex
	req := setGroupInfoExReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandAfterSetGroupInfoEx},
		GroupID:           group.GroupID,
		GroupName:         wrapString(group.GroupName),
		Notification:      wrapString(group.Notification),
		Introduction:      wrapString(group.Introduction),
		FaceURL:           wrapString(group.FaceURL),
		Ex:                wrapString(ex),
		NeedVerification:  wrapInt(group.NeedVerification),
		LookMemberInfo:    wrapInt(group.LookMemberInfo),
		ApplyMemberFriend: wrapInt(group.ApplyMemberFriend),
	}
	s.hooks.Client.AsyncPost(ctx, webhook.CommandAfterSetGroupInfoEx, req, &webhook.CommonCallbackResp{}, &s.hooks.AfterSetGroupInfoEx)
}

func (s *Server) beforeSetMember(ctx context.Context, groupID, userID string, patch *model.MemberPatch) error {
	if s.hooks.Client == nil || !s.hooks.BeforeSetGroupMemberInfo.Enable {
		return nil
	}
	req := setMemberReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandBeforeSetGroupMemberInfo},
		GroupID:           groupID,
		UserID:            userID,
		Nickname:          patch.Nickname,
		FaceURL:           patch.FaceURL,
		RoleLevel:         patch.RoleLevel,
		Ex:                patch.Ex,
	}
	var resp setMemberResp
	if err := webhook.Proceed(s.hooks.Client.SyncPost(ctx, webhook.CommandBeforeSetGroupMemberInfo, req, &resp, &s.hooks.BeforeSetGroupMemberInfo)); err != nil {
		return err
	}
	return applyMemberPatch(patch, &resp)
}

func (s *Server) afterSetMember(ctx context.Context, member *model.Member) {
	if s.hooks.Client == nil || member == nil {
		return
	}
	nickname, face, ex := member.Nickname, member.FaceURL, member.Ex
	role := member.RoleLevel
	req := setMemberReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandAfterSetGroupMemberInfo},
		GroupID:           member.GroupID,
		UserID:            member.UserID,
		Nickname:          &nickname,
		FaceURL:           &face,
		RoleLevel:         &role,
		Ex:                &ex,
	}
	s.hooks.Client.AsyncPost(ctx, webhook.CommandAfterSetGroupMemberInfo, req, &webhook.CommonCallbackResp{}, &s.hooks.AfterSetGroupMemberInfo)
}

func groupCallback(group *model.Group, members []*model.Member) callbackGroup {
	body := callbackGroup{
		GroupID:           group.GroupID,
		GroupName:         group.GroupName,
		Notification:      group.Notification,
		Introduction:      group.Introduction,
		FaceURL:           group.FaceURL,
		OwnerUserID:       group.OwnerUserID,
		Ex:                group.Ex,
		Status:            group.Status,
		CreatorUserID:     group.CreatorUserID,
		GroupType:         group.GroupType,
		NeedVerification:  group.NeedVerification,
		LookMemberInfo:    group.LookMemberInfo,
		ApplyMemberFriend: group.ApplyMemberFriend,
		MemberCount:       uint32(len(members)),
	}
	if !group.CreateTime.IsZero() {
		body.CreateTime = group.CreateTime.UnixMilli()
	}
	for _, member := range members {
		body.InitMemberList = append(body.InitMemberList, callbackMember{UserID: member.UserID, RoleLevel: member.RoleLevel})
	}
	return body
}

func applyCreateGroup(group *model.Group, resp *beforeCreateGroupResp) error {
	if resp.GroupName != nil {
		name, err := normalizeName(*resp.GroupName, true)
		if err != nil {
			return err
		}
		group.GroupName = name
	}
	if resp.Notification != nil {
		value, err := normalizeLimited(*resp.Notification, 8000)
		if err != nil {
			return err
		}
		group.Notification = value
	}
	if resp.Introduction != nil {
		value, err := normalizeLimited(*resp.Introduction, 1024)
		if err != nil {
			return err
		}
		group.Introduction = value
	}
	if resp.FaceURL != nil {
		value, err := normalizeLimited(*resp.FaceURL, 512)
		if err != nil {
			return err
		}
		group.FaceURL = value
	}
	if resp.Ex != nil {
		value, err := normalizeLimited(*resp.Ex, 1024)
		if err != nil {
			return err
		}
		group.Ex = value
	}
	if resp.GroupType != nil {
		value, err := normalizeGroupType(*resp.GroupType)
		if err != nil {
			return err
		}
		group.GroupType = value
	}
	if resp.NeedVerification != nil {
		if err := normalizeVerification(*resp.NeedVerification); err != nil {
			return err
		}
		group.NeedVerification = *resp.NeedVerification
	}
	if resp.LookMemberInfo != nil {
		if err := normalizeFlag(*resp.LookMemberInfo); err != nil {
			return err
		}
		group.LookMemberInfo = *resp.LookMemberInfo
	}
	if resp.ApplyMemberFriend != nil {
		if err := normalizeFlag(*resp.ApplyMemberFriend); err != nil {
			return err
		}
		group.ApplyMemberFriend = *resp.ApplyMemberFriend
	}
	return nil
}

func applyMemberCallbacks(members []*model.Member, list []*memberJoinCallback) error {
	byID := make(map[string]*model.Member, len(members))
	for _, member := range members {
		byID[member.UserID] = member
	}
	for _, item := range list {
		if item == nil || item.UserID == nil {
			continue
		}
		member := byID[*item.UserID]
		if member == nil || member.RoleLevel == model.RoleOwner {
			continue
		}
		if item.Nickname != nil {
			name, err := normalizeName(*item.Nickname, false)
			if err != nil {
				return err
			}
			member.Nickname = name
		}
		if item.FaceURL != nil {
			face, err := normalizeLimited(*item.FaceURL, 512)
			if err != nil {
				return err
			}
			member.FaceURL = face
		}
		if item.Ex != nil {
			ex, err := normalizeLimited(*item.Ex, 1024)
			if err != nil {
				return err
			}
			member.Ex = ex
		}
		if item.RoleLevel != nil && *item.RoleLevel != model.RoleOwner {
			if *item.RoleLevel != model.RoleAdmin && *item.RoleLevel != model.RoleOrdinary {
				continue
			}
			member.RoleLevel = *item.RoleLevel
		}
	}
	return nil
}

func setGroupInfoBody(command string, group *model.Group, patch *model.GroupPatch) setGroupInfoReq {
	req := setGroupInfoReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: command},
		GroupID:           group.GroupID,
		GroupName:         first(patch.Name, group.GroupName),
		Notification:      first(patch.Notification, group.Notification),
		Introduction:      first(patch.Introduction, group.Introduction),
		FaceURL:           first(patch.FaceURL, group.FaceURL),
		Ex:                first(patch.Ex, group.Ex),
		NeedVerification:  firstInt(patch.NeedVerification, group.NeedVerification),
		LookMemberInfo:    firstInt(patch.LookMemberInfo, group.LookMemberInfo),
		ApplyMemberFriend: firstInt(patch.ApplyMemberFriend, group.ApplyMemberFriend),
	}
	return req
}

func applyGroupInfo(patch *model.GroupPatch, resp *setGroupInfoResp) error {
	if resp.GroupName != nil {
		name, err := normalizeName(*resp.GroupName, true)
		if err != nil {
			return err
		}
		patch.Name = &name
	}
	if resp.Notification != nil {
		value, err := normalizeLimited(*resp.Notification, 8000)
		if err != nil {
			return err
		}
		patch.Notification = &value
	}
	if resp.Introduction != nil {
		value, err := normalizeLimited(*resp.Introduction, 1024)
		if err != nil {
			return err
		}
		patch.Introduction = &value
	}
	if resp.FaceURL != nil {
		value, err := normalizeLimited(*resp.FaceURL, 512)
		if err != nil {
			return err
		}
		patch.FaceURL = &value
	}
	if resp.Ex != nil {
		value, err := normalizeLimited(*resp.Ex, 1024)
		if err != nil {
			return err
		}
		patch.Ex = &value
	}
	if resp.NeedVerification != nil {
		if err := normalizeVerification(*resp.NeedVerification); err != nil {
			return err
		}
		patch.NeedVerification = resp.NeedVerification
	}
	if resp.LookMemberInfo != nil {
		if err := normalizeFlag(*resp.LookMemberInfo); err != nil {
			return err
		}
		patch.LookMemberInfo = resp.LookMemberInfo
	}
	if resp.ApplyMemberFriend != nil {
		if err := normalizeFlag(*resp.ApplyMemberFriend); err != nil {
			return err
		}
		patch.ApplyMemberFriend = resp.ApplyMemberFriend
	}
	return nil
}

func setGroupInfoExBody(command string, group *model.Group, patch *model.GroupPatch) setGroupInfoExReq {
	return setGroupInfoExReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: command},
		GroupID:           group.GroupID,
		GroupName:         wrapString(first(patch.Name, group.GroupName)),
		Notification:      wrapString(first(patch.Notification, group.Notification)),
		Introduction:      wrapString(first(patch.Introduction, group.Introduction)),
		FaceURL:           wrapString(first(patch.FaceURL, group.FaceURL)),
		Ex:                wrapString(first(patch.Ex, group.Ex)),
		NeedVerification:  wrapInt(firstInt(patch.NeedVerification, group.NeedVerification)),
		LookMemberInfo:    wrapInt(firstInt(patch.LookMemberInfo, group.LookMemberInfo)),
		ApplyMemberFriend: wrapInt(firstInt(patch.ApplyMemberFriend, group.ApplyMemberFriend)),
	}
}

func applyGroupInfoEx(patch *model.GroupPatch, resp *setGroupInfoExResp) error {
	plain := setGroupInfoResp{}
	if resp.GroupName != nil {
		plain.GroupName = &resp.GroupName.Value
	}
	if resp.Notification != nil {
		plain.Notification = &resp.Notification.Value
	}
	if resp.Introduction != nil {
		plain.Introduction = &resp.Introduction.Value
	}
	if resp.FaceURL != nil {
		plain.FaceURL = &resp.FaceURL.Value
	}
	if resp.Ex != nil {
		plain.Ex = &resp.Ex.Value
	}
	if resp.NeedVerification != nil {
		plain.NeedVerification = &resp.NeedVerification.Value
	}
	if resp.LookMemberInfo != nil {
		plain.LookMemberInfo = &resp.LookMemberInfo.Value
	}
	if resp.ApplyMemberFriend != nil {
		plain.ApplyMemberFriend = &resp.ApplyMemberFriend.Value
	}
	return applyGroupInfo(patch, &plain)
}

func applyMemberPatch(patch *model.MemberPatch, resp *setMemberResp) error {
	if resp.Nickname != nil {
		name, err := normalizeName(*resp.Nickname, false)
		if err != nil {
			return err
		}
		patch.Nickname = &name
	}
	if resp.FaceURL != nil {
		face, err := normalizeLimited(*resp.FaceURL, 512)
		if err != nil {
			return err
		}
		patch.FaceURL = &face
	}
	if resp.Ex != nil {
		ex, err := normalizeLimited(*resp.Ex, 1024)
		if err != nil {
			return err
		}
		patch.Ex = &ex
	}
	if resp.RoleLevel != nil && *resp.RoleLevel != model.RoleOwner {
		if *resp.RoleLevel == model.RoleAdmin || *resp.RoleLevel == model.RoleOrdinary {
			patch.RoleLevel = resp.RoleLevel
		}
	}
	return nil
}

func first(value *string, fallback string) string {
	if value != nil {
		return *value
	}
	return fallback
}

func firstInt(value *int32, fallback int32) int32 {
	if value != nil {
		return *value
	}
	return fallback
}

func wrapString(value string) *wrappedString {
	return &wrappedString{Value: value}
}

func wrapInt(value int32) *wrappedInt32 {
	return &wrappedInt32{Value: value}
}

func dropIDs(ids, refused []string) []string {
	if len(refused) == 0 {
		return ids
	}
	skip := make(map[string]struct{}, len(refused))
	for _, id := range refused {
		skip[id] = struct{}{}
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := skip[id]; !ok {
			out = append(out, id)
		}
	}
	return out
}
