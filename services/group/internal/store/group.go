package store

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

func (s *Store) GroupExists(ctx context.Context, groupID string) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, "SELECT 1 FROM `group` WHERE group_id = ?", groupID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) TakeGroup(ctx context.Context, groupID string) (*model.Group, error) {
	if s.cache == nil {
		return s.takeGroup(ctx, groupID)
	}
	group, err := s.cache.groups.Get(ctx, groupKey(groupID), func(ctx context.Context) (model.Group, error) {
		group, err := s.takeGroup(ctx, groupID)
		if err != nil {
			return model.Group{}, err
		}
		return *group, nil
	})
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func (s *Store) takeGroup(ctx context.Context, groupID string) (*model.Group, error) {
	row := s.db.QueryRowContext(ctx, groupSelect+" WHERE group_id = ?", groupID)
	g, err := scanGroup(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := s.fillOwnersAndCounts(ctx, []*model.Group{g}); err != nil {
		return nil, err
	}
	return g, nil
}

func (s *Store) FindGroups(ctx context.Context, groupIDs []string) ([]*model.Group, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	if s.cache == nil {
		return s.findGroups(ctx, groupIDs)
	}
	var once sync.Once
	load := loadOnce(&once, func() (map[string]model.Group, error) {
		groups, err := s.findGroups(ctx, groupIDs)
		if err != nil {
			return nil, err
		}
		loaded := make(map[string]model.Group, len(groups))
		for _, group := range groups {
			loaded[group.GroupID] = *group
		}
		return loaded, nil
	})
	out := make([]*model.Group, 0, len(groupIDs))
	seen := map[string]struct{}{}
	for _, id := range groupIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		group, err := s.cache.groups.Get(ctx, groupKey(id), func(ctx context.Context) (model.Group, error) {
			loaded, err := load()
			if err != nil {
				return model.Group{}, err
			}
			group, ok := loaded[id]
			if !ok {
				return model.Group{}, ErrNotFound
			}
			return group, nil
		})
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, &group)
	}
	return out, nil
}

func (s *Store) findGroups(ctx context.Context, groupIDs []string) ([]*model.Group, error) {
	args := make([]any, len(groupIDs))
	for i, id := range groupIDs {
		args[i] = id
	}
	rows, err := s.db.QueryContext(ctx, groupSelect+" WHERE group_id IN ("+placeholders(len(groupIDs))+")", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[string]*model.Group{}
	var groups []*model.Group
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		byID[g.GroupID] = g
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.fillOwnersAndCounts(ctx, groups); err != nil {
		return nil, err
	}
	ordered := make([]*model.Group, 0, len(groupIDs))
	seen := map[string]struct{}{}
	for _, id := range groupIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if g, ok := byID[id]; ok {
			ordered = append(ordered, g)
		}
	}
	return ordered, nil
}

func (s *Store) CreateGroup(ctx context.Context, group *model.Group, members []*model.Member, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var notifTime any
	if !group.NotificationUpdateTime.IsZero() {
		notifTime = group.NotificationUpdateTime.UTC()
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO `+"`group`"+` (
			group_id, group_name, notification, introduction, face_url, create_time, status,
			creator_user_id, group_type, need_verification, look_member_info, apply_member_friend,
			notification_update_time, notification_user_id, ex
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		group.GroupID, group.GroupName, group.Notification, group.Introduction, group.FaceURL, group.CreateTime.UTC(), group.Status,
		group.CreatorUserID, group.GroupType, group.NeedVerification, group.LookMemberInfo, group.ApplyMemberFriend,
		notifTime, group.NotificationUserID, group.Ex); err != nil {
		return err
	}
	if err := insertMembers(ctx, tx, members); err != nil {
		return err
	}
	if err := writeJoin(ctx, tx, members, model.VersionInsert, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if s.cache != nil {
		s.cache.dropMembers(ctx, group.GroupID, memberUserIDsOf(members)...)
	}
	return nil
}

func (s *Store) AddMembers(ctx context.Context, groupID string, members []*model.Member, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

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
	if err := insertMembers(ctx, tx, members); err != nil {
		return err
	}
	if err := writeJoin(ctx, tx, members, model.VersionInsert, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if s.cache != nil {
		s.cache.dropMembers(ctx, groupID, memberUserIDsOf(members)...)
	}
	return nil
}

func (s *Store) DeleteMember(ctx context.Context, groupID, userID string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, "DELETE FROM group_member WHERE group_id = ? AND user_id = ?", groupID, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotMember
	}
	if err := incrMembers(ctx, tx, groupID, []string{userID}, model.VersionDelete, now); err != nil {
		return err
	}
	if err := incrJoin(ctx, tx, userID, groupID, model.VersionDelete, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if s.cache != nil {
		s.cache.dropMembers(ctx, groupID, userID)
	}
	return nil
}

func (s *Store) TransferOwner(ctx context.Context, groupID, oldOwner, newOwner string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

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

	type row struct {
		role int32
		mute sql.NullTime
	}
	found := map[string]row{}
	ids := []string{oldOwner, newOwner}
	sort.Strings(ids)
	rows, err := tx.QueryContext(ctx, "SELECT user_id, role_level, mute_end_time FROM group_member WHERE group_id = ? AND user_id IN (?, ?) FOR UPDATE", groupID, ids[0], ids[1])
	if err != nil {
		return err
	}
	for rows.Next() {
		var userID string
		var item row
		if err := rows.Scan(&userID, &item.role, &item.mute); err != nil {
			rows.Close()
			return err
		}
		found[userID] = item
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	oldRow, okOld := found[oldOwner]
	newRow, okNew := found[newOwner]
	if !okOld || !okNew {
		return ErrNotMember
	}
	if oldRow.role != model.RoleOwner {
		return ErrNotMember
	}

	if _, err := tx.ExecContext(ctx, "UPDATE group_member SET role_level = ? WHERE group_id = ? AND user_id = ?", newRow.role, groupID, oldOwner); err != nil {
		return err
	}
	if newRow.mute.Valid && newRow.mute.Time.After(now) {
		if _, err := tx.ExecContext(ctx, "UPDATE group_member SET role_level = ?, mute_end_time = NULL WHERE group_id = ? AND user_id = ?", model.RoleOwner, groupID, newOwner); err != nil {
			return err
		}
	} else if _, err := tx.ExecContext(ctx, "UPDATE group_member SET role_level = ? WHERE group_id = ? AND user_id = ?", model.RoleOwner, groupID, newOwner); err != nil {
		return err
	}
	if err := incrMembers(ctx, tx, groupID, []string{oldOwner, newOwner, model.SortElemID}, model.VersionUpdate, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if s.cache != nil {
		s.cache.dropMembers(ctx, groupID, oldOwner, newOwner)
	}
	return nil
}

func (s *Store) UpdateGroup(ctx context.Context, groupID string, patch model.GroupPatch, now time.Time) error {
	sets := make([]string, 0, 10)
	args := make([]any, 0, 12)
	if patch.Name != nil {
		sets = append(sets, "group_name = ?")
		args = append(args, *patch.Name)
	}
	if patch.Notification != nil {
		sets = append(sets, "notification = ?", "notification_update_time = ?", "notification_user_id = ?")
		args = append(args, *patch.Notification, patch.NotificationTime.UTC(), patch.NotificationUserID)
	}
	if patch.Introduction != nil {
		sets = append(sets, "introduction = ?")
		args = append(args, *patch.Introduction)
	}
	if patch.FaceURL != nil {
		sets = append(sets, "face_url = ?")
		args = append(args, *patch.FaceURL)
	}
	if patch.NeedVerification != nil {
		sets = append(sets, "need_verification = ?")
		args = append(args, *patch.NeedVerification)
	}
	if patch.LookMemberInfo != nil {
		sets = append(sets, "look_member_info = ?")
		args = append(args, *patch.LookMemberInfo)
	}
	if patch.ApplyMemberFriend != nil {
		sets = append(sets, "apply_member_friend = ?")
		args = append(args, *patch.ApplyMemberFriend)
	}
	if patch.Ex != nil {
		sets = append(sets, "ex = ?")
		args = append(args, *patch.Ex)
	}
	if len(sets) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

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
	args = append(args, groupID)
	if _, err := tx.ExecContext(ctx, "UPDATE `group` SET "+strings.Join(sets, ", ")+" WHERE group_id = ?", args...); err != nil {
		return err
	}
	if err := incrMembers(ctx, tx, groupID, []string{""}, model.VersionUpdate, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if s.cache != nil {
		s.cache.dropGroupInfo(ctx, groupID)
	}
	return nil
}

func (s *Store) DismissGroup(ctx context.Context, groupID string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

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
	if _, err := tx.ExecContext(ctx, "UPDATE `group` SET status = ? WHERE group_id = ?", model.GroupStatusDismissed, groupID); err != nil {
		return err
	}
	if err := incrMembers(ctx, tx, groupID, []string{""}, model.VersionUpdate, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if s.cache != nil {
		s.cache.dropGroupInfo(ctx, groupID)
	}
	return nil
}

func (s *Store) JoinedGroupIDs(ctx context.Context, userID string, offset, limit int) (int, []string, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM group_member WHERE user_id = ?", userID).Scan(&total); err != nil {
		return 0, nil, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT group_id FROM group_member WHERE user_id = ? ORDER BY join_time DESC, group_id ASC LIMIT ? OFFSET ?", userID, limit, offset)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, nil, err
		}
		ids = append(ids, id)
	}
	return total, ids, rows.Err()
}

const groupSelect = `SELECT group_id, group_name, notification, introduction, face_url, create_time, status,
	creator_user_id, group_type, need_verification, look_member_info, apply_member_friend,
	notification_update_time, notification_user_id, ex FROM ` + "`group`"

type groupScanner interface {
	Scan(dest ...any) error
}

func scanGroup(row groupScanner) (*model.Group, error) {
	var g model.Group
	var notification sql.NullString
	var notificationTime sql.NullTime
	err := row.Scan(
		&g.GroupID, &g.GroupName, &notification, &g.Introduction, &g.FaceURL, &g.CreateTime, &g.Status,
		&g.CreatorUserID, &g.GroupType, &g.NeedVerification, &g.LookMemberInfo, &g.ApplyMemberFriend,
		&notificationTime, &g.NotificationUserID, &g.Ex,
	)
	if err != nil {
		return nil, err
	}
	g.Notification = notification.String
	if notificationTime.Valid {
		g.NotificationUpdateTime = notificationTime.Time
	}
	return &g, nil
}

func (s *Store) fillOwnersAndCounts(ctx context.Context, groups []*model.Group) error {
	if len(groups) == 0 {
		return nil
	}
	args := make([]any, len(groups))
	for i, g := range groups {
		args[i] = g.GroupID
	}
	in := placeholders(len(groups))
	rows, err := s.db.QueryContext(ctx, "SELECT group_id, COUNT(*) FROM group_member WHERE group_id IN ("+in+") GROUP BY group_id", args...)
	if err != nil {
		return err
	}
	counts := map[string]int32{}
	for rows.Next() {
		var id string
		var n int32
		if err := rows.Scan(&id, &n); err != nil {
			rows.Close()
			return err
		}
		counts[id] = n
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	rows, err = s.db.QueryContext(ctx, "SELECT group_id, user_id FROM group_member WHERE role_level = ? AND group_id IN ("+in+")", append([]any{model.RoleOwner}, args...)...)
	if err != nil {
		return err
	}
	defer rows.Close()
	owners := map[string]string{}
	for rows.Next() {
		var groupID, userID string
		if err := rows.Scan(&groupID, &userID); err != nil {
			return err
		}
		owners[groupID] = userID
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, g := range groups {
		g.MemberCount = counts[g.GroupID]
		g.OwnerUserID = owners[g.GroupID]
	}
	return nil
}

func placeholders(n int) string {
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}
