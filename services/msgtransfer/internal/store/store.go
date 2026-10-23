package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/msgtransfer/internal/model"
)

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, mysqlDSN string, mysqlMaxOpen, mysqlMaxIdle int) (*Store, error) {
	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		return nil, err
	}
	if mysqlMaxOpen > 0 {
		db.SetMaxOpenConns(mysqlMaxOpen)
	}
	if mysqlMaxIdle > 0 {
		db.SetMaxIdleConns(mysqlMaxIdle)
	}
	db.SetConnMaxLifetime(time.Hour)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s.db == nil {
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

// InsertMessages writes one row per message. A repeated sequence is ignored.
func (s *Store) InsertMessages(ctx context.Context, conversationID string, msgs []*msgpb.MsgData) error {
	if len(msgs) == 0 {
		return fmt.Errorf("no messages")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, msg := range msgs {
		if msg == nil || msg.Seq <= 0 {
			return fmt.Errorf("message seq is empty")
		}
		status := msg.Status
		if status == model.StatusSending || status == 0 {
			status = model.StatusSuccess
		}
		options, err := jsonOrNil(msg.Options)
		if err != nil {
			return err
		}
		atUsers, err := jsonOrNil(msg.AtUserIdList)
		if err != nil {
			return err
		}
		read := 0
		if msg.IsRead {
			read = 1
		}
		_, err = tx.ExecContext(ctx, `
INSERT IGNORE INTO msg (
  conversation_id, seq, send_id, recv_id, group_id, client_msg_id, server_msg_id,
  sender_platform_id, sender_nickname, sender_face_url, session_type, msg_from,
  content_type, content, send_time, create_time, status, is_read, options,
  at_user_id_list, attached_info, ex
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			conversationID, msg.Seq, msg.SendId, msg.RecvId, msg.GroupId, msg.ClientMsgId, msg.ServerMsgId,
			msg.SenderPlatformId, msg.SenderNickname, msg.SenderFaceUrl, msg.SessionType, msg.MsgFrom,
			msg.ContentType, msg.Content, msg.SendTime, msg.CreateTime, status, read, options,
			atUsers, msg.AttachedInfo, msg.Ex,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetMessage loads one stored message.
func (s *Store) GetMessage(ctx context.Context, conversationID string, seq int64) (*msgpb.MsgData, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT send_id, recv_id, group_id, client_msg_id, server_msg_id, sender_platform_id,
  sender_nickname, sender_face_url, session_type, msg_from, content_type, content,
  send_time, create_time, status, is_read, options, at_user_id_list, attached_info, ex
FROM msg WHERE conversation_id = ? AND seq = ?`, conversationID, seq)
	msg := &msgpb.MsgData{Seq: seq}
	var (
		options, atUsers, attached sql.NullString
		contentBytes               []byte
		read                       int
	)
	err := row.Scan(
		&msg.SendId, &msg.RecvId, &msg.GroupId, &msg.ClientMsgId, &msg.ServerMsgId, &msg.SenderPlatformId,
		&msg.SenderNickname, &msg.SenderFaceUrl, &msg.SessionType, &msg.MsgFrom, &msg.ContentType, &contentBytes,
		&msg.SendTime, &msg.CreateTime, &msg.Status, &read, &options, &atUsers, &attached, &msg.Ex,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
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
	return msg, nil
}

// SaveMaxSeq raises the stored conversation max when seq is greater.
func (s *Store) SaveMaxSeq(ctx context.Context, conversationID string, maxSeq int64) error {
	if maxSeq <= 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO seq_conversation (conversation_id, max_seq) VALUES (?, ?) AS new
ON DUPLICATE KEY UPDATE max_seq = GREATEST(seq_conversation.max_seq, new.max_seq)`,
		conversationID, maxSeq)
	return err
}

// MaxSeq returns 0 when the conversation has not allocated a sequence yet.
func (s *Store) MaxSeq(ctx context.Context, conversationID string) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, `SELECT max_seq FROM seq_conversation WHERE conversation_id = ?`, conversationID).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return n, err
}

// SetUserReadSeq raises each user's stored read sequence.
func (s *Store) SetUserReadSeq(ctx context.Context, conversationID string, userSeq map[string]int64) error {
	for userID, seq := range userSeq {
		if userID == "" || seq <= 0 {
			continue
		}
		_, err := s.db.ExecContext(ctx, `
INSERT INTO seq_user (user_id, conversation_id, read_seq) VALUES (?, ?, ?) AS new
ON DUPLICATE KEY UPDATE read_seq = GREATEST(seq_user.read_seq, new.read_seq)`,
			userID, conversationID, seq)
		if err != nil {
			return err
		}
	}
	return nil
}

// UserReadSeq returns 0 when the user has no read record.
func (s *Store) UserReadSeq(ctx context.Context, conversationID, userID string) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, `
SELECT read_seq FROM seq_user WHERE user_id = ? AND conversation_id = ?`, userID, conversationID).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return n, err
}

// Ensure creates both single-chat conversation rows. A repeated call keeps the existing rows.
// max_seq on the row stays 0. That field is a per-user visibility cap, not the message cursor.
func (s *Store) Ensure(ctx context.Context, conversationID, sendID, recvID string) error {
	if conversationID == "" || sendID == "" || recvID == "" || sendID == recvID {
		return fmt.Errorf("single conversation is incomplete")
	}
	const query = `
INSERT IGNORE INTO conversation (
  owner_user_id, conversation_id, conversation_type, user_id, group_id
) VALUES (?, ?, 1, ?, '')`
	if _, err := s.db.ExecContext(ctx, query, sendID, conversationID, recvID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, query, recvID, conversationID, sendID)
	return err
}

func jsonOrNil(v any) (any, error) {
	switch t := v.(type) {
	case map[string]bool:
		if len(t) == 0 {
			return nil, nil
		}
	case []string:
		if len(t) == 0 {
			return nil, nil
		}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}
