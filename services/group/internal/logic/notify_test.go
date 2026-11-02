package logic

import (
	"encoding/json"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

func TestGroupInfoOptions(t *testing.T) {
	chat := groupInfoOptions(true)
	if !chat[optSendMsg] || !chat[optUnreadCount] || chat[optHistory] || chat[optOfflinePush] || chat[optNotNotification] {
		t.Fatalf("chat options = %+v", chat)
	}
	push := groupInfoOptions(false)
	if push[optSendMsg] || push[optUnreadCount] || push[optHistory] || push[optOfflinePush] || push[optNotNotification] {
		t.Fatalf("push options = %+v", push)
	}
	group := &model.Group{GroupID: "g1", GroupName: "新群名", GroupType: model.GroupTypeNormal, Notification: "公告"}
	msg, err := notificationMsg("owner", group, contentGroupName, groupInfoNameTips{
		OpUser:               noticeMember{UserID: "owner", Nickname: "群主"},
		Group:                groupNotice(group),
		GroupMemberVersion:   3,
		GroupMemberVersionID: "ver",
	}, 10)
	if err != nil {
		t.Fatal(err)
	}
	msg.Options = groupInfoOptions(true)
	if msg.ContentType != contentGroupName || msg.SessionType != sessionNormal || msg.GroupId != "g1" {
		t.Fatalf("msg = %+v", msg)
	}
	var elem struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(msg.Content, &elem); err != nil {
		t.Fatal(err)
	}
	var tips groupInfoNameTips
	if err := json.Unmarshal([]byte(elem.Detail), &tips); err != nil {
		t.Fatal(err)
	}
	if tips.Group.GroupName != "新群名" || tips.OpUser.Nickname != "群主" || tips.GroupMemberVersion != 3 {
		t.Fatalf("tips = %+v", tips)
	}
}

func TestUserNotificationIsSingle(t *testing.T) {
	msg, err := userNotificationMsg("u1", "owner", contentJoinGroupApplication, joinApplicationTips{
		ReqMsg:    "hi",
		UUID:      "abc",
		Applicant: noticePublicUser{UserID: "u1"},
	}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if msg.SessionType != sessionSingle || msg.SendId != "u1" || msg.RecvId != "owner" || msg.GroupId != "" || msg.ContentType != contentJoinGroupApplication {
		t.Fatalf("msg = %+v", msg)
	}
	if msg.Options["isNotNotification"] {
		t.Fatal("notification option must be false")
	}
}

func TestNotificationMsgCarriesMemberVersion(t *testing.T) {
	group := &model.Group{GroupID: "g1", GroupName: "test群", GroupType: model.GroupTypeSuper, OwnerUserID: "owner"}
	msg, err := notificationMsg("owner", group, contentMemberInvited, memberInvitedTips{
		Group:                groupNotice(group),
		InvitedUserList:      []noticeMember{{GroupID: "g1", UserID: "u2", RoleLevel: model.RoleOrdinary}},
		GroupMemberVersion:   4,
		GroupMemberVersionID: "ver",
	}, time.UnixMilli(10).UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	if msg.SessionType != sessionSuper || msg.GroupId != "g1" || msg.RecvId != "g1" || msg.ContentType != contentMemberInvited {
		t.Fatalf("msg = %+v", msg)
	}
	if msg.Options["isNotNotification"] {
		t.Fatal("notification option must be false")
	}
	var elem struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(msg.Content, &elem); err != nil {
		t.Fatal(err)
	}
	var tips memberInvitedTips
	if err := json.Unmarshal([]byte(elem.Detail), &tips); err != nil {
		t.Fatal(err)
	}
	if tips.GroupMemberVersion != 4 || tips.GroupMemberVersionID != "ver" || tips.Group.GroupID != "g1" || tips.InvitedUserList[0].UserID != "u2" {
		t.Fatalf("tips = %+v", tips)
	}
	body, err := proto.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var back msgpb.MsgData
	if err := proto.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	value, ok := back.Options["isNotNotification"]
	if !ok || value {
		t.Fatalf("options = %+v", back.Options)
	}
}
