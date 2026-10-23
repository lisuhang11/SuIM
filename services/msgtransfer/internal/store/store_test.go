package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/lisuhang11/SuIM/pkg/config"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/msgtransfer/internal/model"
)

func TestInsertAndReadSeq(t *testing.T) {
	ctx := context.Background()
	env := readEnv(filepath.Join("..", "..", "..", "..", ".env"))
	cfg := config.MySQL{
		Address:     []string{"127.0.0.1:3306"},
		Username:    env["MYSQL_USER"],
		Password:    env["MYSQL_PASSWORD"],
		Database:    env["MYSQL_DATABASE"],
		MaxOpenConn: 5,
		MaxIdleConn: 2,
	}
	if cfg.Username == "" || cfg.Database == "" {
		t.Skip("mysql env is not set")
	}
	st, err := Open(ctx, mysqlDSN(cfg), cfg.MaxOpenConn, cfg.MaxIdleConn)
	if err != nil {
		t.Skipf("mysql is not reachable: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "migrations", "msg.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx, string(script)); err != nil {
		t.Fatal(err)
	}

	conversationID := "si_store_" + time.Now().Format("150405.000000")
	t.Cleanup(func() {
		_, _ = st.db.ExecContext(context.Background(), `DELETE FROM msg WHERE conversation_id = ?`, conversationID)
		_, _ = st.db.ExecContext(context.Background(), `DELETE FROM seq_conversation WHERE conversation_id = ?`, conversationID)
		_, _ = st.db.ExecContext(context.Background(), `DELETE FROM seq_user WHERE conversation_id = ?`, conversationID)
	})

	msg := &msgpb.MsgData{
		SendId: "a", RecvId: "b", SessionType: model.SessionSingle, ContentType: 101,
		Content: []byte("hello"), ClientMsgId: "c1", ServerMsgId: "s1", Seq: 1,
		Status: model.StatusSending, Options: map[string]bool{model.OptHistory: true},
		AtUserIdList: []string{"b"},
	}
	if err := st.InsertMessages(ctx, conversationID, []*msgpb.MsgData{msg}); err != nil {
		t.Fatal(err)
	}
	if err := st.InsertMessages(ctx, conversationID, []*msgpb.MsgData{msg}); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetMessage(ctx, conversationID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || string(got.Content) != "hello" || got.Status != model.StatusSuccess {
		t.Fatalf("stored %+v", got)
	}
	if len(got.AtUserIdList) != 1 || got.AtUserIdList[0] != "b" {
		t.Fatalf("at users %+v", got.AtUserIdList)
	}
	if !got.Options[model.OptHistory] {
		t.Fatalf("options %+v", got.Options)
	}
	if err := st.SaveMaxSeq(ctx, conversationID, 2); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveMaxSeq(ctx, conversationID, 1); err != nil {
		t.Fatal(err)
	}
	maxSeq, err := st.MaxSeq(ctx, conversationID)
	if err != nil || maxSeq != 2 {
		t.Fatalf("max %d err %v", maxSeq, err)
	}
	if err := st.SetUserReadSeq(ctx, conversationID, map[string]int64{"a": 4}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetUserReadSeq(ctx, conversationID, map[string]int64{"a": 1}); err != nil {
		t.Fatal(err)
	}
	readSeq, err := st.UserReadSeq(ctx, conversationID, "a")
	if err != nil || readSeq != 4 {
		t.Fatalf("read %d err %v", readSeq, err)
	}
	lower, err := st.UserReadSeq(ctx, conversationID, "missing")
	if err != nil || lower != 0 {
		t.Fatalf("missing read %d err %v", lower, err)
	}
}

func TestEnsureSingleConversation(t *testing.T) {
	ctx := context.Background()
	env := readEnv(filepath.Join("..", "..", "..", "..", ".env"))
	cfg := config.MySQL{
		Address:     []string{"127.0.0.1:3306"},
		Username:    env["MYSQL_USER"],
		Password:    env["MYSQL_PASSWORD"],
		Database:    env["MYSQL_DATABASE"],
		MaxOpenConn: 5,
		MaxIdleConn: 2,
	}
	if cfg.Username == "" || cfg.Database == "" {
		t.Skip("mysql env is not set")
	}
	st, err := Open(ctx, mysqlDSN(cfg), cfg.MaxOpenConn, cfg.MaxIdleConn)
	if err != nil {
		t.Skipf("mysql is not reachable: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	for _, name := range []string{"msg.sql", "conversation.sql"} {
		script, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := st.Migrate(ctx, string(script)); err != nil {
			t.Fatal(err)
		}
	}
	conversationID := "si_ensure_" + time.Now().Format("150405.000000")
	t.Cleanup(func() {
		_, _ = st.db.ExecContext(context.Background(), `DELETE FROM conversation WHERE conversation_id = ?`, conversationID)
	})
	if err := st.Ensure(ctx, conversationID, "b", "a"); err != nil {
		t.Fatal(err)
	}
	if err := st.Ensure(ctx, conversationID, "b", "a"); err != nil {
		t.Fatal(err)
	}
	rows, err := st.db.QueryContext(ctx, `
SELECT owner_user_id, user_id, conversation_type, max_seq
FROM conversation WHERE conversation_id = ? ORDER BY owner_user_id`, conversationID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type row struct {
		owner, peer string
		kind        int
		maxSeq      int64
	}
	var got []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.owner, &item.peer, &item.kind, &item.maxSeq); err != nil {
			t.Fatal(err)
		}
		got = append(got, item)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].owner != "a" || got[0].peer != "b" || got[1].owner != "b" || got[1].peer != "a" {
		t.Fatalf("rows %+v", got)
	}
	for _, item := range got {
		if item.kind != 1 || item.maxSeq != 0 {
			t.Fatalf("row %+v", item)
		}
	}
}

func mysqlDSN(cfg config.MySQL) string {
	return cfg.Username + ":" + cfg.Password + "@tcp(" + cfg.Address[0] + ")/" + cfg.Database + "?parseTime=true&charset=utf8mb4&loc=UTC&timeout=5s"
}

func readEnv(path string) map[string]string {
	b, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
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
