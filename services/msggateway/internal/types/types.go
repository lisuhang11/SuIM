// Package types 定义跨包共享的基础类型与接口，避免循环依赖。
package types

// ---------- 关闭码 ----------

const (
	CloseCodeNormal       = 1000
	CloseCodeGoingAway    = 1001
	CloseCodeKicked       = 4000
	CloseCodeTokenExpired = 4001
)

// ---------- 消息类型 ----------

const (
	MsgTypePush      = "push"
	MsgTypeAck       = "ack"
	MsgTypeHeartbeat = "heartbeat"
	MsgTypeKick      = "kick"
	MsgTypeSync      = "sync"
)

// ---------- Conn 接口 ----------

// WritableConn 是可写连接的抽象，connmgr 通过此接口关闭或检查连接状态。
type WritableConn interface {
	ID() string
	Close(code int, reason string)
	IsClosed() bool
}

// OnlineChangeHook 上线/下线通知回调。
type OnlineChangeHook func(userID string, platformID int32, online bool)

// AuthFunc token 认证回调：返回 (userID, platformID, error)。
type AuthFunc func(token string) (userID string, platformID int32, err error)

// PlatformInfo 平台连接信息。
type PlatformInfo struct {
	PlatformID int32
	ConnIDs    []string
	Tokens     []string
}
