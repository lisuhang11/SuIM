//go:build load

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"math/rand/v2"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
	"github.com/lisuhang11/SuIM/services/relation/internal/logic"
	"github.com/lisuhang11/SuIM/services/relation/internal/model"
	"github.com/lisuhang11/SuIM/services/relation/internal/store"
)

const (
	loadHotN          = 100
	loadMidN          = 400
	loadNormalN       = 9500
	loadHotFriends    = 3500
	loadMidFriends    = 250
	loadNormalFriends = 5
	loadHotInbox      = 2000
	loadMidInbox      = 10
	loadNormalInbox   = 2
	loadPins          = 20
	loadReaders       = 16
	loadWriters       = 8
	loadSyncWorkers   = 4
	loadWarmup        = 2 * time.Second
	loadOnline        = 20 * time.Second
	loadSync          = 10 * time.Second
	loadOwnerBase     = 3_000_001
	loadFriendBase    = 4_000_001
	loadApplicantBase = 5_000_001
	loadWriteBase     = 7_000_001
)

func TestLoadRelation(t *testing.T) {
	ctx := context.Background()
	env := readEnvFile(t, filepath.Join("..", "..", ".env"))
	createBenchDB(t, env)
	cfg := config.MySQL{
		Address:     []string{"127.0.0.1:3306"},
		Username:    env["MYSQL_USER"],
		Password:    env["MYSQL_PASSWORD"],
		Database:    benchDB,
		MaxOpenConn: 40,
		MaxIdleConn: 20,
	}
	st, err := store.Open(ctx, mysqlDSN(cfg), cfg.MaxOpenConn, cfg.MaxIdleConn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	script, err := os.ReadFile(filepath.Join("..", "..", "migrations", "relation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx, string(script)); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("mysql", mysqlDSN(cfg))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = db.Close() })
	truncateBench(t, db)
	t.Cleanup(func() { truncateBench(t, db) })

	started := time.Now()
	pop := seedLoad(t, ctx, db)
	t.Logf("RESULT name=seed ms=%.3f users=%d friend_rows=%d request_rows=%d version_rows=%d",
		float64(time.Since(started).Microseconds())/1000, pop.users(), pop.friendRows, pop.requestRows, pop.versionRows)

	srv := logic.New(st, benchUsers{}, logic.Hooks{})
	assertLoadShape(t, ctx, srv, db, pop)
	t.Logf("EXPLAIN friend_page %s", explainQuery(t, ctx, db, `SELECT friend_user_id FROM friend WHERE owner_user_id = ? ORDER BY is_pinned DESC, create_time ASC, friend_user_id ASC LIMIT ? OFFSET ?`, pop.hot[0].id, 20, 0))
	t.Logf("EXPLAIN recv_page %s", explainQuery(t, ctx, db, `SELECT from_user_id FROM friend_request WHERE to_user_id = ? AND handle_result IN (?) ORDER BY create_time DESC, from_user_id ASC, to_user_id ASC LIMIT ? OFFSET ?`, pop.hot[0].id, 0, 20, 0))

	probeRequestInsert(t, ctx, st, db)
	locksBefore := readLockWaits(ctx, db)
	agreed := runOnline(t, ctx, srv, pop)
	locksAfter := readLockWaits(ctx, db)
	if locksBefore.err != "" || locksAfter.err != "" {
		t.Logf("RESULT name=lock_waits unavailable before=%s after=%s", locksBefore.err, locksAfter.err)
	} else {
		t.Logf("RESULT name=lock_waits before=%d after=%d delta=%d time_ms_before=%d time_ms_after=%d",
			locksBefore.waits, locksAfter.waits, locksAfter.waits-locksBefore.waits, locksBefore.timeMs, locksAfter.timeMs)
	}

	checkAgreed(t, ctx, db, agreed)
	checkSamePair(t, ctx, srv, db)

	runSync(t, ctx, srv, st, pop.hot)
}

type loadUser struct {
	id      string
	friends int
	inbox   int
}

type loadPop struct {
	hot, mid, normal []loadUser
	friendRows       int
	requestRows      int
	versionRows      int
}

func (p loadPop) users() int { return len(p.hot) + len(p.mid) + len(p.normal) }

func (p loadPop) pick(r *rand.Rand) loadUser {
	n := r.IntN(100)
	switch {
	case n < 95:
		return p.normal[r.IntN(len(p.normal))]
	case n < 99:
		return p.mid[r.IntN(len(p.mid))]
	default:
		return p.hot[r.IntN(len(p.hot))]
	}
}

type benchUsers struct{}

func (benchUsers) CheckUsers(context.Context, []string) error { return nil }

func (benchUsers) GetUsers(_ context.Context, ids []string) (map[string]model.UserProfile, error) {
	out := make(map[string]model.UserProfile, len(ids))
	for _, id := range ids {
		out[id] = model.UserProfile{UserID: id, Nickname: id}
	}
	return out, nil
}

type batchInsert struct {
	db     *sql.DB
	prefix string
	tuple  string
	width  int
	n      int
	args   []any
}

func (b *batchInsert) add(ctx context.Context, args ...any) error {
	if len(args) != b.width {
		return fmt.Errorf("batch width %d got %d", b.width, len(args))
	}
	b.args = append(b.args, args...)
	b.n++
	if b.n >= 400 {
		return b.flush(ctx)
	}
	return nil
}

func (b *batchInsert) flush(ctx context.Context) error {
	if b.n == 0 {
		return nil
	}
	query := b.prefix + strings.TrimRight(strings.Repeat(b.tuple+",", b.n), ",")
	if _, err := b.db.ExecContext(ctx, query, b.args...); err != nil {
		return err
	}
	b.n = 0
	b.args = b.args[:0]
	return nil
}

func seedLoad(t *testing.T, ctx context.Context, db *sql.DB) loadPop {
	t.Helper()
	pop := loadPop{
		hot:    make([]loadUser, loadHotN),
		mid:    make([]loadUser, loadMidN),
		normal: make([]loadUser, loadNormalN),
	}
	for i := range pop.hot {
		pop.hot[i] = loadUser{id: benchID(loadOwnerBase + i), friends: loadHotFriends, inbox: loadHotInbox}
	}
	for i := range pop.mid {
		pop.mid[i] = loadUser{id: benchID(loadOwnerBase + loadHotN + i), friends: loadMidFriends, inbox: loadMidInbox}
	}
	for i := range pop.normal {
		pop.normal[i] = loadUser{id: benchID(loadOwnerBase + loadHotN + loadMidN + i), friends: loadNormalFriends, inbox: loadNormalInbox}
	}

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	friends := &batchInsert{
		db: db, width: 5,
		prefix: `INSERT INTO friend (owner_user_id, friend_user_id, remark, create_time, add_source, operator_user_id, is_pinned) VALUES `,
		tuple:  `(?,?, '', ?, 2, ?, ?)`,
	}
	logs := &batchInsert{
		db: db, width: 5,
		prefix: `INSERT INTO friend_version_log (owner_user_id, elem_id, state, version, last_update) VALUES `,
		tuple:  `(?,?,?,?,?)`,
	}
	var friendSeq int
	writeOwner := func(user loadUser, withVersion bool) {
		t.Helper()
		for i := 0; i < user.friends; i++ {
			friendSeq++
			peer := benchID(loadFriendBase + friendSeq)
			at := base.Add(time.Duration(i) * time.Second)
			pinned := 0
			if withVersion && i < loadPins {
				pinned = 1
			}
			if err := friends.add(ctx, user.id, peer, at, user.id, pinned); err != nil {
				t.Fatal(err)
			}
			if err := friends.add(ctx, peer, user.id, at, user.id, 0); err != nil {
				t.Fatal(err)
			}
			if withVersion {
				if err := logs.add(ctx, user.id, peer, model.VersionInsert, uint64(i+1), at); err != nil {
					t.Fatal(err)
				}
			}
		}
		if withVersion {
			at := base.Add(time.Duration(user.friends) * time.Second)
			if err := logs.add(ctx, user.id, model.SortElemID, model.VersionUpdate, uint64(user.friends+1), at); err != nil {
				t.Fatal(err)
			}
			vid := fmt.Sprintf("%024x", friendSeq)
			if _, err := db.ExecContext(ctx, `INSERT INTO friend_version (id, owner_user_id, version, deleted, last_update) VALUES (?, ?, ?, 0, ?)`, vid, user.id, user.friends+1, at); err != nil {
				t.Fatal(err)
			}
			pop.versionRows += user.friends + 1
		}
		pop.friendRows += user.friends * 2
	}
	for _, user := range pop.hot {
		writeOwner(user, true)
	}
	for _, user := range pop.mid {
		writeOwner(user, false)
	}
	for _, user := range pop.normal {
		writeOwner(user, false)
	}
	if err := friends.flush(ctx); err != nil {
		t.Fatal(err)
	}
	if err := logs.flush(ctx); err != nil {
		t.Fatal(err)
	}

	requests := &batchInsert{
		db: db, width: 3,
		prefix: `INSERT INTO friend_request (from_user_id, to_user_id, handle_result, req_msg, create_time, handler_user_id, handle_msg, handle_time) VALUES `,
		tuple:  `(?,?,0,'hi',?,'','',NULL)`,
	}
	var applicant int
	writeInbox := func(user loadUser) {
		t.Helper()
		for i := 0; i < user.inbox; i++ {
			applicant++
			at := base.Add(time.Duration(i) * time.Second)
			if err := requests.add(ctx, benchID(loadApplicantBase+applicant), user.id, at); err != nil {
				t.Fatal(err)
			}
		}
		pop.requestRows += user.inbox
	}
	for _, user := range pop.hot {
		writeInbox(user)
	}
	for _, user := range pop.mid {
		writeInbox(user)
	}
	for _, user := range pop.normal {
		writeInbox(user)
	}
	if err := requests.flush(ctx); err != nil {
		t.Fatal(err)
	}

	var gotFriends, gotRequests int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM friend`).Scan(&gotFriends); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM friend_request`).Scan(&gotRequests); err != nil {
		t.Fatal(err)
	}
	if gotFriends != pop.friendRows || gotRequests != pop.requestRows {
		t.Fatalf("seed friends %d/%d requests %d/%d", gotFriends, pop.friendRows, gotRequests, pop.requestRows)
	}
	return pop
}

func assertLoadShape(t *testing.T, ctx context.Context, srv *logic.Server, db *sql.DB, pop loadPop) {
	t.Helper()
	checkList := func(user loadUser) {
		t.Helper()
		page, err := srv.GetFriendList(ctx, &relationpb.GetFriendListReq{UserId: user.id, Limit: 20})
		if err != nil || int(page.GetTotal()) != user.friends || len(page.GetFriends()) != min(20, user.friends) {
			t.Fatalf("list %s total=%d len=%d want=%d err=%v", user.id, page.GetTotal(), len(page.GetFriends()), user.friends, err)
		}
		recv, err := srv.GetRecvFriendRequests(ctx, &relationpb.GetRecvFriendRequestsReq{UserId: user.id, Limit: 20, HandleResults: []int32{0}})
		if err != nil || int(recv.GetTotal()) != user.inbox || len(recv.GetRequests()) != min(20, user.inbox) {
			t.Fatalf("inbox %s total=%d len=%d want=%d err=%v", user.id, recv.GetTotal(), len(recv.GetRequests()), user.inbox, err)
		}
	}
	checkList(pop.hot[0])
	checkList(pop.mid[0])
	checkList(pop.normal[0])
	if !pop.hot[0].friendsPagePinned(t, ctx, srv) {
		t.Fatal("hot first friend is not pinned")
	}
	head := friendHead(t, ctx, db, pop.hot[0].id)
	near, err := srv.GetIncrementalFriends(ctx, &relationpb.GetIncrementalFriendsReq{UserId: pop.hot[0].id, VersionId: head.id, Version: head.version - 50})
	if err != nil || near.GetFull() || near.GetSortVersion() == 0 {
		t.Fatalf("near %+v err=%v", near, err)
	}
	far, err := srv.GetIncrementalFriends(ctx, &relationpb.GetIncrementalFriendsReq{UserId: pop.hot[0].id, VersionId: head.id, Version: head.version - 500})
	if err != nil || !far.GetFull() {
		t.Fatalf("far %+v err=%v", far, err)
	}
}

func (u loadUser) friendsPagePinned(t *testing.T, ctx context.Context, srv *logic.Server) bool {
	t.Helper()
	page, err := srv.GetFriendList(ctx, &relationpb.GetFriendListReq{UserId: u.id, Limit: 20})
	if err != nil || len(page.GetFriends()) == 0 {
		t.Fatal(err)
	}
	return page.GetFriends()[0].GetIsPinned()
}

type versionHead struct {
	id      string
	version uint64
}

func friendHead(t *testing.T, ctx context.Context, db *sql.DB, owner string) versionHead {
	t.Helper()
	var head versionHead
	err := db.QueryRowContext(ctx, `SELECT id, version FROM friend_version WHERE owner_user_id = ?`, owner).Scan(&head.id, &head.version)
	if err != nil {
		t.Fatal(err)
	}
	return head
}

var errContention = errors.New("contention")

type latency struct {
	mu         sync.Mutex
	samples    []time.Duration
	errs       int
	contention int
	first      string
}

func (l *latency) add(d time.Duration, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if errors.Is(err, errContention) {
		l.contention++
		return
	}
	if err != nil {
		l.errs++
		if l.first == "" {
			l.first = err.Error()
		}
		return
	}
	l.samples = append(l.samples, d)
}

func writeErr(err error) error {
	if err == nil {
		return nil
	}
	if code, ok := errorcode.CodeOf(err); ok && code == errorcode.Internal {
		return errContention
	}
	return err
}

func (l *latency) report(t *testing.T, name string) {
	t.Helper()
	l.mu.Lock()
	samples := append([]time.Duration(nil), l.samples...)
	errs, contention, first := l.errs, l.contention, l.first
	l.mu.Unlock()
	if errs > 0 {
		t.Fatalf("%s errors=%d first=%s", name, errs, first)
	}
	if contention > 0 {
		t.Logf("RESULT name=%s_contention n=%d", name, contention)
	}
	if len(samples) == 0 {
		t.Fatalf("%s recorded no samples", name)
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	ms := func(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
	t.Logf("RESULT name=%s n=%d p50_ms=%.3f p95_ms=%.3f p99_ms=%.3f max_ms=%.3f",
		name, len(samples), ms(percentile(samples, 0.50)), ms(percentile(samples, 0.95)), ms(percentile(samples, 0.99)), ms(samples[len(samples)-1]))
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

type agreedPair struct {
	from, to string
}

func runOnline(t *testing.T, ctx context.Context, srv *logic.Server, pop loadPop) []agreedPair {
	t.Helper()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var record atomic.Bool
	var lists, inbox, apply, agree latency
	var writes atomic.Uint64
	var mu sync.Mutex
	var pairs []agreedPair

	var wg sync.WaitGroup
	for i := 0; i < loadReaders; i++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			r := rand.New(rand.NewPCG(uint64(seed+1), 11))
			for runCtx.Err() == nil {
				user := pop.pick(r)
				if r.IntN(2) == 0 {
					start := time.Now()
					resp, err := srv.GetFriendList(runCtx, &relationpb.GetFriendListReq{UserId: user.id, Limit: 20})
					if runCtx.Err() != nil {
						return
					}
					if err == nil && (int(resp.GetTotal()) != user.friends || len(resp.GetFriends()) != min(20, user.friends)) {
						err = fmt.Errorf("list %s total=%d len=%d", user.id, resp.GetTotal(), len(resp.GetFriends()))
					}
					if record.Load() {
						lists.add(time.Since(start), err)
					}
					continue
				}
				start := time.Now()
				resp, err := srv.GetRecvFriendRequests(runCtx, &relationpb.GetRecvFriendRequestsReq{UserId: user.id, Limit: 20, HandleResults: []int32{model.HandlePending}})
				if runCtx.Err() != nil {
					return
				}
				if err == nil && (int(resp.GetTotal()) != user.inbox || len(resp.GetRequests()) != min(20, user.inbox)) {
					err = fmt.Errorf("inbox %s total=%d len=%d", user.id, resp.GetTotal(), len(resp.GetRequests()))
				}
				if record.Load() {
					inbox.add(time.Since(start), err)
				}
			}
		}(i)
	}
	for i := 0; i < loadWriters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for runCtx.Err() == nil {
				n := writes.Add(1)
				from := benchID(loadWriteBase + int(n)*2)
				to := benchID(loadWriteBase + int(n)*2 + 1)
				start := time.Now()
				_, err := srv.SendFriendRequest(runCtx, &relationpb.SendFriendRequestReq{FromUserId: from, ToUserId: to, Message: "load"})
				if runCtx.Err() != nil {
					return
				}
				if record.Load() {
					apply.add(time.Since(start), writeErr(err))
				}
				if err != nil {
					continue
				}
				start = time.Now()
				_, err = srv.RespondFriendApply(runCtx, &relationpb.RespondFriendApplyReq{FromUserId: from, ToUserId: to, HandleResult: model.HandleAgree, HandleMsg: "ok"})
				if runCtx.Err() != nil {
					return
				}
				if record.Load() {
					agree.add(time.Since(start), writeErr(err))
				}
				if err == nil && record.Load() {
					mu.Lock()
					if len(pairs) < 40 {
						pairs = append(pairs, agreedPair{from: from, to: to})
					}
					mu.Unlock()
				}
			}
		}()
	}

	time.Sleep(loadWarmup)
	record.Store(true)
	time.Sleep(loadOnline)
	cancel()
	wg.Wait()

	lists.report(t, "online_friend_list")
	inbox.report(t, "online_recv_requests")
	apply.report(t, "online_apply")
	agree.report(t, "online_agree")
	return pairs
}

func probeRequestInsert(t *testing.T, ctx context.Context, st *store.Store, db *sql.DB) {
	t.Helper()
	const n = 8
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			from := benchID(6_900_001 + i*2)
			to := benchID(6_900_002 + i*2)
			errs[i] = st.AddFriendRequest(ctx, from, to, "probe", time.Now())
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Logf("PROBE insert %d %v", i, err)
		}
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM friend_request WHERE from_user_id LIKE 'b069%'`); err != nil {
		t.Fatal(err)
	}
}

func checkAgreed(t *testing.T, ctx context.Context, db *sql.DB, pairs []agreedPair) {
	t.Helper()
	if len(pairs) == 0 {
		t.Fatal("no agreed pairs were recorded")
	}
	for _, pair := range pairs {
		var ab, ba int
		err := db.QueryRowContext(ctx, `
			SELECT
				(SELECT COUNT(*) FROM friend WHERE owner_user_id = ? AND friend_user_id = ?),
				(SELECT COUNT(*) FROM friend WHERE owner_user_id = ? AND friend_user_id = ?)`,
			pair.from, pair.to, pair.to, pair.from).Scan(&ab, &ba)
		if err != nil || ab != 1 || ba != 1 {
			t.Fatalf("friend rows %s %s ab=%d ba=%d err=%v", pair.from, pair.to, ab, ba, err)
		}
		for _, owner := range []string{pair.from, pair.to} {
			var n, minV, maxV, head uint64
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*), MIN(version), MAX(version) FROM friend_version_log WHERE owner_user_id = ?`, owner).Scan(&n, &minV, &maxV); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRowContext(ctx, `SELECT version FROM friend_version WHERE owner_user_id = ?`, owner).Scan(&head); err != nil {
				t.Fatal(err)
			}
			if n != 1 || minV != 1 || maxV != 1 || head != 1 {
				t.Fatalf("version %s count=%d min=%d max=%d head=%d", owner, n, minV, maxV, head)
			}
		}
	}
	t.Logf("RESULT name=agreed_pairs checked=%d both_rows=1 version_gap=0", len(pairs))
}

func checkSamePair(t *testing.T, ctx context.Context, srv *logic.Server, db *sql.DB) {
	t.Helper()
	from, to := benchID(loadWriteBase-2), benchID(loadWriteBase-1)
	const racers = 16
	var wg sync.WaitGroup
	var okN, repeated, contention atomic.Int32
	var other atomic.Value
	wg.Add(racers)
	for i := 0; i < racers; i++ {
		go func() {
			defer wg.Done()
			_, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: from, ToUserId: to, Message: "race"})
			if err == nil {
				okN.Add(1)
				return
			}
			if code, ok := errorcode.CodeOf(err); ok && code == errorcode.RelationRequestRepeated {
				repeated.Add(1)
				return
			}
			if code, ok := errorcode.CodeOf(err); ok && code == errorcode.Internal {
				contention.Add(1)
				return
			}
			other.Store(err.Error())
		}()
	}
	wg.Wait()
	if other.Load() != nil {
		t.Fatalf("same pair other error %v", other.Load())
	}
	var rows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM friend_request WHERE from_user_id = ? AND to_user_id = ?`, from, to).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if okN.Load() != 1 || rows != 1 {
		t.Fatalf("same pair ok=%d repeated=%d contention=%d rows=%d", okN.Load(), repeated.Load(), contention.Load(), rows)
	}
	t.Logf("RESULT name=same_pair racers=%d ok=%d repeated=%d deadlock=%d rows=%d", racers, okN.Load(), repeated.Load(), contention.Load(), rows)
}

func runSync(t *testing.T, ctx context.Context, srv *logic.Server, st *store.Store, hot []loadUser) {
	t.Helper()
	heads := make([]versionHead, len(hot))
	for i, user := range hot {
		head, err := st.FriendVersion(ctx, user.id)
		if err != nil || head.Version < 500 {
			t.Fatalf("sync head %+v err=%v", head, err)
		}
		heads[i] = versionHead{id: head.ID, version: head.Version}
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var near, far, full latency
	var wg sync.WaitGroup
	for i := 0; i < loadSyncWorkers; i++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			r := rand.New(rand.NewPCG(uint64(seed+99), 7))
			for runCtx.Err() == nil {
				item := r.IntN(len(hot))
				user := hot[item]
				head := heads[item]
				start := time.Now()
				resp, err := srv.GetIncrementalFriends(runCtx, &relationpb.GetIncrementalFriendsReq{UserId: user.id, VersionId: head.id, Version: head.version - 50})
				if runCtx.Err() != nil {
					return
				}
				if err == nil && (resp.GetFull() || resp.GetSortVersion() == 0) {
					err = fmt.Errorf("near full=%v sort=%d", resp.GetFull(), resp.GetSortVersion())
				}
				near.add(time.Since(start), err)

				start = time.Now()
				resp, err = srv.GetIncrementalFriends(runCtx, &relationpb.GetIncrementalFriendsReq{UserId: user.id, VersionId: head.id, Version: head.version - 500})
				if runCtx.Err() != nil {
					return
				}
				if err == nil && !resp.GetFull() {
					err = errors.New("far incremental was not full")
				}
				far.add(time.Since(start), err)

				start = time.Now()
				ids, err := srv.GetFullFriendUserIDs(runCtx, &relationpb.GetFullFriendUserIDsReq{UserId: user.id})
				if runCtx.Err() != nil {
					return
				}
				if err == nil && len(ids.GetUserIds()) != user.friends {
					err = fmt.Errorf("full ids %d", len(ids.GetUserIds()))
				}
				full.add(time.Since(start), err)
			}
		}(i)
	}
	time.Sleep(loadSync)
	cancel()
	wg.Wait()
	near.report(t, "sync_incremental_near")
	far.report(t, "sync_incremental_far")
	full.report(t, "sync_full_ids")
}

type lockStat struct {
	waits  int64
	timeMs int64
	err    string
}

func readLockWaits(ctx context.Context, db *sql.DB) lockStat {
	waits, err1 := statusNumber(ctx, db, "Innodb_row_lock_waits")
	waitTime, err2 := statusNumber(ctx, db, "Innodb_row_lock_time")
	if err1 != nil || err2 != nil {
		return lockStat{err: errors.Join(err1, err2).Error()}
	}
	return lockStat{waits: waits, timeMs: waitTime}
}

func statusNumber(ctx context.Context, db *sql.DB, name string) (int64, error) {
	switch name {
	case "Innodb_row_lock_waits", "Innodb_row_lock_time":
	default:
		return 0, fmt.Errorf("unknown status %s", name)
	}
	var key, value string
	if err := db.QueryRowContext(ctx, "SHOW GLOBAL STATUS LIKE '"+name+"'").Scan(&key, &value); err != nil {
		return 0, err
	}
	var n int64
	_, err := fmt.Sscan(value, &n)
	return n, err
}

func truncateBench(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	for _, table := range []string{"friend", "friend_request", "black", "friend_version_log", "friend_version"} {
		if _, err := db.ExecContext(ctx, "TRUNCATE TABLE "+table); err != nil {
			t.Fatalf("truncate %s: %v", table, err)
		}
	}
}
