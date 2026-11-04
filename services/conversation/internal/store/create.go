package store

import (
	"context"
	"sort"
	"time"

	"github.com/lisuhang11/SuIM/services/conversation/internal/model"
)

// CreateMissing inserts conversation rows that are not already there.
// An existing row is left as it is, including its pin and sequence fields.
func (s *Store) CreateMissing(ctx context.Context, rows []*model.Conversation, now time.Time) error {
	if len(rows) == 0 {
		return nil
	}
	ordered := append([]*model.Conversation(nil), rows...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].OwnerUserID == ordered[j].OwnerUserID {
			return ordered[i].ConversationID < ordered[j].ConversationID
		}
		return ordered[i].OwnerUserID < ordered[j].OwnerUserID
	})
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, row := range ordered {
		if row == nil || row.OwnerUserID == "" || row.ConversationID == "" {
			continue
		}
		existing, err := lockConversation(ctx, tx, row.OwnerUserID, row.ConversationID)
		if err != nil {
			return err
		}
		if existing != nil {
			continue
		}
		row.CreateTime = now
		if err := insertConversation(ctx, tx, row); err != nil {
			return err
		}
		if err := incrConversation(ctx, tx, row.OwnerUserID, row.ConversationID, model.VersionInsert, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetOwnerSeq writes max_seq or min_seq on rows that already exist.
// Owners without a row are skipped. Unchanged values are skipped. The returned owners are the ones that changed.
func (s *Store) SetOwnerSeq(ctx context.Context, conversationID string, owners []string, max bool, seq int64, now time.Time) ([]string, error) {
	if conversationID == "" || len(owners) == 0 {
		return nil, nil
	}
	order := append([]string(nil), owners...)
	sort.Strings(order)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	column := "min_seq"
	var changed []string
	for _, owner := range order {
		row, err := lockConversation(ctx, tx, owner, conversationID)
		if err != nil {
			return nil, err
		}
		if row == nil {
			continue
		}
		current := row.MinSeq
		if max {
			column = "max_seq"
			current = row.MaxSeq
		}
		if current == seq {
			continue
		}
		if _, err := tx.ExecContext(ctx, "UPDATE conversation SET "+column+" = ? WHERE owner_user_id = ? AND conversation_id = ?", seq, owner, conversationID); err != nil {
			return nil, err
		}
		if err := incrConversation(ctx, tx, owner, conversationID, model.VersionUpdate, now); err != nil {
			return nil, err
		}
		changed = append(changed, owner)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return changed, nil
}

// PinnedIDs lists conversations this owner has pinned.
func (s *Store) PinnedIDs(ctx context.Context, owner string) ([]string, error) {
	if owner == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT conversation_id FROM conversation WHERE owner_user_id = ? AND is_pinned = 1 ORDER BY conversation_id ASC`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
