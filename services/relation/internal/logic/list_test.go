package logic

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/relationpb"
)

func TestFriendListRejectsBadPage(t *testing.T) {
	srv := New(nil, nil, Hooks{})
	_, err := srv.GetFriendList(context.Background(), &relationpb.GetFriendListReq{UserId: "u", Limit: 201})
	got, ok := errorcode.CodeOf(err)
	if !ok || got != errorcode.RelationInvalidPage || status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code=%v err=%v", status.Code(err), err)
	}
	_, err = srv.GetBlackList(context.Background(), &relationpb.GetBlackListReq{UserId: "u", Offset: -1})
	got, ok = errorcode.CodeOf(err)
	if !ok || got != errorcode.RelationInvalidPage {
		t.Fatalf("black page err=%v", err)
	}
}
