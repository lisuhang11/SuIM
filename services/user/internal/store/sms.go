package store

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	smsMissing  = "missing"
	smsOK       = "ok"
	smsMismatch = "mismatch"
	smsLocked   = "locked"
)

// SMSLimit is one counter window.
type SMSLimit struct {
	Key   string
	Limit int64
	TTL   time.Duration
}

// ReserveSMS increments every counter. A later send failure still counts.
// An empty key is skipped. Returns ErrSMSLimited when any window is full.
func (s *Store) ReserveSMS(ctx context.Context, limits []SMSLimit) error {
	for _, lim := range limits {
		if lim.Key == "" || lim.Limit <= 0 {
			continue
		}
		sec := int64(lim.TTL / time.Second)
		if sec < 1 {
			sec = 1
		}
		n, err := reserveScript.Run(ctx, s.rdb, []string{lim.Key}, lim.Limit, sec).Int64()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrSMSLimited
		}
	}
	return nil
}

func (s *Store) SMSBlocked(ctx context.Context, keys []string) (bool, error) {
	var existing []string
	for _, key := range keys {
		if key != "" {
			existing = append(existing, key)
		}
	}
	if len(existing) == 0 {
		return false, nil
	}
	vals, err := s.rdb.MGet(ctx, existing...).Result()
	if err != nil {
		return false, err
	}
	for _, v := range vals {
		if v != nil {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) SaveSMSCode(ctx context.Context, key, codeHash string, ttl time.Duration) error {
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, key, map[string]any{
		"hash":     codeHash,
		"attempts": 0,
		"status":   "pending",
	})
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// ConsumeSMSCode verifies and burns a code. A repeated correct code inside the
// retry window returns the same receipt.
func (s *Store) ConsumeSMSCode(ctx context.Context, key, codeHash, receipt string, maxAttempts int, retryTTL time.Duration) (string, error) {
	sec := int64(retryTTL / time.Second)
	if sec < 1 {
		sec = 1
	}
	res, err := consumeScript.Run(ctx, s.rdb, []string{key}, codeHash, maxAttempts, sec, receipt, time.Now().Unix()).Result()
	if err != nil {
		return "", err
	}
	parts, ok := res.([]any)
	if !ok || len(parts) < 1 {
		return "", fmt.Errorf("sms consume: bad redis reply")
	}
	status, _ := parts[0].(string)
	switch status {
	case smsOK:
		if len(parts) < 2 {
			return "", fmt.Errorf("sms consume: missing receipt")
		}
		got, _ := parts[1].(string)
		if got == "" {
			return "", fmt.Errorf("sms consume: empty receipt")
		}
		return got, nil
	case smsMissing, smsMismatch, smsLocked:
		return "", ErrSMSCode
	default:
		return "", fmt.Errorf("sms consume: %s", status)
	}
}

var reserveScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[2])
end
if n > tonumber(ARGV[1]) then
  return 0
end
return 1
`)

var consumeScript = redis.NewScript(`
local data = redis.call('HMGET', KEYS[1], 'hash', 'attempts', 'status', 'receipt')
if not data[1] then
  return {'missing', ''}
end
local hash, attempts, status, receipt = data[1], tonumber(data[2]) or 0, data[3], data[4]
if status == 'used' then
  if hash == ARGV[1] then
    return {'ok', receipt}
  end
  return {'mismatch', ''}
end
if hash ~= ARGV[1] then
  attempts = redis.call('HINCRBY', KEYS[1], 'attempts', 1)
  if attempts >= tonumber(ARGV[2]) then
    redis.call('DEL', KEYS[1])
    return {'locked', ''}
  end
  return {'mismatch', ''}
end
redis.call('HSET', KEYS[1], 'status', 'used', 'receipt', ARGV[4], 'used_at', ARGV[5])
redis.call('EXPIRE', KEYS[1], ARGV[3])
return {'ok', ARGV[4]}
`)
