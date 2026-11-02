package logic

import (
	"context"
	"testing"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
)

func TestActorRejectsADifferentBodyUser(t *testing.T) {
	s := New(nil, nil, nil)
	_, err := s.DismissGroup(rpcctx.WithUser(context.Background(), "owner"), &grouppb.DismissGroupReq{
		GroupId:  "1",
		OpUserId: "other",
	})
	if code, _ := errorcode.CodeOf(err); code != errorcode.NoPermission {
		t.Fatalf("code=%v err=%v", code, err)
	}
}

func TestActorFillsAnEmptyBodyUser(t *testing.T) {
	id, err := bindActor(rpcctx.WithUser(context.Background(), "owner"), "")
	if err != nil || id != "owner" {
		t.Fatalf("id=%q err=%v", id, err)
	}
	id, err = bindActor(context.Background(), "owner")
	if err != nil || id != "owner" {
		t.Fatalf("id=%q err=%v", id, err)
	}
}
