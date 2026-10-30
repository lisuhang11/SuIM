package logic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"

	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

const (
	msgFromSystem = 200

	sessionNormal = 2
	sessionSuper  = 3

	contentGroupCreated         = 1501
	contentJoinGroupApplication = 1503
	contentMemberQuit           = 1504
	contentApplicationAccepted  = 1505
	contentApplicationRejected  = 1506
	contentOwnerTransferred     = 1507
	contentMemberKicked         = 1508
	contentMemberInvited        = 1509
	contentMemberEnter          = 1510
	contentGroupDismissed       = 1511
	contentMemberMuted          = 1512
	contentMemberCancelMuted    = 1513
	contentGroupMuted           = 1514
	contentGroupCancelMuted     = 1515
	contentMemberInfoSet        = 1516
	contentMemberSetAdmin       = 1517
	contentMemberSetOrdinary    = 1518

	sessionSingle = 1

	receiverApplicant = 0
	receiverAdmin     = 1

	optNotNotification = "isNotNotification"
)

// 成员变更写成通知消息。msggateway 只负责把这条消息推给在线成员。
// 客户端在版本号正好是本地加一时直接套用，否则再拉 incremental_members。
func (s *Server) sendNotice(ctx context.Context, op string, group *model.Group, contentType int32, detail any) {
	if s.sender == nil || group == nil {
		return
	}
	msg, err := notificationMsg(op, group, contentType, detail, s.now().UnixMilli())
	if err != nil {
		slog.Warn("group notice", "err", err, "groupID", group.GroupID, "contentType", contentType)
		return
	}
	if err := s.sender.SendNotification(ctx, msg); err != nil {
		slog.Warn("group notice", "err", err, "groupID", group.GroupID, "contentType", contentType)
	}
}

func (s *Server) memberHead(ctx context.Context, groupID string) (string, uint64) {
	head, err := s.store.MemberVersion(ctx, groupID)
	if err != nil {
		slog.Warn("group member version", "err", err, "groupID", groupID)
		return "", 0
	}
	return head.ID, head.Version
}

func (s *Server) notifyCreated(ctx context.Context, owner string, group *model.Group, members []*model.Member) {
	versionID, version := s.memberHead(ctx, group.GroupID)
	var ownerMember *model.Member
	list := make([]noticeMember, 0, len(members))
	for _, member := range members {
		if member.UserID == owner {
			ownerMember = member
		}
		list = append(list, memberNotice(member))
	}
	s.sendNotice(ctx, owner, group, contentGroupCreated, groupCreatedTips{
		Group:                groupNotice(group),
		OpUser:               memberNotice(ownerMember),
		MemberList:           list,
		OperationTime:        s.now().UnixMilli(),
		GroupOwnerUser:       memberNotice(ownerMember),
		GroupMemberVersion:   version,
		GroupMemberVersionID: versionID,
	})
}

func (s *Server) notifyInvited(ctx context.Context, op string, group *model.Group, opMember *model.Member, invited []*model.Member) {
	versionID, version := s.memberHead(ctx, group.GroupID)
	list := make([]noticeMember, 0, len(invited))
	for _, member := range invited {
		list = append(list, memberNotice(member))
	}
	s.sendNotice(ctx, op, group, contentMemberInvited, memberInvitedTips{
		Group:                groupNotice(group),
		OpUser:               memberNotice(opMember),
		InvitedUserList:      list,
		OperationTime:        s.now().UnixMilli(),
		GroupMemberVersion:   version,
		GroupMemberVersionID: versionID,
		InviterUser:          memberNotice(opMember),
	})
}

func (s *Server) notifyQuit(ctx context.Context, group *model.Group, member *model.Member) {
	versionID, version := s.memberHead(ctx, group.GroupID)
	s.sendNotice(ctx, member.UserID, group, contentMemberQuit, memberQuitTips{
		Group:                groupNotice(group),
		QuitUser:             memberNotice(member),
		OperationTime:        s.now().UnixMilli(),
		GroupMemberVersion:   version,
		GroupMemberVersionID: versionID,
	})
}

func (s *Server) notifyKicked(ctx context.Context, op string, group *model.Group, opMember, target *model.Member) {
	versionID, version := s.memberHead(ctx, group.GroupID)
	s.sendNotice(ctx, op, group, contentMemberKicked, memberKickedTips{
		Group:                groupNotice(group),
		OpUser:               memberNotice(opMember),
		KickedUserList:       []noticeMember{memberNotice(target)},
		OperationTime:        s.now().UnixMilli(),
		GroupMemberVersion:   version,
		GroupMemberVersionID: versionID,
	})
}

func (s *Server) notifyTransferred(ctx context.Context, op string, group *model.Group, oldOwner, newOwner *model.Member) {
	versionID, version := s.memberHead(ctx, group.GroupID)
	s.sendNotice(ctx, op, group, contentOwnerTransferred, ownerTransferredTips{
		Group:                groupNotice(group),
		OpUser:               memberNotice(oldOwner),
		NewGroupOwner:        memberNotice(newOwner),
		OldGroupOwner:        op,
		OperationTime:        s.now().UnixMilli(),
		OldGroupOwnerInfo:    memberNotice(oldOwner),
		GroupMemberVersion:   version,
		GroupMemberVersionID: versionID,
	})
}

func (s *Server) notifyApplicationHandled(ctx context.Context, op string, group *model.Group, opMember *model.Member, applicant model.UserProfile, request *model.Request) {
	if s.sender == nil || group == nil || request == nil {
		return
	}
	contentType := int32(contentApplicationRejected)
	if request.HandleResult == model.HandleAgree {
		contentType = contentApplicationAccepted
	}
	managers, err := s.store.Managers(ctx, group.GroupID)
	if err != nil {
		slog.Warn("group managers", "err", err, "groupID", group.GroupID)
		return
	}
	user := noticePublicUser{UserID: applicant.UserID, Nickname: applicant.Nickname, FaceURL: applicant.AvatarURL}
	var handled int64
	if !request.HandledTime.IsZero() {
		handled = request.HandledTime.UnixMilli()
	}
	tips := applicationHandledTips{
		Group:     groupNotice(group),
		OpUser:    memberNotice(opMember),
		HandleMsg: request.HandledMsg,
		UUID:      newNoticeID(),
		Request: noticeGroupRequest{
			UserInfo:      user,
			GroupInfo:     groupNotice(group),
			HandleResult:  request.HandleResult,
			ReqMsg:        request.ReqMsg,
			HandleMsg:     request.HandledMsg,
			ReqTime:       request.ReqTime.UnixMilli(),
			HandleUserID:  request.HandleUserID,
			HandleTime:    handled,
			Ex:            request.Ex,
			JoinSource:    request.JoinSource,
			InviterUserID: request.InviterUserID,
		},
	}
	seen := map[string]struct{}{}
	for _, manager := range managers {
		if manager.UserID == "" {
			continue
		}
		if _, ok := seen[manager.UserID]; ok {
			continue
		}
		seen[manager.UserID] = struct{}{}
		tips.ReceiverAs = receiverAdmin
		if manager.UserID == request.UserID {
			tips.ReceiverAs = receiverApplicant
		}
		s.sendUserNotice(ctx, op, manager.UserID, contentType, tips)
	}
	if _, ok := seen[request.UserID]; !ok && request.UserID != "" {
		tips.ReceiverAs = receiverApplicant
		s.sendUserNotice(ctx, op, request.UserID, contentType, tips)
	}
}

func (s *Server) notifyAgreedEnter(ctx context.Context, op string, group *model.Group, opMember *model.Member, member *model.Member) {
	if member.InviterUserID == "" {
		s.notifyEntered(ctx, op, group, member)
		return
	}
	inviter := &model.Member{GroupID: group.GroupID, UserID: member.InviterUserID}
	if member.InviterUserID == opMember.UserID {
		inviter = opMember
	} else if found, err := s.store.TakeMember(ctx, group.GroupID, member.InviterUserID); err == nil {
		inviter = found
	}
	versionID, version := s.memberHead(ctx, group.GroupID)
	s.sendNotice(ctx, op, group, contentMemberInvited, memberInvitedTips{
		Group:                groupNotice(group),
		OpUser:               memberNotice(opMember),
		InvitedUserList:      []noticeMember{memberNotice(member)},
		OperationTime:        s.now().UnixMilli(),
		GroupMemberVersion:   version,
		GroupMemberVersionID: versionID,
		InviterUser:          memberNotice(inviter),
	})
}

func (s *Server) notifyMemberInfo(ctx context.Context, op string, group *model.Group, opMember, changed *model.Member, role, profile bool) {
	if !role && !profile {
		return
	}
	versionID, version := s.memberHead(ctx, group.GroupID)
	var sortVersion uint64
	if role {
		sortVersion = version
	}
	tips := memberInfoTips{
		Group:                groupNotice(group),
		OpUser:               memberNotice(opMember),
		OperationTime:        s.now().UnixMilli(),
		ChangedUser:          memberNotice(changed),
		GroupMemberVersion:   version,
		GroupMemberVersionID: versionID,
		GroupSortVersion:     sortVersion,
	}
	if role {
		contentType := int32(contentMemberSetOrdinary)
		if changed.RoleLevel == model.RoleAdmin {
			contentType = contentMemberSetAdmin
		}
		s.sendNotice(ctx, op, group, contentType, tips)
	}
	if profile {
		s.sendNotice(ctx, op, group, contentMemberInfoSet, tips)
	}
}

func (s *Server) notifyEntered(ctx context.Context, op string, group *model.Group, member *model.Member) {
	versionID, version := s.memberHead(ctx, group.GroupID)
	s.sendNotice(ctx, op, group, contentMemberEnter, memberEnterTips{
		Group:                groupNotice(group),
		EntrantUser:          memberNotice(member),
		OperationTime:        s.now().UnixMilli(),
		GroupMemberVersion:   version,
		GroupMemberVersionID: versionID,
	})
}

func (s *Server) notifyJoinApplication(ctx context.Context, op string, group *model.Group, applicant model.UserProfile, request *model.Request) {
	if s.sender == nil || group == nil || request == nil {
		return
	}
	managers, err := s.store.Managers(ctx, group.GroupID)
	if err != nil {
		slog.Warn("group managers", "err", err, "groupID", group.GroupID)
		return
	}
	user := noticePublicUser{UserID: applicant.UserID, Nickname: applicant.Nickname, FaceURL: applicant.AvatarURL}
	tips := joinApplicationTips{
		Group:     groupNotice(group),
		Applicant: user,
		ReqMsg:    request.ReqMsg,
		UUID:      newNoticeID(),
		Request: noticeGroupRequest{
			UserInfo:      user,
			GroupInfo:     groupNotice(group),
			HandleResult:  model.HandlePending,
			ReqMsg:        request.ReqMsg,
			ReqTime:       request.ReqTime.UnixMilli(),
			Ex:            request.Ex,
			JoinSource:    request.JoinSource,
			InviterUserID: request.InviterUserID,
		},
	}
	seen := map[string]struct{}{op: {}}
	s.sendUserNotice(ctx, op, op, contentJoinGroupApplication, tips)
	for _, manager := range managers {
		if manager.UserID == "" {
			continue
		}
		if _, ok := seen[manager.UserID]; ok {
			continue
		}
		seen[manager.UserID] = struct{}{}
		s.sendUserNotice(ctx, op, manager.UserID, contentJoinGroupApplication, tips)
	}
	if _, ok := seen[request.UserID]; !ok && request.UserID != "" {
		s.sendUserNotice(ctx, op, request.UserID, contentJoinGroupApplication, tips)
	}
}

func (s *Server) notifyDismissed(ctx context.Context, op string, group *model.Group, opMember *model.Member) {
	versionID, version := s.memberHead(ctx, group.GroupID)
	s.sendNotice(ctx, op, group, contentGroupDismissed, groupDismissedTips{
		Group:                groupNotice(group),
		OpUser:               memberNotice(opMember),
		OperationTime:        s.now().UnixMilli(),
		GroupMemberVersion:   version,
		GroupMemberVersionID: versionID,
	})
}

func (s *Server) sendUserNotice(ctx context.Context, sendID, recvID string, contentType int32, detail any) {
	if s.sender == nil || sendID == "" || recvID == "" {
		return
	}
	msg, err := userNotificationMsg(sendID, recvID, contentType, detail, s.now().UnixMilli())
	if err != nil {
		slog.Warn("group notice", "err", err, "sendID", sendID, "recvID", recvID, "contentType", contentType)
		return
	}
	if err := s.sender.SendNotification(ctx, msg); err != nil {
		slog.Warn("group notice", "err", err, "sendID", sendID, "recvID", recvID, "contentType", contentType)
	}
}

func userNotificationMsg(sendID, recvID string, contentType int32, detail any, now int64) (*msgpb.MsgData, error) {
	body, err := json.Marshal(detail)
	if err != nil {
		return nil, err
	}
	content, err := json.Marshal(struct {
		Detail string `json:"detail"`
	}{Detail: string(body)})
	if err != nil {
		return nil, err
	}
	return &msgpb.MsgData{
		SendId:      sendID,
		RecvId:      recvID,
		SessionType: sessionSingle,
		MsgFrom:     msgFromSystem,
		ContentType: contentType,
		Content:     content,
		SendTime:    now,
		CreateTime:  now,
		Options:     map[string]bool{optNotNotification: false},
	}, nil
}

func newNoticeID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(buf[:])
}

func notificationMsg(op string, group *model.Group, contentType int32, detail any, now int64) (*msgpb.MsgData, error) {
	body, err := json.Marshal(detail)
	if err != nil {
		return nil, err
	}
	content, err := json.Marshal(struct {
		Detail string `json:"detail"`
	}{Detail: string(body)})
	if err != nil {
		return nil, err
	}
	session := int32(sessionSuper)
	if group.GroupType == model.GroupTypeNormal {
		session = sessionNormal
	}
	return &msgpb.MsgData{
		SendId:      op,
		RecvId:      group.GroupID,
		GroupId:     group.GroupID,
		SessionType: session,
		MsgFrom:     msgFromSystem,
		ContentType: contentType,
		Content:     content,
		SendTime:    now,
		CreateTime:  now,
		Options:     map[string]bool{optNotNotification: false},
	}, nil
}

type noticeGroup struct {
	GroupID                string `json:"groupID"`
	GroupName              string `json:"groupName"`
	Notification           string `json:"notification"`
	Introduction           string `json:"introduction"`
	FaceURL                string `json:"faceURL"`
	OwnerUserID            string `json:"ownerUserID"`
	CreateTime             int64  `json:"createTime"`
	MemberCount            int32  `json:"memberCount"`
	Status                 int32  `json:"status"`
	CreatorUserID          string `json:"creatorUserID"`
	GroupType              int32  `json:"groupType"`
	NeedVerification       int32  `json:"needVerification"`
	LookMemberInfo         int32  `json:"lookMemberInfo"`
	ApplyMemberFriend      int32  `json:"applyMemberFriend"`
	NotificationUpdateTime int64  `json:"notificationUpdateTime"`
	NotificationUserID     string `json:"notificationUserID"`
	Ex                     string `json:"ex"`
}

type noticeMember struct {
	GroupID        string `json:"groupID"`
	UserID         string `json:"userID"`
	Nickname       string `json:"nickname"`
	FaceURL        string `json:"faceURL"`
	RoleLevel      int32  `json:"roleLevel"`
	JoinTime       int64  `json:"joinTime"`
	JoinSource     int32  `json:"joinSource"`
	InviterUserID  string `json:"inviterUserID"`
	OperatorUserID string `json:"operatorUserID"`
	MuteEndTime    int64  `json:"muteEndTime"`
	Ex             string `json:"ex"`
}

type groupCreatedTips struct {
	Group                noticeGroup    `json:"group"`
	OpUser               noticeMember   `json:"opUser"`
	MemberList           []noticeMember `json:"memberList"`
	OperationTime        int64          `json:"operationTime"`
	GroupOwnerUser       noticeMember   `json:"groupOwnerUser"`
	GroupMemberVersion   uint64         `json:"groupMemberVersion"`
	GroupMemberVersionID string         `json:"groupMemberVersionID"`
}

type memberInvitedTips struct {
	Group                noticeGroup    `json:"group"`
	OpUser               noticeMember   `json:"opUser"`
	InvitedUserList      []noticeMember `json:"invitedUserList"`
	OperationTime        int64          `json:"operationTime"`
	GroupMemberVersion   uint64         `json:"groupMemberVersion"`
	GroupMemberVersionID string         `json:"groupMemberVersionID"`
	InviterUser          noticeMember   `json:"inviterUser"`
}

type memberQuitTips struct {
	Group                noticeGroup  `json:"group"`
	QuitUser             noticeMember `json:"quitUser"`
	OperationTime        int64        `json:"operationTime"`
	GroupMemberVersion   uint64       `json:"groupMemberVersion"`
	GroupMemberVersionID string       `json:"groupMemberVersionID"`
}

type memberKickedTips struct {
	Group                noticeGroup    `json:"group"`
	OpUser               noticeMember   `json:"opUser"`
	KickedUserList       []noticeMember `json:"kickedUserList"`
	OperationTime        int64          `json:"operationTime"`
	GroupMemberVersion   uint64         `json:"groupMemberVersion"`
	GroupMemberVersionID string         `json:"groupMemberVersionID"`
}

type ownerTransferredTips struct {
	Group                noticeGroup  `json:"group"`
	OpUser               noticeMember `json:"opUser"`
	NewGroupOwner        noticeMember `json:"newGroupOwner"`
	OldGroupOwner        string       `json:"oldGroupOwner"`
	OperationTime        int64        `json:"operationTime"`
	OldGroupOwnerInfo    noticeMember `json:"oldGroupOwnerInfo"`
	GroupMemberVersion   uint64       `json:"groupMemberVersion"`
	GroupMemberVersionID string       `json:"groupMemberVersionID"`
}

type memberInfoTips struct {
	Group                noticeGroup  `json:"group"`
	OpUser               noticeMember `json:"opUser"`
	OperationTime        int64        `json:"operationTime"`
	ChangedUser          noticeMember `json:"changedUser"`
	GroupMemberVersion   uint64       `json:"groupMemberVersion"`
	GroupMemberVersionID string       `json:"groupMemberVersionID"`
	GroupSortVersion     uint64       `json:"groupSortVersion"`
}

type memberEnterTips struct {
	Group                noticeGroup  `json:"group"`
	EntrantUser          noticeMember `json:"entrantUser"`
	OperationTime        int64        `json:"operationTime"`
	GroupMemberVersion   uint64       `json:"groupMemberVersion"`
	GroupMemberVersionID string       `json:"groupMemberVersionID"`
}

type noticePublicUser struct {
	UserID   string `json:"userID"`
	Nickname string `json:"nickname"`
	FaceURL  string `json:"faceURL"`
}

type noticeGroupRequest struct {
	UserInfo      noticePublicUser `json:"userInfo"`
	GroupInfo     noticeGroup      `json:"groupInfo"`
	HandleResult  int32            `json:"handleResult"`
	ReqMsg        string           `json:"reqMsg"`
	HandleMsg     string           `json:"handleMsg"`
	ReqTime       int64            `json:"reqTime"`
	HandleUserID  string           `json:"handleUserID"`
	HandleTime    int64            `json:"handleTime"`
	Ex            string           `json:"ex"`
	JoinSource    int32            `json:"joinSource"`
	InviterUserID string           `json:"inviterUserID"`
}

type applicationHandledTips struct {
	Group      noticeGroup        `json:"group"`
	OpUser     noticeMember       `json:"opUser"`
	HandleMsg  string             `json:"handleMsg"`
	ReceiverAs int32              `json:"receiverAs"`
	UUID       string             `json:"uuid"`
	Request    noticeGroupRequest `json:"request"`
}

type joinApplicationTips struct {
	Group     noticeGroup        `json:"group"`
	Applicant noticePublicUser   `json:"applicant"`
	ReqMsg    string             `json:"reqMsg"`
	UUID      string             `json:"uuid"`
	Request   noticeGroupRequest `json:"request"`
}

type groupDismissedTips struct {
	Group                noticeGroup  `json:"group"`
	OpUser               noticeMember `json:"opUser"`
	OperationTime        int64        `json:"operationTime"`
	GroupMemberVersion   uint64       `json:"groupMemberVersion"`
	GroupMemberVersionID string       `json:"groupMemberVersionID"`
}

type memberMutedTips struct {
	Group                noticeGroup  `json:"group"`
	OpUser               noticeMember `json:"opUser"`
	OperationTime        int64        `json:"operationTime"`
	MutedUser            noticeMember `json:"mutedUser"`
	MutedSeconds         uint32       `json:"mutedSeconds"`
	GroupMemberVersion   uint64       `json:"groupMemberVersion"`
	GroupMemberVersionID string       `json:"groupMemberVersionID"`
}

type memberCancelMutedTips struct {
	Group                noticeGroup  `json:"group"`
	OpUser               noticeMember `json:"opUser"`
	OperationTime        int64        `json:"operationTime"`
	MutedUser            noticeMember `json:"mutedUser"`
	GroupMemberVersion   uint64       `json:"groupMemberVersion"`
	GroupMemberVersionID string       `json:"groupMemberVersionID"`
}

type groupMutedTips struct {
	Group                noticeGroup  `json:"group"`
	OpUser               noticeMember `json:"opUser"`
	OperationTime        int64        `json:"operationTime"`
	GroupMemberVersion   uint64       `json:"groupMemberVersion"`
	GroupMemberVersionID string       `json:"groupMemberVersionID"`
}

func (s *Server) notifyMemberMuted(ctx context.Context, op, groupID, userID string, seconds uint32) {
	s.notifyMuteMember(ctx, op, groupID, userID, contentMemberMuted, &seconds)
}

func (s *Server) notifyMemberCancelMuted(ctx context.Context, op, groupID, userID string) {
	s.notifyMuteMember(ctx, op, groupID, userID, contentMemberCancelMuted, nil)
}

func (s *Server) notifyMuteMember(ctx context.Context, op, groupID, userID string, contentType int32, seconds *uint32) {
	group, opMember, target := s.muteNotice(ctx, op, groupID, userID)
	if group == nil {
		return
	}
	versionID, version := s.memberHead(ctx, groupID)
	if seconds == nil {
		s.sendNotice(ctx, op, group, contentType, memberCancelMutedTips{
			Group:                groupNotice(group),
			OpUser:               memberNotice(opMember),
			MutedUser:            memberNotice(target),
			OperationTime:        s.now().UnixMilli(),
			GroupMemberVersion:   version,
			GroupMemberVersionID: versionID,
		})
		return
	}
	s.sendNotice(ctx, op, group, contentType, memberMutedTips{
		Group:                groupNotice(group),
		OpUser:               memberNotice(opMember),
		MutedUser:            memberNotice(target),
		MutedSeconds:         *seconds,
		OperationTime:        s.now().UnixMilli(),
		GroupMemberVersion:   version,
		GroupMemberVersionID: versionID,
	})
}

func (s *Server) notifyGroupMuted(ctx context.Context, op, groupID string, cancel bool) {
	group, opMember, _ := s.muteNotice(ctx, op, groupID, op)
	if group == nil {
		return
	}
	versionID, version := s.memberHead(ctx, groupID)
	contentType := int32(contentGroupMuted)
	if cancel {
		contentType = contentGroupCancelMuted
	}
	s.sendNotice(ctx, op, group, contentType, groupMutedTips{
		Group:                groupNotice(group),
		OpUser:               memberNotice(opMember),
		OperationTime:        s.now().UnixMilli(),
		GroupMemberVersion:   version,
		GroupMemberVersionID: versionID,
	})
}

func (s *Server) muteNotice(ctx context.Context, op, groupID, userID string) (*model.Group, *model.Member, *model.Member) {
	group, err := s.store.TakeGroup(ctx, groupID)
	if err != nil {
		slog.Warn("mute notice", "err", err, "groupID", groupID)
		return nil, nil, nil
	}
	members, err := s.store.FindMembers(ctx, groupID, []string{op, userID})
	if err != nil {
		slog.Warn("mute notice", "err", err, "groupID", groupID)
		return nil, nil, nil
	}
	var opMember, target *model.Member
	for _, member := range members {
		if member.UserID == op {
			opMember = member
		}
		if member.UserID == userID {
			target = member
		}
	}
	return group, opMember, target
}

func groupNotice(group *model.Group) noticeGroup {
	if group == nil {
		return noticeGroup{}
	}
	var updated int64
	if !group.NotificationUpdateTime.IsZero() {
		updated = group.NotificationUpdateTime.UnixMilli()
	}
	return noticeGroup{
		GroupID:                group.GroupID,
		GroupName:              group.GroupName,
		Notification:           group.Notification,
		Introduction:           group.Introduction,
		FaceURL:                group.FaceURL,
		OwnerUserID:            group.OwnerUserID,
		CreateTime:             group.CreateTime.UnixMilli(),
		MemberCount:            group.MemberCount,
		Status:                 group.Status,
		CreatorUserID:          group.CreatorUserID,
		GroupType:              group.GroupType,
		NeedVerification:       group.NeedVerification,
		LookMemberInfo:         group.LookMemberInfo,
		ApplyMemberFriend:      group.ApplyMemberFriend,
		NotificationUpdateTime: updated,
		NotificationUserID:     group.NotificationUserID,
		Ex:                     group.Ex,
	}
}

func memberNotice(member *model.Member) noticeMember {
	if member == nil {
		return noticeMember{}
	}
	var mute int64
	if !member.MuteEndTime.IsZero() {
		mute = member.MuteEndTime.UnixMilli()
	}
	return noticeMember{
		GroupID:        member.GroupID,
		UserID:         member.UserID,
		Nickname:       member.Nickname,
		FaceURL:        member.FaceURL,
		RoleLevel:      member.RoleLevel,
		JoinTime:       member.JoinTime.UnixMilli(),
		JoinSource:     member.JoinSource,
		InviterUserID:  member.InviterUserID,
		OperatorUserID: member.OperatorUserID,
		MuteEndTime:    mute,
		Ex:             member.Ex,
	}
}
