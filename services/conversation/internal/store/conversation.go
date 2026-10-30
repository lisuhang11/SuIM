package store

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"

	"github.com/lisuhang11/SuIM/services/conversation/internal/model"
)

const conversationSelect = `SELECT owner_user_id, conversation_id, conversation_type, user_id, group_id,
	recv_msg_opt, is_pinned, attached_info, is_private_chat, group_at_type, ex, burn_duration,
	min_seq, max_seq, msg_destruct_time, latest_msg_destruct_time, is_msg_destruct, create_time
	FROM conversation`

// ApplyResult is who changed, so the caller can send notices after commit.
type ApplyResult struct {
	// FieldOwners had a settings field written. Private-chat-only writes are not included.
	FieldOwners []string
	// PrivateNotes are single-chat owners whose private-chat flag was written. Peer is the other user.
	PrivateNotes []PrivateNote
}

type PrivateNote struct {
	Owner   string
	Peer    string
	Enabled bool
}

type Spec struct {
	ConversationID   string
	ConversationType int32
	UserID           string
	GroupID          string
	Patch            model.Patch
	// Private is set only for a single chat. Nil leaves the flag unchanged.
	Private *bool
}

type ownerJob struct {
	peer    string
	fields  bool
	private *bool
}

// ApplyConversations writes one partial update for each owner.
// A missing row is inserted. An existing row keeps its type and peer.
// min_seq and max_seq are not part of Spec. Owners are locked in id order.
func (s *Store) ApplyConversations(ctx context.Context, owners []string, spec Spec, now time.Time) (ApplyResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ApplyResult{}, err
	}
	defer tx.Rollback()

	jobs := map[string]ownerJob{}
	for _, owner := range owners {
		job := jobs[owner]
		job.peer = spec.UserID
		job.fields = true
		if spec.Private != nil && spec.ConversationType == model.ConversationSingle {
			job.private = spec.Private
		}
		jobs[owner] = job
	}
	if spec.Private != nil && spec.ConversationType == model.ConversationSingle && spec.UserID != "" {
		counterparty := ""
		sortedOwners := append([]string(nil), owners...)
		sort.Strings(sortedOwners)
		for _, owner := range sortedOwners {
			if owner != spec.UserID {
				counterparty = owner
			}
		}
		if counterparty != "" {
			job := jobs[spec.UserID]
			if job.peer == "" {
				job.peer = counterparty
			}
			job.private = spec.Private
			jobs[spec.UserID] = job
		}
	}

	order := make([]string, 0, len(jobs))
	for owner := range jobs {
		order = append(order, owner)
	}
	sort.Strings(order)
	var result ApplyResult
	noted := map[string]struct{}{}
	peerChanged := false
	for _, owner := range order {
		job := jobs[owner]
		wroteField, wrotePrivate, err := applyOwner(ctx, tx, owner, job, spec, now)
		if err != nil {
			return ApplyResult{}, err
		}
		if wroteField {
			result.FieldOwners = append(result.FieldOwners, owner)
		}
		if wrotePrivate && job.fields && owner != spec.UserID {
			result.PrivateNotes = append(result.PrivateNotes, PrivateNote{Owner: owner, Peer: spec.UserID, Enabled: *job.private})
			noted[owner] = struct{}{}
		}
		if wrotePrivate && !job.fields {
			peerChanged = true
		}
	}
	if peerChanged && spec.Private != nil {
		for _, owner := range owners {
			if owner == spec.UserID {
				continue
			}
			if _, ok := noted[owner]; ok {
				continue
			}
			result.PrivateNotes = append(result.PrivateNotes, PrivateNote{Owner: owner, Peer: spec.UserID, Enabled: *spec.Private})
		}
	}
	if err := tx.Commit(); err != nil {
		return ApplyResult{}, err
	}
	return result, nil
}

func applyOwner(ctx context.Context, tx *sql.Tx, owner string, job ownerJob, spec Spec, now time.Time) (field, private bool, err error) {
	row, err := lockConversation(ctx, tx, owner, spec.ConversationID)
	if err != nil {
		return false, false, err
	}
	touchPrivate := job.private != nil
	privateValue := false
	if touchPrivate {
		privateValue = *job.private
	}
	if row == nil {
		if job.fields && spec.Patch.Empty() && !touchPrivate {
			return false, false, nil
		}
		if !job.fields && !touchPrivate {
			return false, false, nil
		}
		created := newConversation(owner, job.peer, spec, now)
		if touchPrivate {
			created.IsPrivateChat = privateValue
		}
		if err := insertConversation(ctx, tx, created); err != nil {
			return false, false, err
		}
		if err := incrConversation(ctx, tx, owner, spec.ConversationID, model.VersionInsert, now); err != nil {
			return false, false, err
		}
		return job.fields && !spec.Patch.Empty(), touchPrivate, nil
	}
	var sets []string
	var args []any
	if job.fields {
		sets, args = fieldSets(row, spec.Patch)
	}
	if touchPrivate && row.IsPrivateChat != privateValue {
		sets = append(sets, "is_private_chat = ?")
		args = append(args, boolArg(privateValue))
		private = true
	}
	if len(sets) == 0 {
		return false, false, nil
	}
	args = append(args, owner, spec.ConversationID)
	if _, err := tx.ExecContext(ctx, "UPDATE conversation SET "+strings.Join(sets, ", ")+" WHERE owner_user_id = ? AND conversation_id = ?", args...); err != nil {
		return false, false, err
	}
	if err := incrConversation(ctx, tx, owner, spec.ConversationID, model.VersionUpdate, now); err != nil {
		return false, false, err
	}
	return len(sets) > 0 && (!private || len(sets) > 1), private, nil
}

func newConversation(owner, peer string, spec Spec, now time.Time) *model.Conversation {
	row := &model.Conversation{
		OwnerUserID:      owner,
		ConversationID:   spec.ConversationID,
		ConversationType: spec.ConversationType,
		UserID:           peer,
		GroupID:          spec.GroupID,
		CreateTime:       now,
	}
	applyPatch(row, spec.Patch)
	return row
}

func applyPatch(row *model.Conversation, patch model.Patch) {
	if patch.RecvMsgOpt != nil {
		row.RecvMsgOpt = *patch.RecvMsgOpt
	}
	if patch.IsPinned != nil {
		row.IsPinned = *patch.IsPinned
	}
	if patch.AttachedInfo != nil {
		row.AttachedInfo = *patch.AttachedInfo
	}
	if patch.Ex != nil {
		row.Ex = *patch.Ex
	}
	if patch.GroupAtType != nil {
		row.GroupAtType = *patch.GroupAtType
	}
	if patch.MsgDestructTime != nil {
		row.MsgDestructTime = *patch.MsgDestructTime
	}
	if patch.IsMsgDestruct != nil {
		row.IsMsgDestruct = *patch.IsMsgDestruct
	}
	if patch.BurnDuration != nil {
		row.BurnDuration = *patch.BurnDuration
	}
}

func fieldSets(row *model.Conversation, patch model.Patch) ([]string, []any) {
	var sets []string
	var args []any
	if patch.RecvMsgOpt != nil && row.RecvMsgOpt != *patch.RecvMsgOpt {
		sets = append(sets, "recv_msg_opt = ?")
		args = append(args, *patch.RecvMsgOpt)
	}
	if patch.IsPinned != nil && row.IsPinned != *patch.IsPinned {
		sets = append(sets, "is_pinned = ?")
		args = append(args, boolArg(*patch.IsPinned))
	}
	if patch.AttachedInfo != nil && row.AttachedInfo != *patch.AttachedInfo {
		sets = append(sets, "attached_info = ?")
		args = append(args, *patch.AttachedInfo)
	}
	if patch.Ex != nil && row.Ex != *patch.Ex {
		sets = append(sets, "ex = ?")
		args = append(args, *patch.Ex)
	}
	if patch.GroupAtType != nil && row.GroupAtType != *patch.GroupAtType {
		sets = append(sets, "group_at_type = ?")
		args = append(args, *patch.GroupAtType)
	}
	if patch.MsgDestructTime != nil && row.MsgDestructTime != *patch.MsgDestructTime {
		sets = append(sets, "msg_destruct_time = ?")
		args = append(args, *patch.MsgDestructTime)
	}
	if patch.IsMsgDestruct != nil && row.IsMsgDestruct != *patch.IsMsgDestruct {
		sets = append(sets, "is_msg_destruct = ?")
		args = append(args, boolArg(*patch.IsMsgDestruct))
	}
	if patch.BurnDuration != nil && row.BurnDuration != *patch.BurnDuration {
		sets = append(sets, "burn_duration = ?")
		args = append(args, *patch.BurnDuration)
	}
	return sets, args
}

func boolArg(v bool) int {
	if v {
		return 1
	}
	return 0
}

func lockConversation(ctx context.Context, tx *sql.Tx, owner, conversationID string) (*model.Conversation, error) {
	row := tx.QueryRowContext(ctx, conversationSelect+" WHERE owner_user_id = ? AND conversation_id = ? FOR UPDATE", owner, conversationID)
	conversation, err := scanConversation(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return conversation, nil
}

func insertConversation(ctx context.Context, tx *sql.Tx, row *model.Conversation) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO conversation (
		owner_user_id, conversation_id, conversation_type, user_id, group_id,
		recv_msg_opt, is_pinned, attached_info, is_private_chat, group_at_type, ex, burn_duration,
		min_seq, max_seq, msg_destruct_time, latest_msg_destruct_time, is_msg_destruct, create_time
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.OwnerUserID, row.ConversationID, row.ConversationType, row.UserID, row.GroupID,
		row.RecvMsgOpt, boolArg(row.IsPinned), row.AttachedInfo, boolArg(row.IsPrivateChat), row.GroupAtType, row.Ex, row.BurnDuration,
		row.MinSeq, row.MaxSeq, row.MsgDestructTime, nullTime(row.LatestMsgDestructTime), boolArg(row.IsMsgDestruct), row.CreateTime.UTC(),
	)
	return err
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC()
}

func (s *Store) ConversationIDs(ctx context.Context, owner string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT conversation_id FROM conversation WHERE owner_user_id = ? ORDER BY conversation_id ASC`, owner)
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

func (s *Store) ConversationsByID(ctx context.Context, owner string, ids []string) ([]*model.Conversation, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(ids)+1)
	args = append(args, owner)
	for _, id := range ids {
		args = append(args, id)
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	rows, err := s.db.QueryContext(ctx, conversationSelect+` WHERE owner_user_id = ? AND conversation_id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	found := map[string]*model.Conversation{}
	for rows.Next() {
		conversation, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		found[conversation.ConversationID] = conversation
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]*model.Conversation, 0, len(ids))
	for _, id := range ids {
		if conversation, ok := found[id]; ok {
			out = append(out, conversation)
		}
	}
	return out, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanConversation(row scanner) (*model.Conversation, error) {
	var conversation model.Conversation
	var pinned, private, destruct int
	var attached sql.NullString
	var latest sql.NullTime
	var created sql.NullTime
	err := row.Scan(
		&conversation.OwnerUserID, &conversation.ConversationID, &conversation.ConversationType, &conversation.UserID, &conversation.GroupID,
		&conversation.RecvMsgOpt, &pinned, &attached, &private, &conversation.GroupAtType, &conversation.Ex, &conversation.BurnDuration,
		&conversation.MinSeq, &conversation.MaxSeq, &conversation.MsgDestructTime, &latest, &destruct, &created,
	)
	if err != nil {
		return nil, err
	}
	conversation.IsPinned = pinned == 1
	conversation.IsPrivateChat = private == 1
	conversation.IsMsgDestruct = destruct == 1
	if attached.Valid {
		conversation.AttachedInfo = attached.String
	}
	if latest.Valid {
		conversation.LatestMsgDestructTime = latest.Time
	}
	if created.Valid {
		conversation.CreateTime = created.Time
	}
	return &conversation, nil
}
