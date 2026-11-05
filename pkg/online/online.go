package online

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Prefix is the sorted-set key for one user's online platforms.
	Prefix = "suim:online:"
	// Channel carries a membership change. The payload is platform IDs joined by
	// ":", with the user ID as the last field. A user who just went fully offline
	// is sent as the user ID alone.
	Channel = "suim:online:change"
	// Expire is how long a platform stays online without a renewal.
	Expire = 30 * time.Minute
)

// Key is the Redis key for one user.
func Key(userID string) string { return Prefix + userID }

// Status is the shared online record. Each user is one sorted set: the member
// is the platform ID and the score is the unix time when that platform expires.
type Status struct {
	rdb    redis.UniversalClient
	expire time.Duration
}

func New(rdb redis.UniversalClient) *Status {
	return &Status{rdb: rdb, expire: Expire}
}

// Set adds online platforms, removes offline platforms, and drops members that
// have already expired. A renewal of a platform that is already present does
// not publish. The returned platforms are the members left after a real change.
func (s *Status) Set(ctx context.Context, userID string, online, offline []int32) (bool, []int32, error) {
	if s == nil || s.rdb == nil || userID == "" {
		return false, nil, nil
	}
	now := time.Now()
	argv := make([]any, 0, 4+len(online)+len(offline))
	argv = append(argv, int32(s.expire/time.Second), now.Unix(), now.Add(s.expire).Unix(), int32(len(offline)))
	for _, platformID := range offline {
		argv = append(argv, platformID)
	}
	for _, platformID := range online {
		argv = append(argv, platformID)
	}
	fields, err := s.rdb.Eval(ctx, setScript, []string{Key(userID)}, argv...).StringSlice()
	if err != nil {
		return false, nil, err
	}
	if len(fields) == 0 || fields[len(fields)-1] == "0" {
		return false, nil, nil
	}
	raw := fields[:len(fields)-1]
	platformIDs := make([]int32, 0, len(raw))
	for _, field := range raw {
		id, err := strconv.Atoi(field)
		if err != nil {
			return false, nil, err
		}
		platformIDs = append(platformIDs, int32(id))
	}
	payload := make([]string, 0, len(raw)+1)
	payload = append(payload, raw...)
	payload = append(payload, userID)
	if err := s.rdb.Publish(ctx, Channel, strings.Join(payload, ":")).Err(); err != nil {
		return true, platformIDs, err
	}
	return true, platformIDs, nil
}

// Get returns platform IDs whose expiry is still in the future.
func (s *Status) Get(ctx context.Context, userID string) ([]int32, error) {
	if s == nil || s.rdb == nil || userID == "" {
		return nil, nil
	}
	return s.members(ctx, Key(userID))
}

// List returns every user who still has an unexpired platform.
func (s *Status) List(ctx context.Context) (map[string][]int32, error) {
	if s == nil || s.rdb == nil {
		return map[string][]int32{}, nil
	}
	out := map[string][]int32{}
	var cursor uint64
	for {
		keys, next, err := s.rdb.Scan(ctx, cursor, Prefix+"*", 200).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			platformIDs, err := s.members(ctx, key)
			if err != nil {
				return nil, err
			}
			if len(platformIDs) == 0 {
				continue
			}
			out[strings.TrimPrefix(key, Prefix)] = platformIDs
		}
		cursor = next
		if cursor == 0 {
			return out, nil
		}
	}
}

// Subscribe calls fn for each membership change until ctx is cancelled.
func (s *Status) Subscribe(ctx context.Context, fn func(userID string, platformIDs []int32)) {
	if s == nil || s.rdb == nil || fn == nil {
		return
	}
	for ctx.Err() == nil {
		pubsub := s.rdb.Subscribe(ctx, Channel)
		ch := pubsub.Channel()
		closed := false
		for !closed && ctx.Err() == nil {
			select {
			case <-ctx.Done():
				closed = true
			case msg, ok := <-ch:
				if !ok {
					closed = true
					break
				}
				userID, platformIDs, err := Parse(msg.Payload)
				if err != nil {
					slog.Warn("online status payload", "err", err)
					continue
				}
				fn(userID, platformIDs)
			}
		}
		_ = pubsub.Close()
		if ctx.Err() != nil {
			return
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// Parse splits a change payload into the user ID and the platforms still online.
func Parse(payload string) (string, []int32, error) {
	parts := strings.Split(payload, ":")
	if len(parts) == 0 || parts[len(parts)-1] == "" {
		return "", nil, errors.New("invalid online payload")
	}
	userID := parts[len(parts)-1]
	platformIDs := make([]int32, 0, len(parts)-1)
	for _, part := range parts[:len(parts)-1] {
		id, err := strconv.Atoi(part)
		if err != nil {
			return "", nil, err
		}
		platformIDs = append(platformIDs, int32(id))
	}
	return userID, platformIDs, nil
}

func (s *Status) members(ctx context.Context, key string) ([]int32, error) {
	raw, err := s.rdb.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: strconv.FormatInt(time.Now().Unix(), 10),
		Max: "+inf",
	}).Result()
	if err != nil {
		return nil, err
	}
	platformIDs := make([]int32, 0, len(raw))
	for _, member := range raw {
		id, err := strconv.Atoi(member)
		if err != nil {
			return nil, err
		}
		platformIDs = append(platformIDs, int32(id))
	}
	return platformIDs, nil
}

const setScript = `
local key = KEYS[1]
local score = ARGV[3]
local before = redis.call("ZCARD", key)
redis.call("ZREMRANGEBYSCORE", key, "-inf", ARGV[2])
for i = 5, tonumber(ARGV[4]) + 4 do
	redis.call("ZREM", key, ARGV[i])
end
local removed = redis.call("ZCARD", key)
for i = 5 + tonumber(ARGV[4]), #ARGV do
	redis.call("ZADD", key, score, ARGV[i])
end
local left = redis.call("ZCARD", key)
if left == 0 then
	redis.call("DEL", key)
else
	redis.call("EXPIRE", key, ARGV[1])
end
local changed = (before ~= removed) or (removed ~= left)
if changed then
	local members = redis.call("ZRANGE", key, 0, -1)
	table.insert(members, "1")
	return members
end
return {"0"}
`
