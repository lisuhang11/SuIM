//go:build load

package main

import (
	"context"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"math/rand/v2"

	grouppb "github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/logic"
	"github.com/lisuhang11/SuIM/services/group/internal/store"
)

func TestLoadGroup(t *testing.T) {
	ctx, db, st, rdb := openGroupBench(t)
	started := time.Now()
	seedGroupBench(t, ctx, db)
	t.Logf("RESULT name=seed ms=%.3f members=%d groups=%d requests=%d",
		float64(time.Since(started).Microseconds())/1000, groupBenchMembers, groupBenchGroups+1, groupBenchRequests)

	cache, stop := store.NewReadCache(ctx, rdb, store.CacheConfig{
		Topic: "suim:group:bench:load", SlotNum: 100, SlotSize: 2000, SuccessExpire: 300 * time.Second, FailedExpire: 5 * time.Second,
	})
	defer stop()
	st.UseCache(cache)
	srv := logic.New(st, benchUsers{}, nil)
	big := groupBenchGroup(1)
	owner := groupBenchUser(1)
	head, err := st.MemberVersion(ctx, big)
	if err != nil {
		t.Fatal(err)
	}
	mustGroups(t, srv, ctx, big, groupBenchMembers)

	readCtx, cancelRead := context.WithTimeout(ctx, 8*time.Second)
	defer cancelRead()
	var readers sync.WaitGroup
	stats := map[string]*callLatency{
		"group_info": {}, "member_page": {}, "member_ids": {}, "joined_page": {}, "incremental_near": {},
	}
	for i := 0; i < 16; i++ {
		readers.Add(1)
		go func(seed uint64) {
			defer readers.Done()
			r := rand.New(rand.NewPCG(seed, 1))
			for readCtx.Err() == nil {
				switch r.IntN(5) {
				case 0:
					start := time.Now()
					_, err := srv.GetGroupsInfo(readCtx, &grouppb.GetGroupsInfoReq{GroupIds: []string{big}})
					stats["group_info"].add(time.Since(start), err, readCtx.Err() != nil)
				case 1:
					start := time.Now()
					_, err := srv.GetGroupMembers(readCtx, &grouppb.GetGroupMembersReq{GroupId: big, UserId: owner, Limit: 20})
					stats["member_page"].add(time.Since(start), err, readCtx.Err() != nil)
				case 2:
					start := time.Now()
					_, err := srv.GetFullGroupMemberUserIDs(readCtx, &grouppb.GetFullGroupMemberUserIDsReq{GroupId: big, UserId: owner})
					stats["member_ids"].add(time.Since(start), err, readCtx.Err() != nil)
				case 3:
					start := time.Now()
					_, err := srv.GetJoinedGroups(readCtx, &grouppb.GetJoinedGroupsReq{UserId: owner, Limit: 20})
					stats["joined_page"].add(time.Since(start), err, readCtx.Err() != nil)
				default:
					start := time.Now()
					_, err := srv.GetIncrementalGroupMembers(readCtx, &grouppb.GetIncrementalGroupMembersReq{GroupId: big, UserId: owner, VersionId: head.ID, Version: head.Version - 50})
					stats["incremental_near"].add(time.Since(start), err, readCtx.Err() != nil)
				}
			}
		}(uint64(i + 1))
	}

	writeCtx, cancelWrite := context.WithTimeout(ctx, 5*time.Second)
	defer cancelWrite()
	writes := &callLatency{}
	var writers sync.WaitGroup
	for w := 0; w < 4; w++ {
		writers.Add(1)
		go func(worker int) {
			defer writers.Done()
			n := 0
			for writeCtx.Err() == nil {
				n++
				user := groupBenchUser(2 + worker*50 + (n % 50))
				nick := "n" + user
				start := time.Now()
				_, err := srv.SetGroupMemberInfo(writeCtx, &grouppb.SetGroupMemberInfoReq{
					OpUserId: owner,
					Members: []*grouppb.SetGroupMemberInfo{{
						GroupId: big, UserId: user, Nickname: &nick,
					}},
				})
				writes.add(time.Since(start), err, writeCtx.Err() != nil)
			}
		}(w)
	}
	writers.Wait()
	readers.Wait()
	for name, stat := range stats {
		stat.report(t, name)
	}
	writes.report(t, "rename")
}

type callLatency struct {
	mu         sync.Mutex
	samples    []time.Duration
	errs       int
	contention int
	first      string
}

func (l *callLatency) add(d time.Duration, err error, cancelled bool) {
	if cancelled && err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err != nil && (strings.Contains(err.Error(), "1213") || strings.Contains(err.Error(), "Deadlock") || strings.Contains(err.Error(), "1205") || strings.Contains(err.Error(), "Lock wait timeout")) {
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

func (l *callLatency) report(t *testing.T, name string) {
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
