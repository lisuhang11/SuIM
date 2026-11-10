package call

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

const (
	memberJoined  = "joined"
	memberRinging = "ringing"

	// ReasonEnded means the group room is gone. ReasonUser means one invitee stopped ringing.
	ReasonEnded = "ended"
	ReasonUser  = "user"
)

// GroupStep is one change to a group voice room.
// Joined is who is in the room after the change. Ringing is who is still being called.
// When Ended is set, both lists are the people who were in that state just before the room was removed.
type GroupStep struct {
	CallID   string
	GroupID  string
	CallerID string
	Media    string
	Actor    string
	Joined   []string
	Ringing  []string
	Busy     []string
	Ended    bool
}

// Kind reports whether callID is a private call or a group voice room.
func (b *Board) Kind(ctx context.Context, callID string) (string, error) {
	if b == nil || b.rdb == nil {
		return "", errorcode.ErrInternal.WithMsg("call signaling is not configured")
	}
	if !validCallID(callID) {
		return "", errorcode.ErrCallInvalid
	}
	state, err := b.rdb.HGet(ctx, b.callKey(callID), "state").Result()
	if errorsIsNil(err) || (err == nil && state == "") {
		return "", errorcode.ErrCallNotFound
	}
	if err != nil {
		return "", err
	}
	kind, err := b.rdb.HGet(ctx, b.callKey(callID), "kind").Result()
	if err != nil && !errorsIsNil(err) {
		return "", err
	}
	if errorsIsNil(err) || kind == "" || kind != KindGroup {
		return KindSingle, nil
	}
	if kind == KindGroup {
		return KindGroup, nil
	}
	return KindSingle, nil
}

// InviteGroup opens a group room and rings the invitees who are not already in a call.
// media is audio or video and stays fixed for the room. The caller is in the room immediately.
// A busy invitee is returned in Busy and is not rung.
func (b *Board) InviteGroup(ctx context.Context, caller, groupID, media string, invitees []string) (GroupStep, error) {
	step := GroupStep{CallerID: caller, GroupID: groupID, Media: media, Actor: caller}
	if b == nil || b.rdb == nil {
		return step, errorcode.ErrInternal.WithMsg("call signaling is not configured")
	}
	invitees = cleanInvitees(caller, invitees)
	if len(invitees) > MaxGroupVoice-1 {
		return step, errorcode.ErrCallFull
	}
	if !validUser(caller) || !validUser(groupID) || (media != MediaAudio && media != MediaVideo) || len(invitees) == 0 {
		return step, errorcode.ErrCallInvalid
	}
	callID, err := newCallID()
	if err != nil {
		return step, errorcode.ErrInternal.WithMsg("create call id")
	}
	step.CallID = callID
	deadline := time.Now().Add(b.ring).Unix()
	args := []any{
		caller, groupID, callID, media, deadline,
		int64(b.acceptedTTL / time.Second), int64(b.ringTTL / time.Second), b.prefix,
	}
	for _, id := range invitees {
		args = append(args, id)
	}
	fields, err := b.rdb.Eval(ctx, inviteGroupScript, []string{
		b.groupKey(groupID), b.callKey(callID), b.memberKey(callID), b.groupRingKey(),
	}, args...).StringSlice()
	if err != nil {
		return step, err
	}
	return parseGroup(step, fields)
}

// AcceptGroup moves a ringing member into the room.
func (b *Board) AcceptGroup(ctx context.Context, userID, callID string) (GroupStep, error) {
	return b.mutate(ctx, acceptGroupScript, userID, callID)
}

// RejectGroup stops ringing one invitee. The room ends when only the host is left and nobody is still ringing.
func (b *Board) RejectGroup(ctx context.Context, userID, callID string) (GroupStep, error) {
	return b.mutate(ctx, rejectGroupScript, userID, callID)
}

// LeaveGroup removes one person who is already in the room.
// The host leaving while nobody else has joined also stops the remaining rings.
func (b *Board) LeaveGroup(ctx context.Context, userID, callID string) (GroupStep, error) {
	return b.mutate(ctx, leaveGroupScript, userID, callID)
}

// EnterGroup joins a room that is already open. A person who is still ringing joins instead of being invited again.
func (b *Board) EnterGroup(ctx context.Context, userID, callID string) (GroupStep, error) {
	return b.mutate(ctx, enterGroupScript, userID, callID)
}

// RestoreRing puts a member back to ringing after a token could not be signed.
func (b *Board) RestoreRing(ctx context.Context, userID, callID string) error {
	_, err := b.mutate(ctx, restoreRingScript, userID, callID)
	return err
}

// UndoJoin removes a member who joined but could not be given a token.
func (b *Board) UndoJoin(ctx context.Context, userID, callID string) error {
	_, err := b.mutate(ctx, undoJoinScript, userID, callID)
	return err
}

// Group returns the voice room open in groupID. An empty CallID means there is none.
func (b *Board) Group(ctx context.Context, groupID string) (GroupStep, error) {
	step := GroupStep{GroupID: groupID}
	if b == nil || b.rdb == nil {
		return step, errorcode.ErrInternal.WithMsg("call signaling is not configured")
	}
	if !validUser(groupID) {
		return step, errorcode.ErrCallInvalid
	}
	callID, err := b.rdb.Get(ctx, b.groupKey(groupID)).Result()
	if errorsIsNil(err) || callID == "" {
		return step, nil
	}
	if err != nil {
		return step, err
	}
	if !validCallID(callID) {
		_ = b.rdb.Del(ctx, b.groupKey(groupID)).Err()
		return step, nil
	}
	vals, err := b.rdb.HGetAll(ctx, b.callKey(callID)).Result()
	if err != nil {
		return step, err
	}
	if len(vals) == 0 || vals["kind"] != KindGroup {
		_ = b.rdb.Del(ctx, b.groupKey(groupID)).Err()
		return GroupStep{GroupID: groupID}, nil
	}
	members, err := b.rdb.HGetAll(ctx, b.memberKey(callID)).Result()
	if err != nil {
		return step, err
	}
	step.CallID = callID
	step.CallerID = vals["caller"]
	step.Media = vals["media"]
	for id, status := range members {
		switch status {
		case memberJoined:
			step.Joined = append(step.Joined, id)
		case memberRinging:
			step.Ringing = append(step.Ringing, id)
		}
	}
	return step, nil
}

// MemberStatus is the stored state of one user in a group room. An empty status means they have no row.
func (b *Board) MemberStatus(ctx context.Context, callID, userID string) (string, error) {
	if b == nil || b.rdb == nil {
		return "", errorcode.ErrInternal.WithMsg("call signaling is not configured")
	}
	if !validCallID(callID) || !validUser(userID) {
		return "", errorcode.ErrCallInvalid
	}
	status, err := b.rdb.HGet(ctx, b.memberKey(callID), userID).Result()
	if errorsIsNil(err) {
		return "", nil
	}
	return status, err
}

// ExpireGroup stops invitees whose ring deadline has passed.
func (b *Board) ExpireGroup(ctx context.Context, now time.Time) ([]GroupStep, error) {
	if b == nil || b.rdb == nil {
		return nil, nil
	}
	ids, err := b.rdb.ZRangeByScore(ctx, b.groupRingKey(), &redis.ZRangeBy{
		Min: "-inf", Max: jsonNumber(now.Unix()), Count: 20,
	}).Result()
	if err != nil {
		return nil, err
	}
	out := make([]GroupStep, 0, len(ids))
	for _, member := range ids {
		callID, userID, ok := strings.Cut(member, ":")
		if !ok {
			_ = b.rdb.ZRem(ctx, b.groupRingKey(), member).Err()
			continue
		}
		step, err := b.mutate(ctx, timeoutGroupScript, userID, callID)
		if err != nil {
			if errors.Is(err, errorcode.ErrCallNotFound) || errors.Is(err, errorcode.ErrCallState) || errors.Is(err, errorcode.ErrCallNotMember) {
				continue
			}
			return out, err
		}
		out = append(out, step)
	}
	return out, nil
}

func (b *Board) mutate(ctx context.Context, script, userID, callID string) (GroupStep, error) {
	step := GroupStep{CallID: callID, Actor: userID}
	if b == nil || b.rdb == nil {
		return step, errorcode.ErrInternal.WithMsg("call signaling is not configured")
	}
	if !validCallID(callID) || !validUser(userID) {
		return step, errorcode.ErrCallInvalid
	}
	fields, err := b.rdb.Eval(ctx, script, []string{
		b.callKey(callID), b.memberKey(callID), b.groupRingKey(),
	}, userID, callID, int64(b.acceptedTTL/time.Second), int64(b.ringTTL/time.Second), b.prefix, jsonNumber(time.Now().Add(b.ring).Unix())).StringSlice()
	if err != nil {
		return step, err
	}
	return parseGroup(step, fields)
}

func (b *Board) groupKey(groupID string) string { return b.prefix + "group:" + groupID }
func (b *Board) memberKey(callID string) string { return b.prefix + "member:" + callID }
func (b *Board) groupRingKey() string           { return b.prefix + "gring" }

func cleanInvitees(caller string, ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == caller || !validUser(id) {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func parseGroup(step GroupStep, fields []string) (GroupStep, error) {
	if len(fields) == 0 {
		return step, errorcode.ErrInternal.WithMsg("update call")
	}
	switch fields[0] {
	case "ok", "ended":
	case "missing":
		return step, errorcode.ErrCallNotFound
	case "forbidden":
		return step, errorcode.ErrCallNotMember
	case "state":
		return step, errorcode.ErrCallState
	case "full":
		return step, errorcode.ErrCallFull
	case "self":
		return step, errorcode.ErrCallBusy
	case "group":
		return step, errorcode.ErrCallGroupBusy
	default:
		return step, errorcode.ErrInternal.WithMsg("update call")
	}
	if len(fields) < 4 {
		return step, errorcode.ErrInternal.WithMsg("update call")
	}
	step.Ended = fields[0] == "ended"
	step.GroupID = fields[1]
	step.CallerID = fields[2]
	step.Media = fields[3]
	var bucket *[]string
	for _, field := range fields[4:] {
		switch field {
		case ":joined":
			bucket = &step.Joined
		case ":ringing":
			bucket = &step.Ringing
		case ":busy":
			bucket = &step.Busy
		default:
			if bucket == nil {
				return step, errorcode.ErrInternal.WithMsg("update call")
			}
			*bucket = append(*bucket, field)
		}
	}
	return step, nil
}

func errorsIsNil(err error) bool {
	return err == redis.Nil
}
