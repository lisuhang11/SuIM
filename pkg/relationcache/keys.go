package relationcache

import "time"

const (
	// Topic is the Redis channel that carries deleted relation cache keys.
	Topic = "suim:relation:cache:delete"

	friendIDsPrefix = "suim:relation:friend-ids:"
	blackIDsPrefix  = "suim:relation:black-ids:"
	versionPrefix   = "suim:relation:friend-ver:"

	redisTTL = 12 * time.Hour
)

// FriendIDsKey is the shared cache key for one user's friend ID list.
func FriendIDsKey(owner string) string { return friendIDsPrefix + owner }

// BlackIDsKey is the shared cache key for one user's block list.
func BlackIDsKey(owner string) string { return blackIDsPrefix + owner }

// VersionKey is the shared cache key for one user's friend version head.
func VersionKey(owner string) string { return versionPrefix + owner }

// Config sizes the process-local cache. Zero values use the OpenIM friend defaults.
type Config struct {
	Topic         string
	SlotNum       int
	SlotSize      int
	SuccessExpire time.Duration
	FailedExpire  time.Duration
}

// DefaultConfig matches OpenIM's friend local cache: 100 slots of 2000 entries,
// successful reads kept for 5 minutes, failures kept for 5 seconds.
func DefaultConfig() Config {
	return Config{
		Topic:         Topic,
		SlotNum:       100,
		SlotSize:      2000,
		SuccessExpire: 300 * time.Second,
		FailedExpire:  5 * time.Second,
	}
}

func (c Config) apply() Config {
	base := DefaultConfig()
	if c.Topic != "" {
		base.Topic = c.Topic
	}
	if c.SlotNum > 0 {
		base.SlotNum = c.SlotNum
	}
	if c.SlotSize > 0 {
		base.SlotSize = c.SlotSize
	}
	if c.SuccessExpire > 0 {
		base.SuccessExpire = c.SuccessExpire
	}
	if c.FailedExpire > 0 {
		base.FailedExpire = c.FailedExpire
	}
	return base
}
