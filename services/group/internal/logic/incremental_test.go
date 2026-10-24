package logic

import (
	"context"
	"testing"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/group"
)

func TestBatchRejectsTooManyGroups(t *testing.T) {
	srv := New(nil, nil, nil)
	groups := make([]*grouppb.GetIncrementalGroupMembersReq, batchMemberLimit+1)
	for i := range groups {
		groups[i] = &grouppb.GetIncrementalGroupMembersReq{GroupId: "g"}
	}
	_, err := srv.BatchGetIncrementalGroupMembers(context.Background(), &grouppb.BatchGetIncrementalGroupMembersReq{
		UserId: "u",
		Groups: groups,
	})
	got, ok := errorcode.CodeOf(err)
	if !ok || got != errorcode.GroupInvalidArgs {
		t.Fatalf("err=%v", err)
	}
}
