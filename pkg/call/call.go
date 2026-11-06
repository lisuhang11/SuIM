// Package call stores one-to-one call signaling in Redis.
// The media itself is not handled here.
package call

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

const (
	// Prefix is the production key prefix. A call id lives at Prefix+"id:"+callID,
	// a busy lock at Prefix+"busy:"+userID, and the ringing index at Prefix+"ringing".
	Prefix = "suim:call:"
	// Channel is the production pub/sub channel. Every msggateway delivers the payload
	// to sockets it still holds.
	Channel = Prefix + "signal"

	// OpInvite and the ops below are the signaling steps carried in an event.
	OpInvite   = "invite"
	OpCancel   = "cancel"
	OpAccept   = "accept"
	OpReject   = "reject"
	OpHangup   = "hangup"
	OpRing     = "ring"
	OpTimeout  = "timeout"
	OpBusy     = "busy"
	OpAnswered = "answered"

	// MediaAudio and MediaVideo are the only call kinds.
	MediaAudio = "audio"
	MediaVideo = "video"

	stateRinging  = "ringing"
	stateAccepted = "accepted"

	ringWait    = 45 * time.Second
	ringTTL     = 70 * time.Second
	acceptedTTL = 30 * time.Minute
)

// Signal is one signaling step stored or published by the board.
type Signal struct {
	Op       string
	CallID   string
	CallerID string
	CalleeID string
	Media    string
	Reason   string
}

// Event is pushed to every gateway. Except is a connection id that should not
// also receive the push, because that connection already got the 1004 reply.
type Event struct {
	Users  []string `json:"users"`
	Except string   `json:"except,omitempty"`
	Op     string   `json:"op"`
	CallID string   `json:"callId"`
	Caller string   `json:"caller"`
	Callee string   `json:"callee"`
	Media  string   `json:"media"`
	Reason string   `json:"reason,omitempty"`
}

// Board is the shared call record.
type Board struct {
	rdb         redis.UniversalClient
	prefix      string
	ring        time.Duration
	ringTTL     time.Duration
	acceptedTTL time.Duration
}

func New(rdb redis.UniversalClient) *Board {
	return newBoard(rdb, Prefix)
}

func newBoard(rdb redis.UniversalClient, prefix string) *Board {
	return &Board{
		rdb:         rdb,
		prefix:      prefix,
		ring:        ringWait,
		ringTTL:     ringTTL,
		acceptedTTL: acceptedTTL,
	}
}

// Invite locks both users and records a ringing call.
func (b *Board) Invite(ctx context.Context, caller, callee, media string) (Signal, error) {
	sig := Signal{Op: OpInvite, CallerID: caller, CalleeID: callee, Media: media}
	if b == nil || b.rdb == nil {
		return sig, errorcode.ErrInternal.WithMsg("call signaling is not configured")
	}
	if !validUser(caller) || !validUser(callee) || caller == callee || (media != MediaAudio && media != MediaVideo) {
		return sig, errorcode.ErrCallInvalid
	}
	callID, err := newCallID()
	if err != nil {
		return sig, errorcode.ErrInternal.WithMsg("create call id")
	}
	sig.CallID = callID
	now := time.Now()
	fields, err := b.rdb.Eval(ctx, inviteScript, []string{
		b.busyKey(caller), b.busyKey(callee), b.callKey(callID), b.ringKey(),
	}, caller, callee, callID, media, now.Add(b.ring).Unix(), int64(b.ringTTL/time.Second)).StringSlice()
	if err != nil {
		return sig, err
	}
	if len(fields) == 0 {
		return sig, errorcode.ErrInternal.WithMsg("invite call")
	}
	switch fields[0] {
	case "ok":
		return sig, nil
	case "self":
		return sig, errorcode.ErrCallBusy
	case "peer":
		sig.Op = OpBusy
		sig.Reason = "peer"
		return sig, errorcode.ErrCallPeerBusy
	default:
		return sig, errorcode.ErrInternal.WithMsg("invite call")
	}
}

// Cancel stops a ringing call. Only the caller can cancel.
func (b *Board) Cancel(ctx context.Context, userID, callID string) (Signal, error) {
	return b.end(ctx, endScript, OpCancel, userID, callID)
}

// Reject stops a ringing call. Only the callee can reject.
func (b *Board) Reject(ctx context.Context, userID, callID string) (Signal, error) {
	return b.end(ctx, rejectScript, OpReject, userID, callID)
}

// Accept moves a ringing call to accepted. Only the callee can accept.
func (b *Board) Accept(ctx context.Context, userID, callID string) (Signal, error) {
	sig := Signal{Op: OpAccept, CallID: callID}
	fields, err := b.eval(ctx, acceptScript, userID, callID, int64(b.acceptedTTL/time.Second))
	if err != nil {
		return sig, err
	}
	return fill(sig, fields)
}

// Hangup ends an accepted call. Either participant can hang up.
func (b *Board) Hangup(ctx context.Context, userID, callID string) (Signal, error) {
	return b.end(ctx, hangupScript, OpHangup, userID, callID)
}

// Expire ends ringing calls whose deadline has passed. A lost race returns nothing for that id.
func (b *Board) Expire(ctx context.Context, now time.Time) ([]Signal, error) {
	if b == nil || b.rdb == nil {
		return nil, nil
	}
	ids, err := b.rdb.ZRangeByScore(ctx, b.ringKey(), &redis.ZRangeBy{
		Min: "-inf", Max: strconvFormat(now.Unix()), Count: 20,
	}).Result()
	if err != nil {
		return nil, err
	}
	out := make([]Signal, 0, len(ids))
	for _, callID := range ids {
		sig, err := b.end(ctx, timeoutScript, OpTimeout, "", callID)
		if err != nil {
			if errors.Is(err, errorcode.ErrCallNotFound) || errors.Is(err, errorcode.ErrCallState) {
				continue
			}
			return out, err
		}
		out = append(out, sig)
	}
	return out, nil
}

// Publish broadcasts one event to every gateway subscribed to this board.
func (b *Board) Publish(ctx context.Context, ev Event) error {
	if b == nil || b.rdb == nil || len(ev.Users) == 0 {
		return nil
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return b.rdb.Publish(ctx, b.channel(), body).Err()
}

// Subscribe calls fn for each event until ctx is cancelled.
func (b *Board) Subscribe(ctx context.Context, fn func(Event)) {
	if b == nil || b.rdb == nil || fn == nil {
		return
	}
	for ctx.Err() == nil {
		pubsub := b.rdb.Subscribe(ctx, b.channel())
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
				var ev Event
				if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
					slog.Warn("call signal payload", "err", err)
					continue
				}
				fn(ev)
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

func (b *Board) end(ctx context.Context, script, op, userID, callID string) (Signal, error) {
	sig := Signal{Op: op, CallID: callID}
	fields, err := b.eval(ctx, script, userID, callID, 0)
	if err != nil {
		return sig, err
	}
	return fill(sig, fields)
}

func (b *Board) eval(ctx context.Context, script, userID, callID string, extra int64) ([]string, error) {
	if b == nil || b.rdb == nil {
		return nil, errorcode.ErrInternal.WithMsg("call signaling is not configured")
	}
	if !validCallID(callID) || (userID != "" && !validUser(userID)) {
		return nil, errorcode.ErrCallInvalid
	}
	fields, err := b.rdb.Eval(ctx, script, []string{
		b.callKey(callID), b.ringKey(), b.busyKeySlot(), callID,
	}, userID, callID, extra, b.prefix).StringSlice()
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, errorcode.ErrInternal.WithMsg("update call")
	}
	switch fields[0] {
	case "ok":
		return fields, nil
	case "missing":
		return nil, errorcode.ErrCallNotFound
	case "forbidden":
		return nil, errorcode.ErrCallNotMember
	case "state":
		return nil, errorcode.ErrCallState
	default:
		return nil, errorcode.ErrInternal.WithMsg("update call")
	}
}

func fill(sig Signal, fields []string) (Signal, error) {
	if len(fields) < 4 {
		return sig, errorcode.ErrInternal.WithMsg("update call")
	}
	sig.CallerID = fields[1]
	sig.CalleeID = fields[2]
	sig.Media = fields[3]
	if len(fields) > 4 {
		sig.Reason = fields[4]
	}
	return sig, nil
}

func (b *Board) callKey(id string) string { return b.prefix + "id:" + id }
func (b *Board) busyKey(userID string) string {
	return b.prefix + "busy:" + userID
}
func (b *Board) ringKey() string { return b.prefix + "ringing" }
func (b *Board) channel() string { return b.prefix + "signal" }

// busyKeySlot is unused as a key. Scripts build busy keys from the prefix argument
// so a call's two users are not known until the hash is read.
func (b *Board) busyKeySlot() string { return b.prefix + "busy" }

func newCallID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func validCallID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, r := range id {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func validUser(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if r < 0x20 || r == ':' {
			return false
		}
	}
	return true
}

func strconvFormat(n int64) string {
	return jsonNumber(n)
}

func jsonNumber(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

const inviteScript = `
local okCaller = redis.call("SET", KEYS[1], ARGV[3], "EX", ARGV[6], "NX")
if not okCaller then
	return {"self"}
end
local okCallee = redis.call("SET", KEYS[2], ARGV[3], "EX", ARGV[6], "NX")
if not okCallee then
	redis.call("DEL", KEYS[1])
	return {"peer"}
end
redis.call("HSET", KEYS[3], "caller", ARGV[1], "callee", ARGV[2], "media", ARGV[4], "state", "ringing")
redis.call("EXPIRE", KEYS[3], ARGV[6])
redis.call("ZADD", KEYS[4], ARGV[5], ARGV[3])
return {"ok"}
`

// endScript cancels. ARGV[1] is the caller. The timeout script reuses the body with an empty user.
const endScript = `
local state = redis.call("HGET", KEYS[1], "state")
if not state then
	redis.call("ZREM", KEYS[2], ARGV[2])
	return {"missing"}
end
local caller = redis.call("HGET", KEYS[1], "caller")
local callee = redis.call("HGET", KEYS[1], "callee")
local media = redis.call("HGET", KEYS[1], "media")
if ARGV[1] ~= caller then
	return {"forbidden"}
end
if state ~= "ringing" then
	return {"state"}
end
redis.call("DEL", KEYS[1], ARGV[4] .. "busy:" .. caller, ARGV[4] .. "busy:" .. callee)
redis.call("ZREM", KEYS[2], ARGV[2])
return {"ok", caller, callee, media, "cancel"}
`

const rejectScript = `
local state = redis.call("HGET", KEYS[1], "state")
if not state then
	redis.call("ZREM", KEYS[2], ARGV[2])
	return {"missing"}
end
local caller = redis.call("HGET", KEYS[1], "caller")
local callee = redis.call("HGET", KEYS[1], "callee")
local media = redis.call("HGET", KEYS[1], "media")
if ARGV[1] ~= callee then
	return {"forbidden"}
end
if state ~= "ringing" then
	return {"state"}
end
redis.call("DEL", KEYS[1], ARGV[4] .. "busy:" .. caller, ARGV[4] .. "busy:" .. callee)
redis.call("ZREM", KEYS[2], ARGV[2])
return {"ok", caller, callee, media, "reject"}
`

const acceptScript = `
local state = redis.call("HGET", KEYS[1], "state")
if not state then
	redis.call("ZREM", KEYS[2], ARGV[2])
	return {"missing"}
end
local caller = redis.call("HGET", KEYS[1], "caller")
local callee = redis.call("HGET", KEYS[1], "callee")
local media = redis.call("HGET", KEYS[1], "media")
if ARGV[1] ~= callee then
	return {"forbidden"}
end
if state ~= "ringing" then
	return {"state"}
end
redis.call("HSET", KEYS[1], "state", "accepted")
redis.call("EXPIRE", KEYS[1], ARGV[3])
redis.call("EXPIRE", ARGV[4] .. "busy:" .. caller, ARGV[3])
redis.call("EXPIRE", ARGV[4] .. "busy:" .. callee, ARGV[3])
redis.call("ZREM", KEYS[2], ARGV[2])
return {"ok", caller, callee, media}
`

const hangupScript = `
local state = redis.call("HGET", KEYS[1], "state")
if not state then
	return {"missing"}
end
local caller = redis.call("HGET", KEYS[1], "caller")
local callee = redis.call("HGET", KEYS[1], "callee")
local media = redis.call("HGET", KEYS[1], "media")
if ARGV[1] ~= caller and ARGV[1] ~= callee then
	return {"forbidden"}
end
if state ~= "accepted" then
	return {"state"}
end
redis.call("DEL", KEYS[1], ARGV[4] .. "busy:" .. caller, ARGV[4] .. "busy:" .. callee)
return {"ok", caller, callee, media, "hangup"}
`

const timeoutScript = `
local state = redis.call("HGET", KEYS[1], "state")
if not state then
	redis.call("ZREM", KEYS[2], ARGV[2])
	return {"missing"}
end
local caller = redis.call("HGET", KEYS[1], "caller")
local callee = redis.call("HGET", KEYS[1], "callee")
local media = redis.call("HGET", KEYS[1], "media")
if state ~= "ringing" then
	redis.call("ZREM", KEYS[2], ARGV[2])
	return {"state"}
end
redis.call("DEL", KEYS[1], ARGV[4] .. "busy:" .. caller, ARGV[4] .. "busy:" .. callee)
redis.call("ZREM", KEYS[2], ARGV[2])
return {"ok", caller, callee, media, "timeout"}
`
