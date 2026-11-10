package call

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

func TestGroupInviteAcceptLeave(t *testing.T) {
	board := openBoard(t)
	ctx := context.Background()
	host, a := testUsers()
	b := a + "b"
	c := b + "c"
	groupID := "g" + host
	step, err := board.InviteGroup(ctx, host, groupID, MediaAudio, []string{a, b})
	if err != nil || step.Ended || !validCallID(step.CallID) || !sameSet(step.Ringing, []string{a, b}) || !sameSet(step.Joined, []string{host}) {
		t.Fatalf("invite %+v err=%v", step, err)
	}
	if kind, err := board.Kind(ctx, step.CallID); err != nil || kind != KindGroup {
		t.Fatalf("kind %s err=%v", kind, err)
	}
	if again, err := board.InviteGroup(ctx, c, groupID, MediaAudio, []string{a}); !errorIs(err, errorcode.ErrCallGroupBusy) {
		t.Fatalf("second room %+v err=%v", again, err)
	}
	joined, err := board.AcceptGroup(ctx, a, step.CallID)
	if err != nil || joined.Ended || !sameSet(joined.Joined, []string{host, a}) || !sameSet(joined.Ringing, []string{b}) {
		t.Fatalf("accept %+v err=%v", joined, err)
	}
	if _, err := board.Accepted(ctx, a, step.CallID); err != nil {
		t.Fatal(err)
	}
	if _, err := board.Accepted(ctx, b, step.CallID); !errorIs(err, errorcode.ErrCallNotMember) {
		t.Fatalf("ringing token %v", err)
	}
	left, err := board.LeaveGroup(ctx, a, step.CallID)
	if err != nil || left.Ended || !sameSet(left.Joined, []string{host}) || !sameSet(left.Ringing, []string{b}) {
		t.Fatalf("leave %+v err=%v", left, err)
	}
	ended, err := board.LeaveGroup(ctx, host, step.CallID)
	if err != nil || !ended.Ended || !sameSet(ended.Ringing, []string{b}) {
		t.Fatalf("host leave %+v err=%v", ended, err)
	}
	if n, err := board.rdb.Exists(ctx, board.busyKey(host), board.busyKey(a), board.busyKey(b), board.groupKey(groupID)).Result(); err != nil || n != 0 {
		t.Fatalf("locks left %d err=%v", n, err)
	}
}

func TestGroupRejectEndsWhenOnlyHostRemains(t *testing.T) {
	board := openBoard(t)
	ctx := context.Background()
	host, a := testUsers()
	b := a + "b"
	groupID := "g" + a
	step, err := board.InviteGroup(ctx, host, groupID, MediaAudio, []string{a, b})
	if err != nil {
		t.Fatal(err)
	}
	kept, err := board.RejectGroup(ctx, a, step.CallID)
	if err != nil || kept.Ended || !sameSet(kept.Ringing, []string{b}) {
		t.Fatalf("reject one %+v err=%v", kept, err)
	}
	ended, err := board.RejectGroup(ctx, b, step.CallID)
	if err != nil || !ended.Ended || !sameSet(ended.Joined, []string{host}) {
		t.Fatalf("reject last %+v err=%v", ended, err)
	}
	if _, err := board.Group(ctx, groupID); err != nil {
		t.Fatal(err)
	}
	view, err := board.Group(ctx, groupID)
	if err != nil || view.CallID != "" {
		t.Fatalf("room left %+v err=%v", view, err)
	}
}

func TestGroupVideoInviteBlocksAudioInSameGroup(t *testing.T) {
	board := openBoard(t)
	ctx := context.Background()
	host, a := testUsers()
	groupID := "gv" + host
	step, err := board.InviteGroup(ctx, host, groupID, MediaVideo, []string{a})
	if err != nil || step.Media != MediaVideo || !sameSet(step.Joined, []string{host}) || !sameSet(step.Ringing, []string{a}) {
		t.Fatalf("video invite %+v err=%v", step, err)
	}
	if _, err := board.InviteGroup(ctx, a+"x", groupID, MediaAudio, []string{a}); !errorIs(err, errorcode.ErrCallGroupBusy) {
		t.Fatalf("audio while video open: %v", err)
	}
	if _, err := board.InviteGroup(ctx, host, groupID+"2", "film", []string{a}); !errorIs(err, errorcode.ErrCallInvalid) {
		t.Fatalf("bad media: %v", err)
	}
}

func TestGroupBusyInviteeAndLateJoin(t *testing.T) {
	board := openBoard(t)
	ctx := context.Background()
	host, peer := testUsers()
	busyUser := peer + "z"
	other := busyUser + "q"
	if _, err := board.Invite(ctx, busyUser, other, MediaAudio); err != nil {
		t.Fatal(err)
	}
	groupID := "g" + peer
	late := other + "j"
	step, err := board.InviteGroup(ctx, host, groupID, MediaAudio, []string{peer, busyUser})
	if err != nil || !sameSet(step.Ringing, []string{peer}) || !sameSet(step.Busy, []string{busyUser}) {
		t.Fatalf("partial %+v err=%v", step, err)
	}
	entered, err := board.EnterGroup(ctx, late, step.CallID)
	if err != nil || entered.Ended || !sameSet(entered.Joined, []string{host, late}) {
		t.Fatalf("join %+v err=%v", entered, err)
	}
	if _, err := board.EnterGroup(ctx, late, step.CallID); !errorIs(err, errorcode.ErrCallState) {
		t.Fatalf("second join %v", err)
	}
}

func TestGroupTimeoutOneInvitee(t *testing.T) {
	board := openBoard(t)
	ctx := context.Background()
	host, a := testUsers()
	b := a + "b"
	groupID := "g" + b
	step, err := board.InviteGroup(ctx, host, groupID, MediaAudio, []string{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if err := board.rdb.ZAdd(ctx, board.groupRingKey(), redis.Z{Score: 1, Member: step.CallID + ":" + a}).Err(); err != nil {
		t.Fatal(err)
	}
	ended, err := board.ExpireGroup(ctx, time.Unix(2, 0))
	if err != nil || len(ended) != 1 || ended[0].Ended || ended[0].Actor != a || !sameSet(ended[0].Ringing, []string{b}) {
		t.Fatalf("expire %+v err=%v", ended, err)
	}
}

func sameSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]int, len(got))
	for _, id := range got {
		seen[id]++
	}
	for _, id := range want {
		if seen[id] == 0 {
			return false
		}
		seen[id]--
	}
	return true
}
