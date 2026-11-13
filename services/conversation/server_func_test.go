package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	conversationpb "github.com/lisuhang11/SuIM/proto/conversation"
	"github.com/lisuhang11/SuIM/services/conversation/internal/logic"
	"github.com/lisuhang11/SuIM/services/conversation/internal/model"
	"github.com/lisuhang11/SuIM/services/conversation/internal/store"
)

func TestCreateSingleRejectsSameUsers(t *testing.T) {
	ctx, srv, a, b, _ := openConversation(t)
	if _, err := srv.CreateSingleChatConversations(ctx, &conversationpb.CreateSingleChatConversationsReq{
		SendId: a, RecvId: a, ConversationType: model.ConversationSingle,
	}); !isCode(err, codes.InvalidArgument, errorcode.ConversationInvalidArgs) {
		t.Fatalf("same users code=%v err=%v", status.Code(err), err)
	}
	_ = b
}

func TestCreateSingleGeneratesStableID(t *testing.T) {
	ctx, srv, a, b, st := openConversation(t)
	// Lexicographic order must not depend on send/recv argument order.
	high, low := "z"+a[1:], "a"+b[1:]
	if _, err := srv.CreateSingleChatConversations(ctx, &conversationpb.CreateSingleChatConversationsReq{
		SendId: high, RecvId: low, ConversationType: model.ConversationSingle,
	}); err != nil {
		t.Fatal(err)
	}
	want := "si_" + low + "_" + high
	rows, err := st.ConversationsByID(ctx, low, []string{want})
	if err != nil || len(rows) != 1 || rows[0].UserID != high {
		t.Fatalf("generated id rows=%+v err=%v want=%s", rows, err, want)
	}
	peer, err := st.ConversationsByID(ctx, high, []string{want})
	if err != nil || len(peer) != 1 || peer[0].UserID != low {
		t.Fatalf("peer row %+v err=%v", peer, err)
	}
}

func TestSetConversationsRejectsRepeatedOwner(t *testing.T) {
	ctx, srv, a, b, _ := openConversation(t)
	if _, err := srv.SetConversations(ctx, &conversationpb.SetConversationsReq{
		UserIds: []string{a, a},
		Conversation: &conversationpb.ConversationReq{
			ConversationId: "si_" + a + "_" + b, ConversationType: model.ConversationSingle, UserId: b, IsPinned: boolPtr(true),
		},
	}); !isCode(err, codes.InvalidArgument, errorcode.ConversationInvalidArgs) {
		t.Fatalf("repeated owner code=%v err=%v", status.Code(err), err)
	}
}

func TestSetConversationsRejectsLongEx(t *testing.T) {
	ctx, srv, a, b, _ := openConversation(t)
	long := strings.Repeat("字", 1025)
	if utf8.RuneCountInString(long) != 1025 {
		t.Fatal("fixture")
	}
	if _, err := srv.SetConversations(ctx, &conversationpb.SetConversationsReq{
		UserIds: []string{a},
		Conversation: &conversationpb.ConversationReq{
			ConversationId: "si_" + a + "_" + b, ConversationType: model.ConversationSingle, UserId: b, Ex: &long,
		},
	}); !isCode(err, codes.InvalidArgument, errorcode.ConversationInvalidArgs) {
		t.Fatalf("long ex code=%v err=%v", status.Code(err), err)
	}
	ok := strings.Repeat("字", 1024)
	if _, err := srv.SetConversations(ctx, &conversationpb.SetConversationsReq{
		UserIds: []string{a},
		Conversation: &conversationpb.ConversationReq{
			ConversationId: "si_" + a + "_" + b, ConversationType: model.ConversationSingle, UserId: b, Ex: &ok,
		},
	}); err != nil {
		t.Fatalf("1024 runes: %v", err)
	}
}

// TestMsgDestructClearRaisesMinSeq: due destruct rows raise the visible floor from history
// and stamp latest_msg_destruct_time so they are not immediately due again.
func TestMsgDestructClearRaisesMinSeq(t *testing.T) {
	ctx, srv, a, b, st := openConversation(t)
	seq := &recordSeq{}
	hist := &stubHistory{seq: 10}
	srv.UseSeq(seq)
	srv.UseHistory(hist)

	conv := fmt.Sprintf("si_dest_%d", time.Now().UnixNano())
	destruct := true
	ttl := int64(3600)
	if _, err := srv.SetConversations(ctx, &conversationpb.SetConversationsReq{
		UserIds: []string{a},
		Conversation: &conversationpb.ConversationReq{
			ConversationId: conv, ConversationType: model.ConversationSingle, UserId: b,
			IsMsgDestruct: &destruct, MsgDestructTime: &ttl,
		},
	}); err != nil {
		t.Fatal(err)
	}
	due, err := srv.GetConversationsNeedClearMsg(ctx, &conversationpb.GetConversationsNeedClearMsgReq{})
	if err != nil {
		t.Fatal(err)
	}
	if !hasConversation(due.GetConversations(), a, conv) {
		t.Fatalf("due list missing %s/%s among %d rows", a, conv, len(due.GetConversations()))
	}

	cleared, err := srv.ClearUserConversationMsg(ctx, &conversationpb.ClearUserConversationMsgReq{Timestamp: time.Now().UnixMilli(), Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.GetCount() < 1 {
		t.Fatalf("clear count=%d", cleared.GetCount())
	}
	if seq.minCalls < 1 || seq.lastSeq != 11 {
		t.Fatalf("min seq calls=%d last=%d", seq.minCalls, seq.lastSeq)
	}
	rows, err := st.ConversationsByID(ctx, a, []string{conv})
	if err != nil || len(rows) != 1 || rows[0].MinSeq != 11 || rows[0].LatestMsgDestructTime.IsZero() {
		t.Fatalf("after clear %+v err=%v", rows, err)
	}
	due, err = srv.GetConversationsNeedClearMsg(ctx, &conversationpb.GetConversationsNeedClearMsgReq{})
	if err != nil {
		t.Fatal(err)
	}
	if hasConversation(due.GetConversations(), a, conv) {
		t.Fatalf("still due after clear among %d rows", len(due.GetConversations()))
	}
}

type stubHistory struct{ seq int64 }

func (s *stubHistory) LastSeqByTime(context.Context, string, int64) (int64, error) {
	return s.seq, nil
}

func hasConversation(rows []*conversationpb.Conversation, owner, id string) bool {
	for _, row := range rows {
		if row.GetOwnerUserId() == owner && row.GetConversationId() == id {
			return true
		}
	}
	return false
}

func openConversation(t *testing.T) (context.Context, *logic.Server, string, string, *store.Store) {
	t.Helper()
	ctx := context.Background()
	env := readEnvFile(t, filepath.Join("..", "..", ".env"))
	cfg := config.MySQL{
		Address: []string{"127.0.0.1:3306"}, Username: env["MYSQL_USER"], Password: env["MYSQL_PASSWORD"],
		Database: env["MYSQL_DATABASE"], MaxOpenConn: 8, MaxIdleConn: 4,
	}
	st, err := store.Open(ctx, mysqlDSN(cfg), cfg.MaxOpenConn, cfg.MaxIdleConn)
	if err != nil {
		t.Skipf("mysql is not reachable: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	script, err := os.ReadFile(filepath.Join("..", "..", "migrations", "conversation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx, string(script)); err != nil {
		t.Fatal(err)
	}
	suffix := time.Now().UnixNano()
	a := fmt.Sprintf("cfa-%d", suffix)
	b := fmt.Sprintf("cfb-%d", suffix)
	t.Cleanup(func() { cleanupConversation(t, cfg, a, b) })
	// CreateSingleGeneratesStableID uses derived IDs; clean those too when present.
	t.Cleanup(func() {
		cleanupConversation(t, cfg, "z"+a[1:], "a"+b[1:])
	})
	return ctx, logic.New(st, &captureSender{}), a, b, st
}
