package store

import (
	"context"
	"database/sql"
	"fmt"
)

// LastSeqByTime returns the greatest seq at or before timeMs for one conversation.
func (s *Store) LastSeqByTime(ctx context.Context, conversationID string, timeMs int64) (int64, error) {
	if conversationID == "" {
		return 0, nil
	}
	var seq sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
SELECT MAX(seq) FROM msg WHERE conversation_id = ? AND send_time <= ?`, conversationID, timeMs).Scan(&seq)
	if err != nil {
		return 0, err
	}
	if !seq.Valid {
		return 0, nil
	}
	return seq.Int64, nil
}

// SetConversationMinSeq raises the conversation floor. A smaller value is ignored.
func (s *Store) SetConversationMinSeq(ctx context.Context, conversationID string, minSeq int64) error {
	if conversationID == "" || minSeq < 0 {
		return fmt.Errorf("conversation min seq is invalid")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO seq_conversation (conversation_id, min_seq, max_seq) VALUES (?, ?, 0) AS new
ON DUPLICATE KEY UPDATE min_seq = GREATEST(seq_conversation.min_seq, new.min_seq)`,
		conversationID, minSeq)
	return err
}

// DestructBefore deletes up to limit conversations' messages older than timestampMs.
// For each conversation it raises seq_conversation.min_seq past the deleted range.
func (s *Store) DestructBefore(ctx context.Context, timestampMs int64, limit int) (int, error) {
	if limit <= 0 {
		limit = 50
	}
	if timestampMs <= 0 {
		return 0, fmt.Errorf("timestamp is invalid")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT conversation_id, MAX(seq) AS last_seq
FROM msg
WHERE send_time < ?
GROUP BY conversation_id
ORDER BY RAND()
LIMIT ?`, timestampMs, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type hit struct {
		id  string
		seq int64
	}
	var hits []hit
	for rows.Next() {
		var h hit
		if err := rows.Scan(&h.id, &h.seq); err != nil {
			return 0, err
		}
		if h.id != "" && h.seq > 0 {
			hits = append(hits, h)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	for _, h := range hits {
		if _, err := s.db.ExecContext(ctx, `
DELETE FROM msg WHERE conversation_id = ? AND send_time < ?`, h.id, timestampMs); err != nil {
			return 0, err
		}
		if err := s.SetConversationMinSeq(ctx, h.id, h.seq+1); err != nil {
			return 0, err
		}
	}
	return len(hits), nil
}
