package store

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

func (s *Store) TakeMember(ctx context.Context, groupID, userID string) (*model.Member, error) {
	if s.cache == nil {
		return s.takeMember(ctx, groupID, userID)
	}
	member, err := s.cache.members.Get(ctx, memberKey(groupID, userID), func(ctx context.Context) (model.Member, error) {
		member, err := s.takeMember(ctx, groupID, userID)
		if err != nil {
			return model.Member{}, err
		}
		return *member, nil
	})
	if err != nil {
		return nil, err
	}
	return &member, nil
}

func (s *Store) takeMember(ctx context.Context, groupID, userID string) (*model.Member, error) {
	row := s.db.QueryRowContext(ctx, memberSelect+" WHERE group_id = ? AND user_id = ?", groupID, userID)
	m, err := scanMember(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotMember
	}
	return m, err
}

func (s *Store) FindMembers(ctx context.Context, groupID string, userIDs []string) ([]*model.Member, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	if s.cache == nil {
		return s.findMembers(ctx, groupID, userIDs)
	}
	var once sync.Once
	load := loadOnce(&once, func() (map[string]model.Member, error) {
		members, err := s.findMembers(ctx, groupID, userIDs)
		if err != nil {
			return nil, err
		}
		loaded := make(map[string]model.Member, len(members))
		for _, member := range members {
			loaded[member.UserID] = *member
		}
		return loaded, nil
	})
	out := make([]*model.Member, 0, len(userIDs))
	seen := map[string]struct{}{}
	for _, id := range userIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		member, err := s.cache.members.Get(ctx, memberKey(groupID, id), func(ctx context.Context) (model.Member, error) {
			loaded, err := load()
			if err != nil {
				return model.Member{}, err
			}
			member, ok := loaded[id]
			if !ok {
				return model.Member{}, ErrNotMember
			}
			return member, nil
		})
		if errors.Is(err, ErrNotMember) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, &member)
	}
	return out, nil
}

func (s *Store) findMembers(ctx context.Context, groupID string, userIDs []string) ([]*model.Member, error) {
	args := make([]any, 0, len(userIDs)+1)
	args = append(args, groupID)
	for _, id := range userIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, memberSelect+" WHERE group_id = ? AND user_id IN ("+placeholders(len(userIDs))+")", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMembers(rows)
}

func (s *Store) PageMembers(ctx context.Context, groupID string, offset, limit int) (int, []*model.Member, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM group_member WHERE group_id = ?", groupID).Scan(&total); err != nil {
		return 0, nil, err
	}
	rows, err := s.db.QueryContext(ctx, memberSelect+" WHERE group_id = ? ORDER BY role_level DESC, join_time ASC, user_id ASC LIMIT ? OFFSET ?", groupID, limit, offset)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	members, err := scanMembers(rows)
	return total, members, err
}

func (s *Store) MemberUserIDs(ctx context.Context, groupID string) ([]string, error) {
	if s.cache == nil {
		return s.memberUserIDs(ctx, groupID)
	}
	ids, err := s.cache.memberIDs.Get(ctx, memberIDsKey(groupID), func(ctx context.Context) ([]string, error) {
		return s.memberUserIDs(ctx, groupID)
	})
	if err != nil {
		return nil, err
	}
	return copyIDs(ids), nil
}

func (s *Store) memberUserIDs(ctx context.Context, groupID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT user_id FROM group_member WHERE group_id = ? ORDER BY role_level DESC, join_time ASC, user_id ASC", groupID)
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

func (s *Store) CreateRequests(ctx context.Context, requests []*model.Request) error {
	if len(requests) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	sorted := append([]*model.Request(nil), requests...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].GroupID == sorted[j].GroupID {
			return sorted[i].UserID < sorted[j].UserID
		}
		return sorted[i].GroupID < sorted[j].GroupID
	})
	for _, req := range sorted {
		var handle int32
		err := tx.QueryRowContext(ctx, "SELECT handle_result FROM group_request WHERE group_id = ? AND user_id = ? FOR UPDATE", req.GroupID, req.UserID).Scan(&handle)
		if err == sql.ErrNoRows {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO group_request (group_id, user_id, handle_result, req_msg, handled_msg, req_time, handle_user_id, handled_time, join_source, inviter_user_id, ex)
				VALUES (?, ?, 0, ?, '', ?, '', NULL, ?, ?, '')`,
				req.GroupID, req.UserID, req.ReqMsg, req.ReqTime.UTC(), req.JoinSource, req.InviterUserID); err != nil {
				if isDuplicate(err) {
					return ErrRequestRepeated
				}
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if handle == model.HandlePending {
			return ErrRequestRepeated
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE group_request
			SET handle_result = 0, req_msg = ?, handled_msg = '', req_time = ?, handle_user_id = '', handled_time = NULL, join_source = ?, inviter_user_id = ?
			WHERE group_id = ? AND user_id = ?`,
			req.ReqMsg, req.ReqTime.UTC(), req.JoinSource, req.InviterUserID, req.GroupID, req.UserID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const memberSelect = `SELECT group_id, user_id, nickname, face_url, role_level, join_time, join_source, inviter_user_id, operator_user_id, mute_end_time, ex FROM group_member`

func insertMembers(ctx context.Context, tx *sql.Tx, members []*model.Member) error {
	sorted := append([]*model.Member(nil), members...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].UserID < sorted[j].UserID })
	userIDs := make([]string, 0, len(sorted))
	for _, m := range sorted {
		var mute any
		if !m.MuteEndTime.IsZero() {
			mute = m.MuteEndTime.UTC()
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO group_member (
				group_id, user_id, nickname, face_url, role_level, join_time, join_source,
				inviter_user_id, operator_user_id, mute_end_time, ex
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			m.GroupID, m.UserID, m.Nickname, m.FaceURL, m.RoleLevel, m.JoinTime.UTC(), m.JoinSource,
			m.InviterUserID, m.OperatorUserID, mute, m.Ex); err != nil {
			if isDuplicate(err) {
				return ErrAlreadyMember
			}
			return err
		}
		userIDs = append(userIDs, m.UserID)
	}
	if len(sorted) == 0 {
		return nil
	}
	return incrMembers(ctx, tx, sorted[0].GroupID, userIDs, model.VersionInsert, sorted[0].JoinTime)
}

func writeJoin(ctx context.Context, tx *sql.Tx, members []*model.Member, state int32, now time.Time) error {
	sorted := append([]*model.Member(nil), members...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].UserID < sorted[j].UserID })
	for _, m := range sorted {
		if err := incrJoin(ctx, tx, m.UserID, m.GroupID, state, now); err != nil {
			return err
		}
	}
	return nil
}

type memberScanner interface {
	Scan(dest ...any) error
}

func scanMember(row memberScanner) (*model.Member, error) {
	var m model.Member
	var mute sql.NullTime
	err := row.Scan(
		&m.GroupID, &m.UserID, &m.Nickname, &m.FaceURL, &m.RoleLevel, &m.JoinTime, &m.JoinSource,
		&m.InviterUserID, &m.OperatorUserID, &mute, &m.Ex,
	)
	if err != nil {
		return nil, err
	}
	if mute.Valid {
		m.MuteEndTime = mute.Time
	}
	return &m, nil
}

func scanMembers(rows *sql.Rows) ([]*model.Member, error) {
	var members []*model.Member
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}
