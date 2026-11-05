//go:build load

package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"math/rand/v2"

	userpb "github.com/lisuhang11/SuIM/proto/user"
	"github.com/lisuhang11/SuIM/services/user/internal/logic"
)

const (
	userLoadReaders = 16
	userLoadWriters = 4
	userLoadRead    = 8 * time.Second
	userLoadLogin   = 5 * time.Second
)

func TestLoadUser(t *testing.T) {
	ctx, db, st, rdb, issuer := openUserBench(t)
	seedUserBench(t, ctx, db)
	hot := userBenchID(userBenchHot)
	raw := insertBenchToken(t, ctx, db, issuer, hot)
	var mu sync.Mutex
	var issued []string
	t.Cleanup(func() {
		mu.Lock()
		raws := append(append([]string{}, issued...), raw)
		mu.Unlock()
		deleteBenchTokenKeys(t, ctx, rdb, raws...)
	})

	hits := &tokenHits{}
	srv := logic.New(st, issuer, newBenchCache(hits))
	for i := 0; i < 50; i++ {
		mustValidate(t, ctx, srv, raw, hot)
	}
	hits.hit.Store(0)
	hits.miss.Store(0)
	hits.failed.Store(0)

	var getUser, validate callLatency
	readCtx, readCancel := context.WithTimeout(ctx, userLoadRead)
	var wg sync.WaitGroup
	for i := 0; i < userLoadReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for readCtx.Err() == nil {
				start := time.Now()
				if rand.IntN(5) == 0 {
					id := userBenchID(rand.IntN(userBenchN) + 1)
					_, err := srv.GetUserByID(readCtx, &userpb.GetUserByIDReq{UserId: id})
					if readCtx.Err() != nil {
						return
					}
					getUser.add(time.Since(start), err)
					continue
				}
				_, err := srv.ValidateToken(readCtx, &userpb.ValidateTokenReq{Token: raw})
				if readCtx.Err() != nil {
					return
				}
				validate.add(time.Since(start), err)
			}
		}()
	}
	wg.Wait()
	readCancel()
	getUser.report(t, "get_user")
	validate.report(t, "validate")
	t.Logf("RESULT name=validate_cache %s", hits.String())
	if hits.hit.Load() == 0 {
		t.Fatal("token cache recorded no local hit")
	}

	var login callLatency
	loginCtx, loginCancel := context.WithTimeout(ctx, userLoadLogin)
	for i := 0; i < userLoadWriters; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			n := 0
			for loginCtx.Err() == nil {
				n++
				id := userBenchID(worker*50 + (n % 50) + 1)
				start := time.Now()
				resp, err := srv.Login(loginCtx, &userpb.LoginReq{
					Account: id + "@bench.test", Password: userBenchPass, PlatformId: 3,
				})
				if loginCtx.Err() != nil {
					return
				}
				login.add(time.Since(start), err)
				if err == nil {
					mu.Lock()
					issued = append(issued, resp.GetToken())
					mu.Unlock()
				}
			}
		}(i)
	}
	wg.Wait()
	loginCancel()
	login.report(t, "login")
}
