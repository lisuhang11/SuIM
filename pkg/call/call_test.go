package call

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

func TestInviteAcceptHangup(t *testing.T) {
	board := openBoard(t)
	ctx := context.Background()
	caller, callee := testUsers()
	sig, err := board.Invite(ctx, caller, callee, MediaAudio)
	if err != nil || sig.Op != OpInvite || !validCallID(sig.CallID) {
		t.Fatalf("invite %+v err=%v", sig, err)
	}
	accepted, err := board.Accept(ctx, callee, sig.CallID)
	if err != nil || accepted.CallerID != caller || accepted.CalleeID != callee || accepted.Media != MediaAudio {
		t.Fatalf("accept %+v err=%v", accepted, err)
	}
	if got, err := board.Accepted(ctx, caller, sig.CallID); err != nil || got.CalleeID != callee || got.Media != MediaAudio {
		t.Fatalf("accepted read %+v err=%v", got, err)
	}
	if _, err := board.Accepted(ctx, "outsider", sig.CallID); !errorIs(err, errorcode.ErrCallNotMember) {
		t.Fatalf("outsider %v", err)
	}
	if _, err := board.Hangup(ctx, caller, sig.CallID); err != nil {
		t.Fatal(err)
	}
	if _, err := board.Hangup(ctx, caller, sig.CallID); !errorIs(err, errorcode.ErrCallNotFound) {
		t.Fatalf("second hangup %v", err)
	}
	if n, err := board.rdb.Exists(ctx, board.busyKey(caller), board.busyKey(callee)).Result(); err != nil || n != 0 {
		t.Fatalf("busy left %d err=%v", n, err)
	}
}

func TestInviteBusy(t *testing.T) {
	board := openBoard(t)
	ctx := context.Background()
	caller, callee := testUsers()
	other := caller + "x"
	first, err := board.Invite(ctx, caller, callee, MediaVideo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := board.Invite(ctx, caller, other, MediaAudio); !errorIs(err, errorcode.ErrCallBusy) {
		t.Fatalf("self busy %v", err)
	}
	if _, err := board.Invite(ctx, other, callee, MediaAudio); !errorIs(err, errorcode.ErrCallPeerBusy) {
		t.Fatalf("peer busy %v", err)
	}
	if _, err := board.Cancel(ctx, callee, first.CallID); !errorIs(err, errorcode.ErrCallNotMember) {
		t.Fatalf("callee cancel %v", err)
	}
	if _, err := board.Cancel(ctx, caller, first.CallID); err != nil {
		t.Fatal(err)
	}
}

func TestRejectAndTimeout(t *testing.T) {
	board := openBoard(t)
	ctx := context.Background()
	caller, callee := testUsers()
	sig, err := board.Invite(ctx, caller, callee, MediaAudio)
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := board.Reject(ctx, callee, sig.CallID)
	if err != nil || rejected.Reason != "reject" || rejected.CallerID != caller {
		t.Fatalf("reject %+v err=%v", rejected, err)
	}

	board.ring = -time.Second
	sig, err = board.Invite(ctx, caller, callee, MediaAudio)
	if err != nil {
		t.Fatal(err)
	}
	ended, err := board.Expire(ctx, time.Now())
	if err != nil || len(ended) != 1 || ended[0].CallID != sig.CallID || ended[0].Reason != "timeout" {
		t.Fatalf("expire %+v err=%v", ended, err)
	}
	if _, err := board.Accept(ctx, callee, sig.CallID); !errorIs(err, errorcode.ErrCallNotFound) {
		t.Fatalf("accept after timeout %v", err)
	}
}

func TestCallPublish(t *testing.T) {
	board := openBoard(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	before, err := board.rdb.PubSubNumSub(ctx, board.channel()).Result()
	if err != nil {
		t.Fatal(err)
	}
	seen := make(chan Event, 1)
	go board.Subscribe(ctx, func(ev Event) {
		select {
		case seen <- ev:
		default:
		}
	})
	waitSubscribers(t, ctx, board.rdb, board.channel(), before[board.channel()])
	ev := Event{Users: []string{"bob"}, Op: OpRing, CallID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Caller: "ann", Callee: "bob", Media: MediaAudio}
	if err := board.Publish(ctx, ev); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-seen:
		if len(got.Users) != 1 || got.Users[0] != "bob" || got.Op != OpRing || got.CallID != ev.CallID {
			t.Fatalf("event %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("event was not delivered")
	}
}

func openBoard(t *testing.T) *Board {
	t.Helper()
	env := readCallEnv(t, "../../.env")
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", Password: env["REDIS_PASSWORD"]})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		t.Skipf("redis is not reachable: %v", err)
	}
	board := newBoard(rdb, Prefix+"t"+strconv.FormatInt(time.Now().UnixNano(), 10)+":")
	t.Cleanup(func() {
		var cursor uint64
		for {
			keys, next, err := rdb.Scan(context.Background(), cursor, board.prefix+"*", 100).Result()
			if err != nil {
				break
			}
			if len(keys) > 0 {
				_ = rdb.Del(context.Background(), keys...).Err()
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
		_ = rdb.Close()
	})
	return board
}

func testUsers() (string, string) {
	id := strconv.FormatInt(time.Now().UnixNano(), 36)
	return "ca" + id, "cb" + id
}

func errorIs(err, target error) bool {
	return errors.Is(err, target)
}

func waitSubscribers(t *testing.T, ctx context.Context, rdb redis.UniversalClient, channel string, before int64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		n, err := rdb.PubSubNumSub(ctx, channel).Result()
		if err == nil && n[channel] > before {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("subscribers %v err=%v", n, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func readCallEnv(t *testing.T, path string) map[string]string {
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
