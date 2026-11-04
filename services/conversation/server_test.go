package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/idhash"
	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/conversation/internal/logic"
	"github.com/lisuhang11/SuIM/services/conversation/internal/model"
	"github.com/lisuhang11/SuIM/services/conversation/internal/store"
)

func TestConversationSync(t *testing.T) {
	ctx := context.Background()
	env := readEnvFile(t, filepath.Join("..", "..", ".env"))
	mysqlCfg := config.MySQL{
		Address:     []string{"127.0.0.1:3306"},
		Username:    env["MYSQL_USER"],
		Password:    env["MYSQL_PASSWORD"],
		Database:    env["MYSQL_DATABASE"],
		MaxOpenConn: 5,
		MaxIdleConn: 2,
	}
	st, err := store.Open(ctx, mysqlDSN(mysqlCfg), mysqlCfg.MaxOpenConn, mysqlCfg.MaxIdleConn)
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
	a := fmt.Sprintf("ca-%d", suffix)
	b := fmt.Sprintf("cb-%d", suffix)
	t.Cleanup(func() { cleanupConversation(t, mysqlCfg, a, b) })
	notices := &captureSender{}
	srv := logic.New(st, notices)

	single := fmt.Sprintf("si_%s_%s", a, b)
	groupConv := fmt.Sprintf("g_%d", suffix)
	if _, err := srv.SetConversations(ctx, &conversationpb.SetConversationsReq{}); !isCode(err, codes.InvalidArgument, errorcode.ConversationInvalidArgs) {
		t.Fatalf("empty users code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.SetConversations(ctx, &conversationpb.SetConversationsReq{
		UserIds: []string{a},
		Conversation: &conversationpb.ConversationReq{
			ConversationId: single, ConversationType: model.ConversationSingle, UserId: b, RecvMsgOpt: i32Ptr(9),
		},
	}); !isCode(err, codes.InvalidArgument, errorcode.ConversationInvalidArgs) {
		t.Fatalf("bad recv opt code=%v err=%v", status.Code(err), err)
	}

	minSeq := int64(99)
	if _, err := srv.SetConversations(ctx, &conversationpb.SetConversationsReq{
		UserIds: []string{a},
		Conversation: &conversationpb.ConversationReq{
			ConversationId: single, ConversationType: model.ConversationSingle, UserId: b,
			IsPinned: boolPtr(true), MinSeq: &minSeq,
		},
	}); err != nil {
		t.Fatal(err)
	}
	full, err := srv.GetFullOwnerConversationIDs(ctx, &conversationpb.GetFullOwnerConversationIDsReq{UserId: a})
	if err != nil || full.GetEqual() || len(full.GetConversationIds()) != 1 || full.GetConversationIds()[0] != single || full.GetVersion() == 0 {
		t.Fatalf("full %+v err=%v", full, err)
	}
	again, err := srv.GetFullOwnerConversationIDs(ctx, &conversationpb.GetFullOwnerConversationIDsReq{UserId: a, IdHash: idhash.Hash(full.GetConversationIds())})
	if err != nil || !again.GetEqual() || again.GetConversationIds() != nil || again.GetVersionId() != full.GetVersionId() {
		t.Fatalf("equal full %+v err=%v", again, err)
	}
	rows, err := st.ConversationsByID(ctx, a, []string{single})
	if err != nil || len(rows) != 1 || rows[0].MinSeq != 0 || !rows[0].IsPinned {
		t.Fatalf("created row %+v err=%v", rows, err)
	}
	base, err := srv.GetIncrementalConversation(ctx, &conversationpb.GetIncrementalConversationReq{UserId: a})
	if err != nil || !base.GetFull() || base.GetVersionId() != full.GetVersionId() {
		t.Fatalf("baseline %+v err=%v", base, err)
	}
	caught, err := srv.GetIncrementalConversation(ctx, &conversationpb.GetIncrementalConversationReq{UserId: a, VersionId: full.GetVersionId(), Version: full.GetVersion()})
	if err != nil || caught.GetFull() || len(caught.GetInsert()) != 0 || len(caught.GetUpdate()) != 0 {
		t.Fatalf("caught up %+v err=%v", caught, err)
	}
	if _, err := srv.SetConversations(ctx, &conversationpb.SetConversationsReq{
		UserIds: []string{a},
		Conversation: &conversationpb.ConversationReq{
			ConversationId: single, ConversationType: model.ConversationSingle, UserId: b, IsPinned: boolPtr(true),
		},
	}); err != nil {
		t.Fatal(err)
	}
	same, err := srv.GetIncrementalConversation(ctx, &conversationpb.GetIncrementalConversationReq{UserId: a, VersionId: caught.GetVersionId(), Version: caught.GetVersion()})
	if err != nil || same.GetFull() || len(same.GetUpdate()) != 0 || len(same.GetInsert()) != 0 {
		t.Fatalf("unchanged delta %+v err=%v", same, err)
	}
	if _, err := srv.SetConversations(ctx, &conversationpb.SetConversationsReq{
		UserIds: []string{a},
		Conversation: &conversationpb.ConversationReq{
			ConversationId: single, ConversationType: model.ConversationSingle, UserId: b, Ex: strPtr("note"),
		},
	}); err != nil {
		t.Fatal(err)
	}
	updated, err := srv.GetIncrementalConversation(ctx, &conversationpb.GetIncrementalConversationReq{UserId: a, VersionId: same.GetVersionId(), Version: same.GetVersion()})
	if err != nil || updated.GetFull() || len(updated.GetUpdate()) != 1 || updated.GetUpdate()[0].GetEx() != "note" || !updated.GetUpdate()[0].GetIsPinned() {
		t.Fatalf("update delta %+v err=%v", updated, err)
	}

	if _, err := srv.SetConversations(ctx, &conversationpb.SetConversationsReq{
		UserIds: []string{a},
		Conversation: &conversationpb.ConversationReq{
			ConversationId: single, ConversationType: model.ConversationSingle, UserId: b, IsPrivateChat: boolPtr(true),
		},
	}); err != nil {
		t.Fatal(err)
	}
	peerIDs, err := srv.GetFullOwnerConversationIDs(ctx, &conversationpb.GetFullOwnerConversationIDsReq{UserId: b})
	if err != nil || len(peerIDs.GetConversationIds()) != 1 || peerIDs.GetConversationIds()[0] != single {
		t.Fatalf("peer ids %+v err=%v", peerIDs, err)
	}
	peerDelta, err := srv.GetIncrementalConversation(ctx, &conversationpb.GetIncrementalConversationReq{UserId: b, VersionId: peerIDs.GetVersionId(), Version: peerIDs.GetVersion()})
	if err != nil || peerDelta.GetFull() {
		t.Fatalf("peer head %+v err=%v", peerDelta, err)
	}
	peerRows, err := st.ConversationsByID(ctx, b, []string{single})
	if err != nil || len(peerRows) != 1 || !peerRows[0].IsPrivateChat || peerRows[0].UserID != a {
		t.Fatalf("peer row %+v err=%v", peerRows, err)
	}
	if !noticeHas(notices.msgs, 1701, `"isPrivate":true`) || !noticeHas(notices.msgs, 1300, `"userID":"`+a+`"`) {
		t.Fatalf("notices missing private on: %+v", noticeTypes(notices.msgs))
	}
	if _, err := srv.SetConversations(ctx, &conversationpb.SetConversationsReq{
		UserIds: []string{a},
		Conversation: &conversationpb.ConversationReq{
			ConversationId: single, ConversationType: model.ConversationSingle, UserId: b, IsPrivateChat: boolPtr(false),
		},
	}); err != nil {
		t.Fatal(err)
	}
	if !noticeHas(notices.msgs, 1701, `"isPrivate":false`) {
		t.Fatalf("private off notice missing: %+v", noticeTypes(notices.msgs))
	}

	if _, err := srv.SetConversations(ctx, &conversationpb.SetConversationsReq{
		UserIds: []string{a},
		Conversation: &conversationpb.ConversationReq{
			ConversationId: groupConv, ConversationType: model.ConversationGroup, GroupId: "g1", IsPrivateChat: boolPtr(true), IsPinned: boolPtr(true),
		},
	}); err != nil {
		t.Fatal(err)
	}
	groupRows, err := st.ConversationsByID(ctx, a, []string{groupConv})
	if err != nil || len(groupRows) != 1 || groupRows[0].IsPrivateChat || !groupRows[0].IsPinned {
		t.Fatalf("group row %+v err=%v", groupRows, err)
	}
	peerGroup, err := st.ConversationsByID(ctx, b, []string{groupConv})
	if err != nil || len(peerGroup) != 0 {
		t.Fatalf("group peer %+v err=%v", peerGroup, err)
	}
}

func TestConversationRowsStayInThisService(t *testing.T) {
	ctx := context.Background()
	env := readEnvFile(t, filepath.Join("..", "..", ".env"))
	mysqlCfg := config.MySQL{
		Address:     []string{"127.0.0.1:3306"},
		Username:    env["MYSQL_USER"],
		Password:    env["MYSQL_PASSWORD"],
		Database:    env["MYSQL_DATABASE"],
		MaxOpenConn: 5,
		MaxIdleConn: 2,
	}
	st, err := store.Open(ctx, mysqlDSN(mysqlCfg), mysqlCfg.MaxOpenConn, mysqlCfg.MaxIdleConn)
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
	a := fmt.Sprintf("cc-%d", suffix)
	b := fmt.Sprintf("cd-%d", suffix)
	t.Cleanup(func() { cleanupConversation(t, mysqlCfg, a, b) })
	seq := &recordSeq{}
	srv := logic.New(st, &captureSender{})
	srv.UseSeq(seq)

	single := fmt.Sprintf("si_rows_%d", suffix)
	if _, err := srv.CreateSingleChatConversations(ctx, &conversationpb.CreateSingleChatConversationsReq{
		SendId: a, RecvId: b, ConversationId: single, ConversationType: model.ConversationSingle,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.CreateSingleChatConversations(ctx, &conversationpb.CreateSingleChatConversationsReq{
		SendId: a, RecvId: b, ConversationId: single, ConversationType: model.ConversationSingle,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.SetConversations(ctx, &conversationpb.SetConversationsReq{
		UserIds: []string{a},
		Conversation: &conversationpb.ConversationReq{
			ConversationId: single, ConversationType: model.ConversationSingle, UserId: b, IsPinned: boolPtr(true),
		},
	}); err != nil {
		t.Fatal(err)
	}
	pinned, err := srv.GetPinnedConversationIDs(ctx, &conversationpb.GetPinnedConversationIDsReq{UserId: a})
	if err != nil || len(pinned.GetConversationIds()) != 1 || pinned.GetConversationIds()[0] != single {
		t.Fatalf("pinned %+v err=%v", pinned, err)
	}
	none, err := srv.GetPinnedConversationIDs(ctx, &conversationpb.GetPinnedConversationIDsReq{UserId: b})
	if err != nil || len(none.GetConversationIds()) != 0 {
		t.Fatalf("peer pinned %+v err=%v", none, err)
	}

	groupID := fmt.Sprintf("g%d", suffix%100000000)
	if _, err := srv.CreateGroupChatConversations(ctx, &conversationpb.CreateGroupChatConversationsReq{
		GroupId: groupID, UserIds: []string{a, b}, ConversationType: model.ConversationGroup,
	}); err != nil {
		t.Fatal(err)
	}
	if seq.maxCalls != 1 || seq.lastSeq != 0 || len(seq.lastUsers) != 2 {
		t.Fatalf("group seq %+v", seq)
	}
	groupConv := "g_" + groupID
	if _, err := srv.SetConversationMaxSeq(ctx, &conversationpb.SetConversationMaxSeqReq{
		ConversationId: groupConv, OwnerUserIds: []string{a}, MaxSeq: 8,
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := st.ConversationsByID(ctx, a, []string{groupConv})
	if err != nil || len(rows) != 1 || rows[0].MaxSeq != 8 || rows[0].GroupID != groupID {
		t.Fatalf("group row %+v err=%v", rows, err)
	}
	if _, err := srv.SetConversationMinSeq(ctx, &conversationpb.SetConversationMinSeqReq{
		ConversationId: groupConv, OwnerUserIds: []string{a}, MinSeq: 3,
	}); err != nil {
		t.Fatal(err)
	}
	rows, err = st.ConversationsByID(ctx, a, []string{groupConv})
	if err != nil || len(rows) != 1 || rows[0].MinSeq != 3 || rows[0].MaxSeq != 8 {
		t.Fatalf("seq row %+v err=%v", rows, err)
	}
	notice := fmt.Sprintf("n_rows_%d", suffix)
	if _, err := srv.CreateSingleChatConversations(ctx, &conversationpb.CreateSingleChatConversationsReq{
		SendId: a, RecvId: b, ConversationId: notice, ConversationType: model.ConversationNotification,
	}); err != nil {
		t.Fatal(err)
	}
	recv, err := st.ConversationsByID(ctx, b, []string{notice})
	send, sendErr := st.ConversationsByID(ctx, a, []string{notice})
	if err != nil || sendErr != nil || len(recv) != 1 || len(send) != 0 || recv[0].UserID != a {
		t.Fatalf("notification recv=%+v send=%+v err=%v %v", recv, send, err, sendErr)
	}
}

type recordSeq struct {
	maxCalls  int
	minCalls  int
	lastSeq   int64
	lastUsers []string
}

func (r *recordSeq) SetUserMaxSeq(_ context.Context, _ string, userIDs []string, seq int64) error {
	r.maxCalls++
	r.lastSeq = seq
	r.lastUsers = append([]string(nil), userIDs...)
	return nil
}

func (r *recordSeq) SetUserMinSeq(_ context.Context, _ string, _ []string, seq int64) error {
	r.minCalls++
	r.lastSeq = seq
	return nil
}

type captureSender struct {
	msgs []*msgpb.MsgData
}

func (c *captureSender) SendNotification(_ context.Context, msg *msgpb.MsgData) error {
	c.msgs = append(c.msgs, msg)
	return nil
}

func noticeHas(msgs []*msgpb.MsgData, contentType int32, part string) bool {
	for _, msg := range msgs {
		if msg.GetContentType() != contentType {
			continue
		}
		var wrap struct {
			Detail string `json:"detail"`
		}
		if json.Unmarshal(msg.GetContent(), &wrap) != nil {
			continue
		}
		if strings.Contains(wrap.Detail, part) {
			return true
		}
	}
	return false
}

func noticeTypes(msgs []*msgpb.MsgData) []int32 {
	out := make([]int32, 0, len(msgs))
	for _, msg := range msgs {
		out = append(out, msg.GetContentType())
	}
	return out
}

func boolPtr(v bool) *bool { return &v }

func i32Ptr(v int32) *int32 { return &v }

func strPtr(v string) *string { return &v }

func isCode(err error, grpc codes.Code, code errorcode.Code) bool {
	got, ok := errorcode.CodeOf(err)
	return ok && got == code && status.Code(err) == grpc
}

func cleanupConversation(t *testing.T, cfg config.MySQL, ids ...string) {
	t.Helper()
	db, err := sql.Open("mysql", mysqlDSN(cfg))
	if err != nil {
		t.Errorf("cleanup connect: %v", err)
		return
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, id := range ids {
		if _, err := db.ExecContext(ctx, `DELETE FROM conversation WHERE owner_user_id = ?`, id); err != nil {
			t.Errorf("cleanup conversation: %v", err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM conversation_version_log WHERE d_id = ?`, id); err != nil {
			t.Errorf("cleanup conversation version log: %v", err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM conversation_version WHERE d_id = ?`, id); err != nil {
			t.Errorf("cleanup conversation version: %v", err)
		}
	}
}

func readEnvFile(t *testing.T, path string) map[string]string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		out[k] = v
	}
	return out
}
