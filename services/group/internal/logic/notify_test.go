package logic

import (
	"encoding/json"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

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
