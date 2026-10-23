package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/idhash"
	"github.com/lisuhang11/SuIM/proto/grouppb"
	"github.com/lisuhang11/SuIM/services/group/internal/logic"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
	"github.com/lisuhang11/SuIM/services/group/internal/store"
)

type knownUsers map[string]struct{}

func (u knownUsers) GetUsers(_ context.Context, ids []string) (map[string]model.UserProfile, error) {
	out := make(map[string]model.UserProfile, len(ids))
	for _, id := range ids {
		if _, ok := u[id]; !ok {
			continue
		}
		out[id] = model.UserProfile{UserID: id, Nickname: id, AvatarURL: id + ".png"}
	}
	return out, nil
}

func TestGroup(t *testing.T) {
	ctx := context.Background()
	env := readEnvFile(t, filepath.Join("..", "..", ".env"))
	mysqlCfg := config.MySQL{
		Address:     []string{"127.0.0.1:3306"},
		Username:    env["MYSQL_USER"],
		Password:    env["MYSQL_PASSWORD"],
		Database:    env["MYSQL_DATABASE"],
		MaxOpenConn: 5,
		MaxIdleConn: 2,
	}
	st, err := store.Open(ctx, mysqlDSN(mysqlCfg), mysqlCfg.MaxOpenConn, mysqlCfg.MaxIdleConn)
	if err != nil {
		t.Skipf("mysql is not reachable: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	script, err := os.ReadFile(filepath.Join("..", "..", "migrations", "group.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx, string(script)); err != nil {
		t.Fatal(err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	a := "ga-" + suffix
	b := "gb-" + suffix
	c := "gc-" + suffix
	d := "gd-" + suffix
	t.Cleanup(func() { cleanupGroup(t, mysqlCfg, a, b, c, d) })

	srv := logic.New(st, knownUsers{a: {}, b: {}, c: {}, d: {}})

	if _, err := srv.CreateGroup(ctx, &grouppb.CreateGroupReq{CreatorUserId: a, GroupName: "n", GroupType: 3}); !isCode(err, codes.InvalidArgument, errorcode.GroupTypeUnsupported) {
		t.Fatalf("type code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.CreateGroup(ctx, &grouppb.CreateGroupReq{CreatorUserId: "bad:id", GroupName: "n", GroupType: model.GroupTypeSuper}); !isCode(err, codes.InvalidArgument, errorcode.GroupInvalidUserID) {
		t.Fatalf("bad id code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.CreateGroup(ctx, &grouppb.CreateGroupReq{CreatorUserId: a, GroupName: "n", GroupType: model.GroupTypeSuper, MemberIds: []string{"missing"}}); !isCode(err, codes.NotFound, errorcode.GroupUserNotFound) {
		t.Fatalf("missing user code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.CreateGroup(ctx, &grouppb.CreateGroupReq{CreatorUserId: a, GroupName: "n", GroupType: model.GroupTypeSuper, MemberIds: []string{a}}); !isCode(err, codes.InvalidArgument, errorcode.GroupMemberRepeated) {
		t.Fatalf("owner repeated code=%v err=%v", status.Code(err), err)
	}

	created, err := srv.CreateGroup(ctx, &grouppb.CreateGroupReq{
		CreatorUserId:    a,
		GroupName:        "room",
		Notification:     "hello",
		GroupType:        model.GroupTypeSuper,
		NeedVerification: model.NeedVerifyInviteDirect,
		MemberIds:        []string{b},
	})
	if err != nil {
		t.Fatal(err)
	}
	gid := created.GetGroupId()
	if gid == "" || created.GetGroup().GetOwnerUserId() != a || created.GetGroup().GetMemberCount() != 2 || created.GetGroup().GetNotificationUserId() != a {
		t.Fatalf("created %+v", created.GetGroup())
	}

	got := oneGroup(t, srv, ctx, gid)
	if got.GetGroupName() != "room" {
		t.Fatalf("get %+v", got)
	}
	missing, err := srv.GetGroupsInfo(ctx, &grouppb.GetGroupsInfoReq{GroupIds: []string{"missing-group"}})
	if err != nil || len(missing.GetGroups()) != 0 {
		t.Fatalf("missing group %+v err=%v", missing, err)
	}
	listed, err := srv.GetGroupsInfo(ctx, &grouppb.GetGroupsInfoReq{GroupIds: []string{gid, "missing-group", gid}})
	if err != nil || len(listed.GetGroups()) != 1 {
		t.Fatalf("groups %+v err=%v", listed, err)
	}

	members, err := srv.GetGroupMembers(ctx, &grouppb.GetGroupMembersReq{GroupId: gid, Limit: 1})
	if err != nil || members.GetTotal() != 2 || len(members.GetMembers()) != 1 || members.GetMembers()[0].GetRoleLevel() != model.RoleOwner || members.GetMembers()[0].GetNickname() != a {
		t.Fatalf("members %+v err=%v", members, err)
	}
	ids, err := srv.GetGroupMemberUserIDs(ctx, &grouppb.GetGroupMemberUserIDsReq{GroupId: gid})
	if err != nil || len(ids.GetUserIds()) != 2 || ids.GetUserIds()[0] != a {
		t.Fatalf("ids %+v err=%v", ids, err)
	}

	if _, err := srv.InviteUserToGroup(ctx, &grouppb.InviteUserToGroupReq{GroupId: gid, OpUserId: b, UserIds: []string{c}}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.InviteUserToGroup(ctx, &grouppb.InviteUserToGroupReq{GroupId: gid, OpUserId: a, UserIds: []string{c}}); !isCode(err, codes.AlreadyExists, errorcode.GroupAlreadyMember) {
		t.Fatalf("already member code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.InviteUserToGroup(ctx, &grouppb.InviteUserToGroupReq{GroupId: gid, OpUserId: d, UserIds: []string{}}); !isCode(err, codes.InvalidArgument, errorcode.GroupInvalidArgs) {
		t.Fatalf("empty invite code=%v err=%v", status.Code(err), err)
	}

	joined, err := srv.GetJoinedGroups(ctx, &grouppb.GetJoinedGroupsReq{UserId: b, Limit: 10})
	if err != nil || joined.GetTotal() != 1 || joined.GetGroups()[0].GetGroupId() != gid {
		t.Fatalf("joined %+v err=%v", joined, err)
	}

	verified, err := srv.CreateGroup(ctx, &grouppb.CreateGroupReq{
		CreatorUserId:    a,
		GroupName:        "verify",
		GroupType:        model.GroupTypeSuper,
		NeedVerification: model.NeedVerifyAll,
		MemberIds:        []string{b},
	})
	if err != nil {
		t.Fatal(err)
	}
	vgid := verified.GetGroupId()
	if _, err := srv.InviteUserToGroup(ctx, &grouppb.InviteUserToGroupReq{GroupId: vgid, OpUserId: b, UserIds: []string{c}, Reason: "please"}); err != nil {
		t.Fatal(err)
	}
	afterReq, err := srv.GetGroupMemberUserIDs(ctx, &grouppb.GetGroupMemberUserIDsReq{GroupId: vgid})
	if err != nil || len(afterReq.GetUserIds()) != 2 {
		t.Fatalf("request path still two members %+v err=%v", afterReq, err)
	}
	if _, err := srv.InviteUserToGroup(ctx, &grouppb.InviteUserToGroupReq{GroupId: vgid, OpUserId: b, UserIds: []string{c}}); !isCode(err, codes.AlreadyExists, errorcode.GroupRequestRepeated) {
		t.Fatalf("pending request code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.InviteUserToGroup(ctx, &grouppb.InviteUserToGroupReq{GroupId: vgid, OpUserId: a, UserIds: []string{d}}); err != nil {
		t.Fatal(err)
	}

	if _, err := srv.KickGroupMember(ctx, &grouppb.KickGroupMemberReq{GroupId: gid, OpUserId: b, UserId: c}); !isCode(err, codes.PermissionDenied, errorcode.GroupNoPermission) {
		t.Fatalf("ordinary kick code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.KickGroupMember(ctx, &grouppb.KickGroupMemberReq{GroupId: gid, OpUserId: a, UserId: a}); !isCode(err, codes.InvalidArgument, errorcode.GroupKickSelf) {
		t.Fatalf("kick self code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.KickGroupMember(ctx, &grouppb.KickGroupMemberReq{GroupId: gid, OpUserId: a, UserId: c}); err != nil {
		t.Fatal(err)
	}

	if err := setRole(t, mysqlCfg, gid, b, model.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.InviteUserToGroup(ctx, &grouppb.InviteUserToGroupReq{GroupId: gid, OpUserId: a, UserIds: []string{c}}); err != nil {
		t.Fatal(err)
	}
	if err := setRole(t, mysqlCfg, gid, c, model.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.KickGroupMember(ctx, &grouppb.KickGroupMemberReq{GroupId: gid, OpUserId: b, UserId: c}); !isCode(err, codes.PermissionDenied, errorcode.GroupNoPermission) {
		t.Fatalf("admin kick admin code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.InviteUserToGroup(ctx, &grouppb.InviteUserToGroupReq{GroupId: gid, OpUserId: a, UserIds: []string{d}}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.KickGroupMember(ctx, &grouppb.KickGroupMemberReq{GroupId: gid, OpUserId: b, UserId: d}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.KickGroupMember(ctx, &grouppb.KickGroupMemberReq{GroupId: gid, OpUserId: b, UserId: a}); !isCode(err, codes.InvalidArgument, errorcode.GroupOwnerKick) {
		t.Fatalf("kick owner code=%v err=%v", status.Code(err), err)
	}

	if _, err := srv.QuitGroup(ctx, &grouppb.QuitGroupReq{GroupId: gid, UserId: a}); !isCode(err, codes.PermissionDenied, errorcode.GroupOwnerQuit) {
		t.Fatalf("owner quit code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.QuitGroup(ctx, &grouppb.QuitGroupReq{GroupId: gid, UserId: b}); err != nil {
		t.Fatal(err)
	}

	if err := setMute(t, mysqlCfg, gid, c); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.TransferGroupOwner(ctx, &grouppb.TransferGroupOwnerReq{GroupId: gid, OpUserId: a, NewOwnerUserId: a}); !isCode(err, codes.InvalidArgument, errorcode.GroupSameOwner) {
		t.Fatalf("same owner code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.TransferGroupOwner(ctx, &grouppb.TransferGroupOwnerReq{GroupId: gid, OpUserId: a, NewOwnerUserId: c}); err != nil {
		t.Fatal(err)
	}
	transferred := oneGroup(t, srv, ctx, gid)
	if transferred.GetOwnerUserId() != c {
		t.Fatalf("transferred %+v", transferred)
	}
	roleA, muteC, err := memberState(t, mysqlCfg, gid, a, c)
	if err != nil {
		t.Fatal(err)
	}
	if roleA != model.RoleAdmin || muteC {
		t.Fatalf("roleA=%d muteC=%v", roleA, muteC)
	}

	if _, err := srv.UpdateGroupInfo(ctx, &grouppb.UpdateGroupInfoReq{GroupId: gid, OpUserId: c}); !isCode(err, codes.InvalidArgument, errorcode.GroupNothingToUpdate) {
		t.Fatalf("empty update code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.UpdateGroupInfo(ctx, &grouppb.UpdateGroupInfoReq{GroupId: gid, OpUserId: b, GroupName: strPtr("nope")}); !isCode(err, codes.PermissionDenied, errorcode.GroupNoPermission) {
		t.Fatalf("outsider update code=%v err=%v", status.Code(err), err)
	}
	renamed, err := srv.UpdateGroupInfo(ctx, &grouppb.UpdateGroupInfoReq{GroupId: gid, OpUserId: c, GroupName: strPtr("renamed")})
	if err != nil || renamed.GetGroup().GetGroupName() != "renamed" {
		t.Fatalf("rename %+v err=%v", renamed, err)
	}

	if _, err := srv.DismissGroup(ctx, &grouppb.DismissGroupReq{GroupId: gid, OpUserId: a}); !isCode(err, codes.PermissionDenied, errorcode.GroupNoPermission) {
		t.Fatalf("non owner dismiss code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.DismissGroup(ctx, &grouppb.DismissGroupReq{GroupId: gid, OpUserId: c}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.DismissGroup(ctx, &grouppb.DismissGroupReq{GroupId: gid, OpUserId: c}); !isCode(err, codes.FailedPrecondition, errorcode.GroupDismissed) {
		t.Fatalf("dismiss again code=%v err=%v", status.Code(err), err)
	}
	dismissed := oneGroup(t, srv, ctx, gid)
	if dismissed.GetStatus() != model.GroupStatusDismissed || dismissed.GetMemberCount() == 0 {
		t.Fatalf("dismissed group %+v", dismissed)
	}
	if _, err := srv.InviteUserToGroup(ctx, &grouppb.InviteUserToGroupReq{GroupId: gid, OpUserId: c, UserIds: []string{b}}); !isCode(err, codes.FailedPrecondition, errorcode.GroupDismissed) {
		t.Fatalf("invite dismissed code=%v err=%v", status.Code(err), err)
	}
}

func TestGroupSync(t *testing.T) {
	ctx := context.Background()
	env := readEnvFile(t, filepath.Join("..", "..", ".env"))
	mysqlCfg := config.MySQL{
		Address:     []string{"127.0.0.1:3306"},
		Username:    env["MYSQL_USER"],
		Password:    env["MYSQL_PASSWORD"],
		Database:    env["MYSQL_DATABASE"],
		MaxOpenConn: 5,
		MaxIdleConn: 2,
	}
	st, err := store.Open(ctx, mysqlDSN(mysqlCfg), mysqlCfg.MaxOpenConn, mysqlCfg.MaxIdleConn)
	if err != nil {
		t.Skipf("mysql is not reachable: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	script, err := os.ReadFile(filepath.Join("..", "..", "migrations", "group.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx, string(script)); err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	a := "sa-" + suffix
	b := "sb-" + suffix
	c := "sc-" + suffix
	d := "sd-" + suffix
	t.Cleanup(func() { cleanupGroup(t, mysqlCfg, a, b, c, d) })
	srv := logic.New(st, knownUsers{a: {}, b: {}, c: {}, d: {}})

	created, err := srv.CreateGroup(ctx, &grouppb.CreateGroupReq{
		CreatorUserId: a,
		GroupName:     "room",
		GroupType:     model.GroupTypeNormal,
		MemberIds:     []string{b},
	})
	if err != nil {
		t.Fatal(err)
	}
	gid := created.GetGroupId()

	joined, err := srv.GetIncrementalJoinGroups(ctx, &grouppb.GetIncrementalJoinGroupsReq{UserId: a})
	if err != nil || !joined.GetFull() || joined.GetVersion() == 0 || joined.GetVersionId() == "" || len(joined.GetInsert()) != 0 {
		t.Fatalf("full join %+v err=%v", joined, err)
	}
	same, err := srv.GetIncrementalJoinGroups(ctx, &grouppb.GetIncrementalJoinGroupsReq{UserId: a, VersionId: joined.GetVersionId(), Version: joined.GetVersion()})
	if err != nil || same.GetFull() || len(same.GetInsert()) != 0 || len(same.GetDelete()) != 0 {
		t.Fatalf("unchanged join %+v err=%v", same, err)
	}
	reset, err := srv.GetIncrementalJoinGroups(ctx, &grouppb.GetIncrementalJoinGroupsReq{UserId: a, VersionId: "other", Version: joined.GetVersion()})
	if err != nil || !reset.GetFull() || len(reset.GetInsert()) != 0 {
		t.Fatalf("version mismatch %+v err=%v", reset, err)
	}
	page, err := srv.GetJoinedGroups(ctx, &grouppb.GetJoinedGroupsReq{UserId: a, Limit: 10})
	if err != nil || page.GetTotal() != 1 || page.GetGroups()[0].GetGroupId() != gid {
		t.Fatalf("joined page %+v err=%v", page, err)
	}
	groupIDs, err := srv.GetFullJoinGroupIDs(ctx, &grouppb.GetFullJoinGroupIDsReq{UserId: a})
	if err != nil || groupIDs.GetEqual() || len(groupIDs.GetGroupIds()) != 1 || groupIDs.GetGroupIds()[0] != gid {
		t.Fatalf("join ids %+v err=%v", groupIDs, err)
	}
	equalGroups, err := srv.GetFullJoinGroupIDs(ctx, &grouppb.GetFullJoinGroupIDsReq{UserId: a, IdHash: idhash.Hash(groupIDs.GetGroupIds())})
	if err != nil || !equalGroups.GetEqual() || len(equalGroups.GetGroupIds()) != 0 {
		t.Fatalf("equal join ids %+v err=%v", equalGroups, err)
	}

	members, err := srv.GetIncrementalGroupMembers(ctx, &grouppb.GetIncrementalGroupMembersReq{GroupId: gid, UserId: a})
	if err != nil || !members.GetFull() || len(members.GetInsert()) != 0 || members.GetGroup().GetGroupId() != gid || members.GetGroup().GetMemberCount() != 2 {
		t.Fatalf("full members %+v err=%v", members, err)
	}
	memberIDs, err := srv.GetFullGroupMemberUserIDs(ctx, &grouppb.GetFullGroupMemberUserIDsReq{GroupId: gid})
	if err != nil || memberIDs.GetEqual() || len(memberIDs.GetUserIds()) != 2 || memberIDs.GetUserIds()[0] != a {
		t.Fatalf("member ids %+v err=%v", memberIDs, err)
	}
	equalMembers, err := srv.GetFullGroupMemberUserIDs(ctx, &grouppb.GetFullGroupMemberUserIDsReq{GroupId: gid, IdHash: idhash.Hash(memberIDs.GetUserIds())})
	if err != nil || !equalMembers.GetEqual() || len(equalMembers.GetUserIds()) != 0 {
		t.Fatalf("equal member ids %+v err=%v", equalMembers, err)
	}
	if _, err := srv.InviteUserToGroup(ctx, &grouppb.InviteUserToGroupReq{GroupId: gid, OpUserId: a, UserIds: []string{c}}); err != nil {
		t.Fatal(err)
	}
	delta, err := srv.GetIncrementalGroupMembers(ctx, &grouppb.GetIncrementalGroupMembersReq{GroupId: gid, UserId: a, VersionId: members.GetVersionId(), Version: members.GetVersion()})
	if err != nil || delta.GetFull() || len(delta.GetInsert()) != 1 || delta.GetInsert()[0].GetUserId() != c || delta.GetGroup().GetMemberCount() != 3 {
		t.Fatalf("member insert %+v err=%v", delta, err)
	}
	if _, err := srv.GetIncrementalGroupMembers(ctx, &grouppb.GetIncrementalGroupMembersReq{GroupId: gid, UserId: d}); !isCode(err, codes.NotFound, errorcode.GroupNotMember) {
		t.Fatalf("outsider code=%v err=%v", status.Code(err), err)
	}

	dismissed, err := srv.CreateGroup(ctx, &grouppb.CreateGroupReq{CreatorUserId: a, GroupName: "gone", GroupType: model.GroupTypeNormal})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.DismissGroup(ctx, &grouppb.DismissGroupReq{GroupId: dismissed.GetGroupId(), OpUserId: a}); err != nil {
		t.Fatal(err)
	}
	batch, err := srv.BatchGetIncrementalGroupMembers(ctx, &grouppb.BatchGetIncrementalGroupMembersReq{
		UserId: a,
		Groups: []*grouppb.GetIncrementalGroupMembersReq{
			{GroupId: gid},
			{GroupId: dismissed.GetGroupId()},
			{GroupId: gid},
		},
	})
	if err != nil || len(batch.GetGroups()) != 1 || batch.GetGroups()[gid] == nil || !batch.GetGroups()[gid].GetFull() {
		t.Fatalf("batch %+v err=%v", batch, err)
	}
}

func oneGroup(t *testing.T, srv *logic.Server, ctx context.Context, groupID string) *grouppb.GroupInfo {
	t.Helper()
	resp, err := srv.GetGroupsInfo(ctx, &grouppb.GetGroupsInfoReq{GroupIds: []string{groupID}})
	if err != nil || len(resp.GetGroups()) != 1 {
		t.Fatalf("group %s %+v err=%v", groupID, resp, err)
	}
	return resp.GetGroups()[0]
}

func strPtr(v string) *string { return &v }

func setRole(t *testing.T, cfg config.MySQL, groupID, userID string, role int32) error {
	t.Helper()
	db, err := sql.Open("mysql", mysqlDSN(cfg))
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(context.Background(), "UPDATE group_member SET role_level = ? WHERE group_id = ? AND user_id = ?", role, groupID, userID)
	return err
}

func setMute(t *testing.T, cfg config.MySQL, groupID, userID string) error {
	t.Helper()
	db, err := sql.Open("mysql", mysqlDSN(cfg))
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(context.Background(), "UPDATE group_member SET mute_end_time = ? WHERE group_id = ? AND user_id = ?", time.Now().Add(time.Hour).UTC(), groupID, userID)
	return err
}

func memberState(t *testing.T, cfg config.MySQL, groupID, oldOwner, newOwner string) (int32, bool, error) {
	t.Helper()
	db, err := sql.Open("mysql", mysqlDSN(cfg))
	if err != nil {
		return 0, false, err
	}
	defer db.Close()
	var role int32
	if err := db.QueryRowContext(context.Background(), "SELECT role_level FROM group_member WHERE group_id = ? AND user_id = ?", groupID, oldOwner).Scan(&role); err != nil {
		return 0, false, err
	}
	var mute sql.NullTime
	if err := db.QueryRowContext(context.Background(), "SELECT mute_end_time FROM group_member WHERE group_id = ? AND user_id = ?", groupID, newOwner).Scan(&mute); err != nil {
		return 0, false, err
	}
	return role, mute.Valid, nil
}

func isCode(err error, grpc codes.Code, code errorcode.Code) bool {
	got, ok := errorcode.CodeOf(err)
	return ok && got == code && status.Code(err) == grpc
}

func cleanupGroup(t *testing.T, cfg config.MySQL, ids ...string) {
	t.Helper()
	db, err := sql.Open("mysql", mysqlDSN(cfg))
	if err != nil {
		t.Errorf("cleanup connect: %v", err)
		return
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, id := range ids {
		rows, err := db.QueryContext(ctx, "SELECT group_id FROM `group` WHERE creator_user_id = ?", id)
		if err != nil {
			t.Errorf("cleanup groups: %v", err)
			continue
		}
		var groupIDs []string
		for rows.Next() {
			var gid string
			if err := rows.Scan(&gid); err != nil {
				t.Errorf("cleanup scan: %v", err)
			}
			groupIDs = append(groupIDs, gid)
		}
		rows.Close()
		for _, gid := range groupIDs {
			for _, stmt := range []string{
				"DELETE FROM group_member WHERE group_id = ?",
				"DELETE FROM group_request WHERE group_id = ?",
				"DELETE FROM group_member_version_log WHERE d_id = ?",
				"DELETE FROM group_member_version WHERE d_id = ?",
				"DELETE FROM `group` WHERE group_id = ?",
			} {
				if _, err := db.ExecContext(ctx, stmt, gid); err != nil {
					t.Errorf("cleanup %s: %v", stmt, err)
				}
			}
		}
		for _, stmt := range []string{
			"DELETE FROM group_member WHERE user_id = ?",
			"DELETE FROM group_request WHERE user_id = ?",
			"DELETE FROM group_join_version_log WHERE d_id = ?",
			"DELETE FROM group_join_version WHERE d_id = ?",
		} {
			if _, err := db.ExecContext(ctx, stmt, id); err != nil {
				t.Errorf("cleanup %s: %v", stmt, err)
			}
		}
	}
}

func readEnvFile(t *testing.T, path string) map[string]string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		out[k] = v
	}
	return out
}
