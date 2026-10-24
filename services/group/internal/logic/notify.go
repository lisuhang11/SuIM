package logic

import (
	"context"
	"encoding/json"
	"log/slog"

	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

const (
	msgFromSystem = 200

	sessionNormal = 2
	sessionSuper  = 3

	contentGroupCreated     = 1501
	contentMemberQuit       = 1504
	contentOwnerTransferred = 1507
	contentMemberKicked     = 1508
	contentMemberInvited    = 1509
	contentGroupDismissed   = 1511

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

type groupDismissedTips struct {
	Group                noticeGroup  `json:"group"`
	OpUser               noticeMember `json:"opUser"`
	OperationTime        int64        `json:"operationTime"`
	GroupMemberVersion   uint64       `json:"groupMemberVersion"`
	GroupMemberVersionID string       `json:"groupMemberVersionID"`
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
