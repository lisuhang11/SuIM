package store

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limit is one fixed window. N is how much this call adds.
type Limit struct {
	Key   string
	Limit int64
	N     int64
	TTL   time.Duration
}

// Quota counts create, invite, and join attempts in Redis.
type Quota struct {
	rdb redis.UniversalClient
}

func NewQuota(rdb redis.UniversalClient) *Quota {
	if rdb == nil {
		return nil
	}
	return &Quota{rdb: rdb}
}

// Reserve adds N to every window. A window that would pass its limit is left unchanged.
// Redis errors are returned as-is. ErrLimited means one window is full.
func (q *Quota) Reserve(ctx context.Context, limits []Limit) error {
	if q == nil || q.rdb == nil {
		return nil
	}
	for _, lim := range limits {
		if lim.Key == "" || lim.Limit <= 0 {
			continue
		}
		n := lim.N
		if n < 1 {
			n = 1
		}
		sec := int64(lim.TTL / time.Second)
		if sec < 1 {
			sec = 1
		}
		got, err := reserveScript.Run(ctx, q.rdb, []string{lim.Key}, n, lim.Limit, sec).Int64()
		if err != nil {
			return err
		}
		if got == 0 {
			return ErrLimited
		}
	}
	return nil
}

var reserveScript = redis.NewScript(`
local n = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])
local next = redis.call('INCRBY', KEYS[1], n)
if next == n then
  redis.call('EXPIRE', KEYS[1], ttl)
end
if next > limit then
  redis.call('DECRBY', KEYS[1], n)
  return 0
end
return 1
`)
