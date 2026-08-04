// Package repository 提供消息持久化的 GORM 实现。
package repository

import (
	"context"
	"errors"
	"fmt"
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
	// ErrRevokeExpired 超过撤回时限时返回。
	ErrRevokeExpired = errors.New("message can only be revoked within 2 minutes")
)

// RevokeTimeLimit 发送后可撤回的最长时长。
const RevokeTimeLimit = 2 * time.Minute

// messageRepository GORM 实现的消息仓库。
type messageRepository struct {
	db *gorm.DB
}

// NewMessageRepository 创建 GORM 支持的消息仓库。
func NewMessageRepository(db *gorm.DB) interfaces.MessageRepository {
	return &messageRepository{db: db}
}

func visibleToUser(q *gorm.DB, userID string) *gorm.DB {
	return q.Where(
		"EXISTS (SELECT 1 FROM seq_user su WHERE su.user_id = ? AND su.conversation_id = msg_info.conversation_id AND msg_info.seq BETWEEN su.min_seq AND su.max_seq)",
		userID,
	).Where(
		"NOT EXISTS (SELECT 1 FROM msg_delete md WHERE md.message_id = msg_info.id AND md.user_id = ?)",
		userID,
	)
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

		// 仅推进本条消息参与者（发送者+接收者）的 conversation.max_seq。
		// 退群/被踢用户不再出现在接收列表中，其会话 max_seq 自然冻结（对齐 OpenIM）。
		participantIDs := make([]string, 0, len(users))
		seen := make(map[string]struct{}, len(users))
		for _, u := range users {
			if u == "" {
				continue
			}
			if _, ok := seen[u]; ok {
				continue
			}
			seen[u] = struct{}{}
			participantIDs = append(participantIDs, u)
		}
		if len(participantIDs) > 0 {
			if err := tx.Table("conversation").
				Where("conversation_id = ? AND owner_user_id IN ?", msg.ConversationID, participantIDs).
				Update("max_seq", next).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetByClientMsgIDs 按客户端消息 ID 列表批量查询。
func (r *messageRepository) GetByClientMsgIDs(ctx context.Context, userID string, ids []string) ([]types.Message, error) {
	var msgs []types.Message
	if len(ids) == 0 {
		return msgs, nil
	}
	q := r.db.WithContext(ctx).Model(&types.Message{})
	err := visibleToUser(q, userID).Where("client_msg_id IN ?", ids).Find(&msgs).Error
	return msgs, err
}

func (r *messageRepository) GetBySenderClientMsgIDs(ctx context.Context, senderID string, ids []string) ([]types.Message, error) {
	var msgs []types.Message
	if len(ids) == 0 {
		return msgs, nil
	}
	err := r.db.WithContext(ctx).
		Where("send_id = ? AND client_msg_id IN ?", senderID, ids).
		Find(&msgs).Error
	return msgs, err
}

// GetBySeqs 按 seq 列表查询指定会话的消息。
func (r *messageRepository) GetBySeqs(ctx context.Context, userID, conversationID string, seqs []int64) ([]types.Message, error) {
	var msgs []types.Message
	if len(seqs) == 0 {
		return msgs, nil
	}
	q := r.db.WithContext(ctx).Model(&types.Message{})
	err := visibleToUser(q, userID).
		Where("conversation_id = ? AND seq IN ?", conversationID, seqs).
		Order("seq ASC").Find(&msgs).Error
	return msgs, err
}

// GetHistory 游标分页加载历史消息：多取 1 条判断是否还有更多，
// 返回截断后的结果和截断前的匹配行数（调用方据此推导 is_end）。
func (r *messageRepository) GetHistory(ctx context.Context, userID, conversationID string, anchorSeq int64, limit, order int) ([]types.Message, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	q := visibleToUser(r.db.WithContext(ctx).Model(&types.Message{}), userID).
		Where("conversation_id = ?", conversationID)
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

// Revoke 撤回消息（仅发送者可撤回），否则返回 ErrRevokePermission；成功时返回撤回后快照。
func (r *messageRepository) Revoke(ctx context.Context, conversationID, clientMsgID, sendID string, revokeRole int32, revokeNickname string) (*types.Message, error) {
	var out types.Message
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var m types.Message
		if err := tx.Where("conversation_id = ? AND client_msg_id = ?", conversationID, clientMsgID).First(&m).Error; err != nil {
			return err
		}
		if m.SendID != sendID {
			return ErrRevokePermission
		}
		if m.Status == types.MsgStatusRevoke {
			out = m
			return nil
		}
		if !withinRevokeWindow(m.SendTime, time.Now()) {
			return ErrRevokeExpired
		}
		now := time.Now().UnixMilli()
		if err := tx.Model(&types.Message{}).
			Where("conversation_id = ? AND client_msg_id = ?", conversationID, clientMsgID).
			Updates(map[string]any{
				"status":          types.MsgStatusRevoke,
				"revoke_role":     revokeRole,
				"revoke_user_id":  sendID,
				"revoke_nickname": revokeNickname,
				"revoke_time":     now,
			}).Error; err != nil {
			return err
		}
		m.Status = types.MsgStatusRevoke
		m.RevokeRole = int(revokeRole)
		m.RevokeUserID = sendID
		m.RevokeNickname = revokeNickname
		m.RevokeTime = now
		out = m
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// withinRevokeWindow 判断 sendTime（Unix 毫秒，兼容秒）是否仍在撤回时限内。
func withinRevokeWindow(sendTimeMs int64, now time.Time) bool {
	if sendTimeMs <= 0 {
		return false
	}
	if sendTimeMs < 1e12 {
		sendTimeMs *= 1000
	}
	elapsed := now.Sub(time.UnixMilli(sendTimeMs))
	return elapsed >= 0 && elapsed <= RevokeTimeLimit
}

// SetReadSeq 推进单个用户的已读游标，不修改公共消息行。
func (r *messageRepository) SetReadSeq(ctx context.Context, userID, conversationID string, seq int64) error {
	result := r.db.WithContext(ctx).Exec(
		"UPDATE seq_user SET read_seq = GREATEST(read_seq, LEAST(?, max_seq)) WHERE user_id = ? AND conversation_id = ?",
		seq, userID, conversationID,
	)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := r.db.WithContext(ctx).Model(&types.SeqUser{}).
			Where("user_id = ? AND conversation_id = ?", userID, conversationID).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
	}
	return nil
}

// ListSeqUser 返回用户的 seq_user 行；conversationIDs 为空时返回该用户全部行。
func (r *messageRepository) ListSeqUser(ctx context.Context, userID string, conversationIDs []string) ([]types.SeqUser, error) {
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	q := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if len(conversationIDs) > 0 {
		q = q.Where("conversation_id IN ?", conversationIDs)
	}
	var rows []types.SeqUser
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// MapSendTimeByConvSeq 按 (conversation_id, seq) 批量查 send_time；结果 key 为 conversation_id。
func (r *messageRepository) MapSendTimeByConvSeq(ctx context.Context, conversationSeqs map[string]int64) (map[string]int64, error) {
	out := make(map[string]int64, len(conversationSeqs))
	if len(conversationSeqs) == 0 {
		return out, nil
	}
	type row struct {
		ConversationID string `gorm:"column:conversation_id"`
		Seq            int64  `gorm:"column:seq"`
		SendTime       int64  `gorm:"column:send_time"`
	}
	q := r.db.WithContext(ctx).Model(&types.Message{}).Select("conversation_id, seq, send_time")
	first := true
	for id, seq := range conversationSeqs {
		if id == "" || seq <= 0 {
			continue
		}
		if first {
			q = q.Where("(conversation_id = ? AND seq = ?)", id, seq)
			first = false
		} else {
			q = q.Or("(conversation_id = ? AND seq = ?)", id, seq)
		}
	}
	if first {
		return out, nil
	}
	var rows []row
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ConversationID] = row.SendTime
	}
	return out, nil
}

// GetLastMessage 批量取每个会话对用户可见的最后一条消息（跳过用户已删）。
func (r *messageRepository) GetLastMessage(ctx context.Context, userID string, conversationIDs []string) (map[string]types.Message, error) {
	out := make(map[string]types.Message, len(conversationIDs))
	if userID == "" || len(conversationIDs) == 0 {
		return out, nil
	}
	var rows []types.Message
	err := visibleToUser(r.db.WithContext(ctx).Model(&types.Message{}), userID).
		Where("conversation_id IN ?", conversationIDs).
		Where("seq = (SELECT MAX(m2.seq) FROM msg_info m2 WHERE m2.conversation_id = msg_info.conversation_id "+
			"AND EXISTS (SELECT 1 FROM seq_user su WHERE su.user_id = ? AND su.conversation_id = m2.conversation_id AND m2.seq BETWEEN su.min_seq AND su.max_seq) "+
			"AND NOT EXISTS (SELECT 1 FROM msg_delete md WHERE md.message_id = m2.id AND md.user_id = ?))", userID, userID).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for i := range rows {
		out[rows[i].ConversationID] = rows[i]
	}
	return out, nil
}

// DeleteForUser 按 seq 记录用户级删除标记，不影响其他会话成员。
func (r *messageRepository) DeleteForUser(ctx context.Context, userID, conversationID string, seqs []int64) error {
	if len(seqs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Exec(
		"INSERT IGNORE INTO msg_delete (message_id, user_id, created_at) "+
			"SELECT mi.id, ?, ? FROM msg_info mi "+
			"WHERE mi.conversation_id = ? AND mi.seq IN ? "+
			"AND EXISTS (SELECT 1 FROM seq_user su WHERE su.user_id = ? AND su.conversation_id = mi.conversation_id AND mi.seq BETWEEN su.min_seq AND su.max_seq)",
		userID, time.Now().UnixMilli(), conversationID, seqs, userID,
	).Error
}
