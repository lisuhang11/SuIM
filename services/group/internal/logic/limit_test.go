package logic

import (
	"context"
	"strconv"
	"testing"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
	"github.com/lisuhang11/SuIM/services/group/internal/store"
)

type rejectQuota struct{}

func (rejectQuota) Reserve(context.Context, []store.Limit) error {
	return store.ErrLimited
}

type countingQuota struct {
	calls int
	added int64
}

func (q *countingQuota) Reserve(_ context.Context, limits []store.Limit) error {
	q.calls++
	if len(limits) > 0 {
		q.added += limits[0].N
	}
	return store.ErrLimited
}

func TestCreateAndInviteRejectOversizedBatches(t *testing.T) {
	s := New(nil, nil, nil)
	quota := &countingQuota{}
	s.UseQuota(quota)

	members := make([]string, maxCreateMembers+1)
	for i := range members {
		members[i] = "u" + strconv.Itoa(i)
	}
	_, err := s.CreateGroup(context.Background(), &grouppb.CreateGroupReq{
		CreatorUserId: "owner",
		GroupName:     "room",
		GroupType:     model.GroupTypeNormal,
		MemberIds:     members,
	})
	if code, _ := errorcode.CodeOf(err); code != errorcode.GroupInvalidArgs || quota.calls != 0 {
		t.Fatalf("create code=%v calls=%d err=%v", code, quota.calls, err)
	}

	invited := make([]string, maxInviteUsers+1)
	for i := range invited {
		invited[i] = "v" + strconv.Itoa(i)
	}
	_, err = s.InviteUserToGroup(context.Background(), &grouppb.InviteUserToGroupReq{
		GroupId:  "g1",
		OpUserId: "owner",
		UserIds:  invited,
	})
	if code, _ := errorcode.CodeOf(err); code != errorcode.GroupInvalidArgs || quota.calls != 0 {
		t.Fatalf("invite code=%v calls=%d err=%v", code, quota.calls, err)
	}
}

func TestJoinCountsBeforeUserLookup(t *testing.T) {
	s := New(nil, nil, nil)
	s.UseQuota(rejectQuota{})
	_, err := s.JoinGroup(context.Background(), &grouppb.JoinGroupReq{
		GroupId:    "g1",
		UserId:     "u1",
		JoinSource: model.JoinBySearch,
	})
	if code, _ := errorcode.CodeOf(err); code != errorcode.GroupTooFrequent {
		t.Fatalf("code=%v err=%v", code, err)
	}
}

func TestInviteCountsPeople(t *testing.T) {
	s := New(nil, nil, nil)
	quota := &countingQuota{}
	s.UseQuota(quota)
	_, err := s.InviteUserToGroup(context.Background(), &grouppb.InviteUserToGroupReq{
		GroupId:  "g1",
		OpUserId: "owner",
		UserIds:  []string{"a", "b", "c"},
	})
	if code, _ := errorcode.CodeOf(err); code != errorcode.GroupTooFrequent || quota.calls != 1 || quota.added != 3 {
		t.Fatalf("code=%v calls=%d added=%d err=%v", code, quota.calls, quota.added, err)
	}
}
