//go:build perf

package main

import (
	"testing"
	"time"

	grouppb "github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/logic"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
	"github.com/lisuhang11/SuIM/services/group/internal/store"
)

func TestPerfGroup(t *testing.T) {
	ctx, db, st, rdb := openGroupBench(t)
	started := time.Now()
	seedGroupBench(t, ctx, db)
	t.Logf("RESULT name=seed ms=%.3f", float64(time.Since(started).Microseconds())/1000)

	big := groupBenchGroup(1)
	owner := groupBenchUser(1)
	memberSelect := `SELECT group_id, user_id, nickname, face_url, role_level, join_time, join_source, inviter_user_id, operator_user_id, mute_end_time, ex FROM group_member`
	t.Logf("EXPLAIN group %s", explainGroup(t, ctx, db, "SELECT group_id FROM `group` WHERE group_id = ?", big))
	t.Logf("EXPLAIN member_count %s", explainGroup(t, ctx, db, "SELECT COUNT(*) FROM group_member WHERE group_id = ?", big))
	t.Logf("EXPLAIN member_page %s", explainGroup(t, ctx, db, memberSelect+" WHERE group_id = ? ORDER BY role_level DESC, join_time ASC, user_id ASC LIMIT ? OFFSET ?", big, 20, 0))
	t.Logf("EXPLAIN member_deep %s", explainGroup(t, ctx, db, memberSelect+" WHERE group_id = ? ORDER BY role_level DESC, join_time ASC, user_id ASC LIMIT ? OFFSET ?", big, 20, groupBenchMembers-20))
	t.Logf("EXPLAIN member_ids %s", explainGroup(t, ctx, db, "SELECT user_id FROM group_member WHERE group_id = ? ORDER BY role_level DESC, join_time ASC, user_id ASC", big))
	t.Logf("EXPLAIN member_one %s", explainGroup(t, ctx, db, memberSelect+" WHERE group_id = ? AND user_id = ?", big, owner))
	t.Logf("EXPLAIN joined_page %s", explainGroup(t, ctx, db, "SELECT group_id FROM group_member WHERE user_id = ? ORDER BY join_time DESC, group_id ASC LIMIT ? OFFSET ?", owner, 20, 0))
	t.Logf("EXPLAIN joined_count %s", explainGroup(t, ctx, db, "SELECT COUNT(*) FROM group_member WHERE user_id = ?", owner))
	t.Logf("EXPLAIN requests %s", explainGroup(t, ctx, db, "SELECT group_id, user_id FROM group_request WHERE group_id = ? AND handle_result IN (?) ORDER BY req_time DESC, group_id ASC, user_id ASC LIMIT ? OFFSET ?", big, 0, 20, 0))
	t.Logf("EXPLAIN member_log %s", explainGroup(t, ctx, db, "SELECT e_id, state, version FROM group_member_version_log WHERE d_id = ? AND version > ? ORDER BY version LIMIT ?", big, groupBenchMembers-50, 200))
	t.Logf("EXPLAIN managed %s", explainGroup(t, ctx, db, "SELECT group_id FROM group_member WHERE user_id = ? AND role_level >= ? ORDER BY group_id", owner, model.RoleAdmin))

	srv := logic.New(st, benchUsers{}, nil)
	mustGroups(t, srv, ctx, big, groupBenchMembers)
	timeCalls(t, "group_info", groupBenchRepeat, func() {
		mustGroups(t, srv, ctx, big, groupBenchMembers)
	})
	timeCalls(t, "member_one", groupBenchRepeat, func() {
		resp, err := srv.GetGroupMember(ctx, &grouppb.GetGroupMemberReq{GroupId: big, UserId: owner})
		if err != nil || resp.GetMember().GetUserId() != owner {
			t.Fatalf("member %+v err=%v", resp, err)
		}
	})
	timeCalls(t, "member_page", groupBenchRepeat, func() {
		resp, err := srv.GetGroupMembers(ctx, &grouppb.GetGroupMembersReq{GroupId: big, UserId: owner, Limit: 20})
		if err != nil || resp.GetTotal() != groupBenchMembers || len(resp.GetMembers()) != 20 {
			t.Fatalf("page total=%d len=%d err=%v", resp.GetTotal(), len(resp.GetMembers()), err)
		}
	})
	timeCalls(t, "member_deep", groupBenchRepeat, func() {
		resp, err := srv.GetGroupMembers(ctx, &grouppb.GetGroupMembersReq{GroupId: big, UserId: owner, Offset: groupBenchMembers - 20, Limit: 20})
		if err != nil || len(resp.GetMembers()) != 20 {
			t.Fatalf("deep len=%d err=%v", len(resp.GetMembers()), err)
		}
	})
	timeCalls(t, "member_ids", groupBenchRepeat, func() {
		resp, err := srv.GetFullGroupMemberUserIDs(ctx, &grouppb.GetFullGroupMemberUserIDsReq{GroupId: big, UserId: owner})
		if err != nil || len(resp.GetUserIds()) != groupBenchMembers {
			t.Fatalf("ids %d err=%v", len(resp.GetUserIds()), err)
		}
	})
	timeCalls(t, "joined_page", groupBenchRepeat, func() {
		resp, err := srv.GetJoinedGroups(ctx, &grouppb.GetJoinedGroupsReq{UserId: owner, Limit: 20})
		if err != nil || resp.GetTotal() != groupBenchGroups+1 || len(resp.GetGroups()) != 20 {
			t.Fatalf("joined total=%d len=%d err=%v", resp.GetTotal(), len(resp.GetGroups()), err)
		}
	})
	timeCalls(t, "incremental_zero", groupBenchRepeat, func() {
		resp, err := srv.GetIncrementalGroupMembers(ctx, &grouppb.GetIncrementalGroupMembersReq{GroupId: big, UserId: owner})
		if err != nil || !resp.GetFull() {
			t.Fatalf("zero %+v err=%v", resp, err)
		}
	})
	head, err := st.MemberVersion(ctx, big)
	if err != nil {
		t.Fatal(err)
	}
	timeCalls(t, "incremental_near", groupBenchRepeat, func() {
		resp, err := srv.GetIncrementalGroupMembers(ctx, &grouppb.GetIncrementalGroupMembersReq{GroupId: big, UserId: owner, VersionId: head.ID, Version: head.Version - 50})
		if err != nil || resp.GetFull() || len(resp.GetInsert()) == 0 {
			t.Fatalf("near full=%v insert=%d err=%v", resp.GetFull(), len(resp.GetInsert()), err)
		}
	})
	timeCalls(t, "incremental_far", groupBenchRepeat, func() {
		resp, err := srv.GetIncrementalGroupMembers(ctx, &grouppb.GetIncrementalGroupMembersReq{GroupId: big, UserId: owner, VersionId: head.ID, Version: head.Version - 500})
		if err != nil || !resp.GetFull() {
			t.Fatalf("far %+v err=%v", resp, err)
		}
	})
	timeCalls(t, "requests_page", groupBenchRepeat, func() {
		resp, err := srv.GetGroupApplicationList(ctx, &grouppb.GetGroupApplicationListReq{FromUserId: owner, GroupIds: []string{big}, HandleResults: []int32{0}, Limit: 20})
		if err != nil || resp.GetTotal() != groupBenchRequests || len(resp.GetRequests()) != 20 {
			t.Fatalf("requests total=%d len=%d err=%v", resp.GetTotal(), len(resp.GetRequests()), err)
		}
	})

	cache, stop := store.NewReadCache(ctx, rdb, store.CacheConfig{
		Topic: "suim:group:bench:delete", SlotNum: 100, SlotSize: 2000, SuccessExpire: 300 * time.Second, FailedExpire: 5 * time.Second,
	})
	defer stop()
	st.UseCache(cache)
	cached := logic.New(st, benchUsers{}, nil)
	mustGroups(t, cached, ctx, big, groupBenchMembers)
	_, _ = cached.GetGroupMember(ctx, &grouppb.GetGroupMemberReq{GroupId: big, UserId: owner})
	_, _ = cached.GetFullGroupMemberUserIDs(ctx, &grouppb.GetFullGroupMemberUserIDsReq{GroupId: big, UserId: owner})
	timeCalls(t, "group_info_local", groupBenchRepeat, func() {
		mustGroups(t, cached, ctx, big, groupBenchMembers)
	})
	timeCalls(t, "member_one_local", groupBenchRepeat, func() {
		resp, err := cached.GetGroupMember(ctx, &grouppb.GetGroupMemberReq{GroupId: big, UserId: owner})
		if err != nil || resp.GetMember().GetUserId() != owner {
			t.Fatalf("cached member %+v err=%v", resp, err)
		}
	})
	timeCalls(t, "member_ids_local", groupBenchRepeat, func() {
		resp, err := cached.GetFullGroupMemberUserIDs(ctx, &grouppb.GetFullGroupMemberUserIDsReq{GroupId: big, UserId: owner})
		if err != nil || len(resp.GetUserIds()) != groupBenchMembers {
			t.Fatalf("cached ids %d err=%v", len(resp.GetUserIds()), err)
		}
	})
}
