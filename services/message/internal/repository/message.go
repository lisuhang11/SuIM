// Package repository 提供消息持久化的 GORM 实现。
package repository

import (
	"context"
	"errors"
	"strconv"
	"time"

	"message/internal/types"
	"message/internal/types/interfaces"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrRevokePermission 非发送者尝试撤回消息时返回。
	// 服务层通过 errors.Is 识别后包装为 *AppError。
	ErrRevokePermission = errors.New("only the sender can revoke this message")
)

// messageRepository GORM 实现的消息仓库。
type messageRepository struct {
	db *gorm.DB
}

// NewMessageRepository 创建 GORM 支持的消息仓库。
func NewMessageRepository(db *gorm.DB) interfaces.MessageRepository {
	return &messageRepository{db: db}
}

// docID 返回批次文档标识（每 100 条消息一个 doc，对应 OpenIM 的 MongoDB 设计）。
func docID(conversationID string, seq int64) string {
	return conversationID + ":" + strconv.FormatInt(seq/100, 10)
}

// SendMessage 在事务中原子执行：锁定会话 seq（FOR UPDATE），
// 计算下一个 seq，插入消息（含 doc_id/msg_index），
// 更新发送者和接收者的 seq_user.max_seq，
// 并尽最大努力同步 conversation.max_seq（供会话服务使用）。
func (r *messageRepository) SendMessage(ctx context.Context, msg *types.Message, recvUserIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 锁定当前会话 seq 值。
		var cur int64
		if err := tx.Raw("SELECT max_seq FROM seq_conversation WHERE conversation_id = ? FOR UPDATE", msg.ConversationID).Scan(&cur).Error; err != nil {
			return err
		}
		next := cur + 1
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "conversation_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"max_seq"}),
		}).Create(&types.SeqConversation{ConversationID: msg.ConversationID, MaxSeq: next}).Error; err != nil {
			return err
		}

		msg.Seq = next
		msg.DocID = docID(msg.ConversationID, next)
		msg.MsgIndex = int(next % 100)

		if err := tx.Create(msg).Error; err != nil {
			return err
		}

		// 更新发送者和接收者的 seq_user.max_seq。
		users := append([]string{}, recvUserIDs...)
		users = append(users, msg.SendID)
		for _, u := range users {
			if u == "" {
				continue
			}
			if err := tx.Exec(
				"INSERT INTO seq_user (user_id, conversation_id, max_seq) VALUES (?, ?, ?) "+
					"ON DUPLICATE KEY UPDATE max_seq = GREATEST(max_seq, VALUES(max_seq))",
				u, msg.ConversationID, next).Error; err != nil {
				return err
			}
			if u == msg.SendID {
				// 发送者默认已读自己的消息。
				if err := tx.Exec(
					"INSERT INTO seq_user (user_id, conversation_id, read_seq) VALUES (?, ?, ?) "+
						"ON DUPLICATE KEY UPDATE read_seq = GREATEST(read_seq, VALUES(read_seq))",
					u, msg.ConversationID, next).Error; err != nil {
					return err
				}
			}
		}

		// 尽最大努力同步 conversation.max_seq（供会话服务未读模型使用）。
		if err := tx.Table("conversation").Where("conversation_id = ?", msg.ConversationID).Update("max_seq", next).Error; err != nil {
			return err
		}
		return nil
	})
}

// GetByClientMsgIDs 按客户端消息 ID 列表批量查询。
func (r *messageRepository) GetByClientMsgIDs(ctx context.Context, ids []string) ([]types.Message, error) {
	var msgs []types.Message
	if len(ids) == 0 {
		return msgs, nil
	}
	err := r.db.WithContext(ctx).Where("client_msg_id IN ?", ids).Find(&msgs).Error
	return msgs, err
}

// GetBySeqs 按 seq 列表查询指定会话的消息。
func (r *messageRepository) GetBySeqs(ctx context.Context, conversationID string, seqs []int64) ([]types.Message, error) {
	var msgs []types.Message
	if len(seqs) == 0 {
		return msgs, nil
	}
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND seq IN ?", conversationID, seqs).
		Order("seq ASC").Find(&msgs).Error
	return msgs, err
}

// GetHistory 游标分页加载历史消息：多取 1 条判断是否还有更多，
// 返回截断后的结果和截断前的匹配行数（调用方据此推导 is_end）。
func (r *messageRepository) GetHistory(ctx context.Context, conversationID string, anchorSeq int64, limit, order int) ([]types.Message, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	q := r.db.WithContext(ctx).Model(&types.Message{}).Where("conversation_id = ?", conversationID)
	if anchorSeq != 0 {
		if order == 1 {
			q = q.Where("seq > ?", anchorSeq)
		} else {
			q = q.Where("seq < ?", anchorSeq)
		}
	}
	if order == 1 {
		q = q.Order("seq ASC")
	} else {
		q = q.Order("seq DESC")
	}
	var raw []types.Message
	if err := q.Limit(limit + 1).Find(&raw).Error; err != nil {
		return nil, 0, err
	}
	matched := int64(len(raw))
	if len(raw) > limit {
		raw = raw[:limit]
	}
	return raw, matched, nil
}

// Revoke 撤回消息（仅发送者可撤回），否则返回 ErrRevokePermission。
func (r *messageRepository) Revoke(ctx context.Context, conversationID, clientMsgID, sendID string, revokeRole int32, revokeNickname string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var m types.Message
		if err := tx.Where("conversation_id = ? AND client_msg_id = ?", conversationID, clientMsgID).First(&m).Error; err != nil {
			return err
		}
		if m.SendID != sendID {
			return ErrRevokePermission
		}
		return tx.Model(&types.Message{}).
			Where("conversation_id = ? AND client_msg_id = ?", conversationID, clientMsgID).
			Updates(map[string]any{
				"status":          types.MsgStatusRevoke,
				"revoke_role":     revokeRole,
				"revoke_user_id":  sendID,
				"revoke_nickname": revokeNickname,
				"revoke_time":     time.Now().UnixMilli(),
			}).Error
	})
}

// MarkMessagesRead 将指定 seq 及之前的消息标记为已读。
func (r *messageRepository) MarkMessagesRead(ctx context.Context, conversationID string, seq int64) error {
	return r.db.WithContext(ctx).Model(&types.Message{}).
		Where("conversation_id = ? AND seq <= ?", conversationID, seq).
		Update("is_read", true).Error
}

// SetConversationMinSeq 推进用户的已读游标（min_seq），但不会回退。
// 尽最大努力同步（会话服务兼容）；权威已读游标在 seq_user.read_seq。
func (r *messageRepository) SetConversationMinSeq(ctx context.Context, conversationID, userID string, seq int64) error {
	return r.db.WithContext(ctx).Table("conversation").
		Where("conversation_id = ? AND owner_user_id = ? AND min_seq < ?", conversationID, userID, seq).
		Update("min_seq", seq).Error
}

// Delete 按 seq 列表删除消息。
func (r *messageRepository) Delete(ctx context.Context, conversationID string, seqs []int64) error {
	if len(seqs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("conversation_id = ? AND seq IN ?", conversationID, seqs).
		Delete(&types.Message{}).Error
}
