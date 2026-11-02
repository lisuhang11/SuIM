package rpcctx

import (
	"context"
	"testing"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

func TestActorUsesContextUser(t *testing.T) {
	ctx := WithUser(context.Background(), "a")
	id, err := Actor(ctx, "")
	if err != nil || id != "a" {
		t.Fatalf("id=%q err=%v", id, err)
	}
	id, err = Actor(ctx, "a")
	if err != nil || id != "a" {
		t.Fatalf("id=%q err=%v", id, err)
	}
	if _, err = Actor(ctx, "b"); err == nil || !errorcode.ErrNoPermission.Is(err) {
		t.Fatalf("err=%v", err)
	}
}

func TestOneOfAcceptsEitherUser(t *testing.T) {
	ctx := WithUser(context.Background(), "b")
	if err := OneOf(ctx, "a", "b"); err != nil {
		t.Fatal(err)
	}
	if err := OneOf(ctx, "a", "c"); err == nil {
		t.Fatal("expected no permission")
	}
	if err := OneOf(context.Background(), "a", "c"); err != nil {
		t.Fatal(err)
	}
}
