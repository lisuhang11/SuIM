package store

import (
	"context"
	"time"

	"github.com/lisuhang11/SuIM/services/conversation/internal/model"
)

// FindDueDestruct lists conversations whose timed clear window has elapsed.
// Condition: is_msg_destruct=1, msg_destruct_time>0, and either never cleared
// or DATE_ADD(latest, INTERVAL msg_destruct_time SECOND) < ts.
// Compare DATETIME values from Go. UNIX_TIMESTAMP()/FROM_UNIXTIME() are avoided:
// on some MySQL sessions they return 0 / skewed instants and keep cleared rows "due".
func (s *Store) FindDueDestruct(ctx context.Context, tsMs int64, limit int) ([]*model.Conversation, error) {
	if limit <= 0 {
		limit = 100
	}
	when := time.UnixMilli(tsMs).UTC()
	rows, err := s.db.QueryContext(ctx, conversationSelect+`
WHERE is_msg_destruct = 1
  AND msg_destruct_time <> 0
  AND (
    latest_msg_destruct_time IS NULL
    OR DATE_ADD(latest_msg_destruct_time, INTERVAL msg_destruct_time SECOND) < ?
  )
ORDER BY RAND()
LIMIT ?`, when, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Conversation
	for rows.Next() {
		row, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// MarkDestructed writes latest_msg_destruct_time and optionally min_seq for one owner row.
// minSeq < 0 leaves min_seq unchanged.
func (s *Store) MarkDestructed(ctx context.Context, owner, conversationID string, minSeq int64, when time.Time) error {
	if owner == "" || conversationID == "" {
		return nil
	}
	when = when.UTC()
	if minSeq >= 0 {
		_, err := s.db.ExecContext(ctx, `
UPDATE conversation
SET min_seq = ?, latest_msg_destruct_time = ?
WHERE owner_user_id = ? AND conversation_id = ?`, minSeq, when, owner, conversationID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE conversation
SET latest_msg_destruct_time = ?
WHERE owner_user_id = ? AND conversation_id = ?`, when, owner, conversationID)
	return err
}
