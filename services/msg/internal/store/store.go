package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

// Store reads sequenced messages written by msgtransfer.
// Seq assignment stays in msgtransfer. This service only reads the rows and publishes new sends.
type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, mysqlDSN string, maxOpen, maxIdle int) (*Store, error) {
	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		return nil, err
	}
	if maxOpen > 0 {
		db.SetMaxOpenConns(maxOpen)
	}
	if maxIdle > 0 {
		db.SetMaxIdleConns(maxIdle)
	}
	db.SetConnMaxLifetime(time.Hour)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context, script string) error {
	for _, stmt := range strings.Split(script, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

type SeqState struct {
	MaxSeq     int64
	HasReadSeq int64
}

// ConversationIDs returns conversations this user has sent, received, or read.
func (s *Store) ConversationIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT conversation_id FROM msg WHERE send_id = ? OR recv_id = ?
UNION
SELECT conversation_id FROM seq_user WHERE user_id = ?`, userID, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStrings(rows)
}

func (s *Store) MaxSeqs(ctx context.Context, conversationIDs []string) (map[string]int64, error) {
	out := map[string]int64{}
	if len(conversationIDs) == 0 {
		return out, nil
	}
	query := `SELECT conversation_id, max_seq FROM seq_conversation WHERE conversation_id IN (` + placeholders(len(conversationIDs)) + `)`
	rows, err := s.db.QueryContext(ctx, query, args(conversationIDs)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var seq int64
		if err := rows.Scan(&id, &seq); err != nil {
			return nil, err
		}
		out[id] = seq
	}
	return out, rows.Err()
}

func (s *Store) SeqState(ctx context.Context, userID string, conversationIDs []string) (map[string]SeqState, error) {
	maxSeq, err := s.MaxSeqs(ctx, conversationIDs)
	if err != nil {
		return nil, err
	}
	readSeq := map[string]int64{}
	if len(conversationIDs) > 0 {
		query := `SELECT conversation_id, read_seq FROM seq_user WHERE user_id = ? AND conversation_id IN (` + placeholders(len(conversationIDs)) + `)`
		params := append([]any{userID}, args(conversationIDs)...)
		rows, err := s.db.QueryContext(ctx, query, params...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var seq int64
			if err := rows.Scan(&id, &seq); err != nil {
				return nil, err
			}
			readSeq[id] = seq
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	out := make(map[string]SeqState, len(conversationIDs))
	for _, id := range conversationIDs {
		out[id] = SeqState{MaxSeq: maxSeq[id], HasReadSeq: readSeq[id]}
	}
	return out, nil
}

func (s *Store) Range(ctx context.Context, conversationID string, begin, end, limit int64) ([]*msgpb.MsgData, error) {
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	if end <= 0 {
		end = begin + limit - 1
	}
	if begin <= 0 {
		begin = 1
	}
	rows, err := s.db.QueryContext(ctx, selectMsg+`
WHERE conversation_id = ? AND seq >= ? AND seq <= ?
ORDER BY seq ASC LIMIT ?`, conversationID, begin, end, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMsgs(rows)
}

func (s *Store) BySeqs(ctx context.Context, conversationID string, seqs []int64) ([]*msgpb.MsgData, error) {
	if len(seqs) == 0 {
		return nil, nil
	}
	if len(seqs) > 200 {
		seqs = seqs[:200]
	}
	query := selectMsg + `WHERE conversation_id = ? AND seq IN (` + placeholders(len(seqs)) + `) ORDER BY seq ASC`
	params := make([]any, 0, len(seqs)+1)
	params = append(params, conversationID)
	for _, seq := range seqs {
		params = append(params, seq)
	}
	rows, err := s.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMsgs(rows)
}

func (s *Store) Last(ctx context.Context, conversationID string) (*msgpb.MsgData, error) {
	rows, err := s.db.QueryContext(ctx, selectMsg+`
WHERE conversation_id = ? ORDER BY seq DESC LIMIT 1`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	msgs, err := scanMsgs(rows)
	if err != nil || len(msgs) == 0 {
		return nil, err
	}
	return msgs[0], nil
}

const selectMsg = `
SELECT seq, send_id, recv_id, group_id, client_msg_id, server_msg_id, sender_platform_id,
  sender_nickname, sender_face_url, session_type, msg_from, content_type, content,
  send_time, create_time, status, is_read, options, at_user_id_list, attached_info, ex
FROM msg `

func scanMsgs(rows *sql.Rows) ([]*msgpb.MsgData, error) {
	var out []*msgpb.MsgData
	for rows.Next() {
		msg := &msgpb.MsgData{}
		var (
			options, atUsers, attached sql.NullString
			contentBytes               []byte
			read                       int
		)
		if err := rows.Scan(
			&msg.Seq, &msg.SendId, &msg.RecvId, &msg.GroupId, &msg.ClientMsgId, &msg.ServerMsgId, &msg.SenderPlatformId,
			&msg.SenderNickname, &msg.SenderFaceUrl, &msg.SessionType, &msg.MsgFrom, &msg.ContentType, &contentBytes,
			&msg.SendTime, &msg.CreateTime, &msg.Status, &read, &options, &atUsers, &attached, &msg.Ex,
		); err != nil {
			return nil, err
		}
		msg.Content = contentBytes
		msg.IsRead = read == 1
		if attached.Valid {
			msg.AttachedInfo = attached.String
		}
		if options.Valid && options.String != "" && options.String != "null" {
			msg.Options = map[string]bool{}
			if err := json.Unmarshal([]byte(options.String), &msg.Options); err != nil {
				return nil, err
			}
		}
		if atUsers.Valid && atUsers.String != "" && atUsers.String != "null" {
			if err := json.Unmarshal([]byte(atUsers.String), &msg.AtUserIdList); err != nil {
				return nil, err
			}
		}
		if msg.Status == 0 {
			msg.Status = 2
		}
		out = append(out, msg)
	}
	return out, rows.Err()
}

func scanStrings(rows *sql.Rows) ([]string, error) {
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		if s != "" {
			out = append(out, s)
		}
	}
	return out, rows.Err()
}

func placeholders(n int) string {
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}

func args(ids []string) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}
