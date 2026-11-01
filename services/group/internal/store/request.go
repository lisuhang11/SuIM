package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

const requestSelect = `SELECT group_id, user_id, handle_result, req_msg, handled_msg, req_time, handle_user_id, handled_time, join_source, inviter_user_id, ex FROM group_request`

func (s *Store) TakeRequest(ctx context.Context, groupID, userID string) (*model.Request, error) {
	row := s.db.QueryRowContext(ctx, requestSelect+" WHERE group_id = ? AND user_id = ?", groupID, userID)
	req, err := scanRequest(row)
	if err == sql.ErrNoRows {
		return nil, ErrRequestNotFound
	}
	return req, err
}

func (s *Store) ManagedGroupIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT group_id FROM group_member WHERE user_id = ? AND role_level >= ? ORDER BY group_id", userID, model.RoleAdmin)
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

func (s *Store) PageGroupRequests(ctx context.Context, groupIDs []string, handleResults []int32, offset, limit int) (int, []*model.Request, error) {
	if len(groupIDs) == 0 {
		return 0, nil, nil
	}
	where, args := requestWhere("", groupIDs, handleResults)
	return s.pageRequests(ctx, where, args, offset, limit)
}

func (s *Store) PageUserRequests(ctx context.Context, userID string, groupIDs []string, handleResults []int32, offset, limit int) (int, []*model.Request, error) {
	where, args := requestWhere(userID, groupIDs, handleResults)
	return s.pageRequests(ctx, where, args, offset, limit)
}

// HandleRequest closes a pending request. member is inserted in the same transaction when agreeing a user who is not yet in the group.
func (s *Store) HandleRequest(ctx context.Context, groupID, userID, handledMsg string, handleResult int32, handleUserID string, now time.Time, member *model.Member, limit int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if member != nil {
		var status int32
		err = tx.QueryRowContext(ctx, "SELECT status FROM `group` WHERE group_id = ? FOR UPDATE", groupID).Scan(&status)
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if status == model.GroupStatusDismissed {
			return ErrDismissed
		}
		var current int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM group_member WHERE group_id = ?", groupID).Scan(&current); err != nil {
			return err
		}
		if err := roomFor(1, current, limit); err != nil {
			return err
		}
	}
	var handle int32
	err = tx.QueryRowContext(ctx, "SELECT handle_result FROM group_request WHERE group_id = ? AND user_id = ? FOR UPDATE", groupID, userID).Scan(&handle)
	if err == sql.ErrNoRows {
		return ErrRequestNotFound
	}
	if err != nil {
		return err
	}
	if handle != model.HandlePending {
		return ErrRequestHandled
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE group_request
		SET handle_result = ?, handled_msg = ?, handle_user_id = ?, handled_time = ?
		WHERE group_id = ? AND user_id = ?`,
		handleResult, handledMsg, handleUserID, now.UTC(), groupID, userID); err != nil {
		return err
	}
	if member != nil {
		if err := insertMembers(ctx, tx, []*model.Member{member}); err != nil {
			return err
		}
		if err := writeJoin(ctx, tx, []*model.Member{member}, model.VersionInsert, now); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if member != nil && s.cache != nil {
		s.cache.dropMembers(ctx, groupID, member.UserID)
	}
	return nil
}

func (s *Store) pageRequests(ctx context.Context, where string, args []any, offset, limit int) (int, []*model.Request, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM group_request WHERE "+where, args...).Scan(&total); err != nil {
		return 0, nil, err
	}
	rows, err := s.db.QueryContext(ctx, requestSelect+" WHERE "+where+" ORDER BY req_time DESC, group_id ASC, user_id ASC LIMIT ? OFFSET ?", append(append([]any{}, args...), limit, offset)...)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	var requests []*model.Request
	for rows.Next() {
		req, err := scanRequest(rows)
		if err != nil {
			return 0, nil, err
		}
		requests = append(requests, req)
	}
	return total, requests, rows.Err()
}

func requestWhere(userID string, groupIDs []string, handleResults []int32) (string, []any) {
	var conds []string
	var args []any
	if userID != "" {
		conds = append(conds, "user_id = ?")
		args = append(args, userID)
	}
	if len(groupIDs) > 0 {
		conds = append(conds, "group_id IN ("+placeholders(len(groupIDs))+")")
		for _, id := range groupIDs {
			args = append(args, id)
		}
	}
	if len(handleResults) > 0 {
		conds = append(conds, "handle_result IN ("+placeholders(len(handleResults))+")")
		for _, result := range handleResults {
			args = append(args, result)
		}
	}
	where := conds[0]
	for _, cond := range conds[1:] {
		where += " AND " + cond
	}
	return where, args
}

type requestScanner interface {
	Scan(dest ...any) error
}

func scanRequest(row requestScanner) (*model.Request, error) {
	var req model.Request
	var handled sql.NullTime
	err := row.Scan(
		&req.GroupID, &req.UserID, &req.HandleResult, &req.ReqMsg, &req.HandledMsg, &req.ReqTime,
		&req.HandleUserID, &handled, &req.JoinSource, &req.InviterUserID, &req.Ex,
	)
	if err != nil {
		return nil, err
	}
	if handled.Valid {
		req.HandledTime = handled.Time
	}
	return &req, nil
}
