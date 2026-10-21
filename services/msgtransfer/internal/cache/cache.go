package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"

	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

const (
	seqKeyPrefix    = "suim:seq:conv:"
	msgKeyPrefix    = "suim:msg:"
	clientKeyPrefix = "suim:msg:client:"
	readKeyPrefix   = "suim:seq:read:"
)

// seqIncr keeps the greater read sequence.
var seqIncr = redis.NewScript(`
local cur = tonumber(redis.call('GET', KEYS[1]) or '0')
local seq = tonumber(ARGV[1])
if seq > cur then
  redis.call('SET', KEYS[1], seq)
end
return 1
`)

// Cache stores conversation sequences and recent message bodies.
type Cache struct {
	rdb        redis.UniversalClient
	msgTTL     time.Duration
	loadMaxSeq func(context.Context, string) (int64, error)
}

func New(rdb redis.UniversalClient, msgTTL time.Duration, loadMaxSeq func(context.Context, string) (int64, error)) *Cache {
	if msgTTL <= 0 {
		msgTTL = 24 * time.Hour
	}
	return &Cache{rdb: rdb, msgTTL: msgTTL, loadMaxSeq: loadMaxSeq}
}

// Malloc reserves n sequence numbers and returns the max before this reservation.
func (c *Cache) Malloc(ctx context.Context, conversationID string, n int64) (int64, error) {
	if n <= 0 {
		return 0, fmt.Errorf("malloc count must be positive")
	}
	if err := c.ensureSeq(ctx, conversationID); err != nil {
		return 0, err
	}
	next, err := c.rdb.IncrBy(ctx, seqKeyPrefix+conversationID, n).Result()
	if err != nil {
		return 0, err
	}
	return next - n, nil
}

func (c *Cache) ensureSeq(ctx context.Context, conversationID string) error {
	key := seqKeyPrefix + conversationID
	n, err := c.rdb.Exists(ctx, key).Result()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	var max int64
	if c.loadMaxSeq != nil {
		max, err = c.loadMaxSeq(ctx, conversationID)
		if err != nil {
			return err
		}
	}
	return c.rdb.SetNX(ctx, key, max, 0).Err()
}

// ClientSeq returns a sequence already assigned to this client message id.
func (c *Cache) ClientSeq(ctx context.Context, conversationID, clientMsgID string) (int64, bool, error) {
	if clientMsgID == "" {
		return 0, false, nil
	}
	v, err := c.rdb.Get(ctx, clientKey(conversationID, clientMsgID)).Result()
	if err == redis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	seq, err := strconv.ParseInt(v, 10, 64)
	if err != nil || seq <= 0 {
		return 0, false, nil
	}
	return seq, true, nil
}

// RememberClientSeq records the sequence chosen for a client message id.
func (c *Cache) RememberClientSeq(ctx context.Context, conversationID, clientMsgID string, seq int64) error {
	if clientMsgID == "" || seq <= 0 {
		return nil
	}
	return c.rdb.Set(ctx, clientKey(conversationID, clientMsgID), seq, c.msgTTL).Err()
}

// SetMessages stores message bodies so a later read can skip MySQL.
func (c *Cache) SetMessages(ctx context.Context, conversationID string, msgs []*msgpb.MsgData) error {
	if len(msgs) == 0 {
		return nil
	}
	_, err := c.rdb.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, msg := range msgs {
			if msg == nil || msg.Seq <= 0 {
				continue
			}
			body, err := proto.Marshal(msg)
			if err != nil {
				return err
			}
			pipe.Set(ctx, msgKey(conversationID, msg.Seq), body, c.msgTTL)
		}
		return nil
	})
	return err
}

// SetUserReadSeq raises each user's read sequence when the new value is greater.
func (c *Cache) SetUserReadSeq(ctx context.Context, conversationID string, userSeq map[string]int64) error {
	for userID, seq := range userSeq {
		if userID == "" || seq <= 0 {
			continue
		}
		if err := seqIncr.Run(ctx, c.rdb, []string{readKey(conversationID, userID)}, seq).Err(); err != nil {
			return err
		}
	}
	return nil
}

func clientKey(conversationID, clientMsgID string) string {
	return clientKeyPrefix + conversationID + ":" + clientMsgID
}

func msgKey(conversationID string, seq int64) string {
	return fmt.Sprintf("%s%s:%d", msgKeyPrefix, conversationID, seq)
}

func readKey(conversationID, userID string) string {
	return readKeyPrefix + conversationID + ":" + userID
}
