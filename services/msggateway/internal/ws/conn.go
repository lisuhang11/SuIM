// Package ws 封装 WebSocket 连接与消息读写。
package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"msggateway/internal/types"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ---------- 消息结构 ----------

// WSMessage 通用 WebSocket 消息格式。
type WSMessage struct {
	Type    string          `json:"type"`
	SeqID   string          `json:"seq_id,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
	ErrCode int32           `json:"err_code,omitempty"`
	ErrMsg  string          `json:"err_msg,omitempty"`
}

// HeartbeatMsg 心跳消息体。
type HeartbeatMsg struct {
	Timestamp int64 `json:"timestamp"`
}

// KickMsg 踢下线通知体。
type KickMsg struct {
	Reason string `json:"reason"`
}

// ---------- 连接封装 ----------

// Conn 封装 gorilla/websocket.Conn，实现 types.WritableConn 接口。
type Conn struct {
	cid        string
	UserID     string
	PlatformID int32
	Token      string

	conn *websocket.Conn
	mu   sync.Mutex

	readTimeout  time.Duration
	writeTimeout time.Duration

	closeOnce sync.Once
	closed    chan struct{}
}

// 确保 Conn 实现 types.WritableConn。
var _ types.WritableConn = (*Conn)(nil)

// ID 返回连接唯一标识（实现 types.WritableConn）。
func (c *Conn) ID() string { return c.cid }

// NewConn 创建连接封装。
func NewConn(userID string, platformID int32, token string, wsConn *websocket.Conn,
	readTimeout, writeTimeout time.Duration) *Conn {

	return &Conn{
		cid:          uuid.NewString(),
		UserID:       userID,
		PlatformID:   platformID,
		Token:        token,
		conn:         wsConn,
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
		closed:       make(chan struct{}),
	}
}

// ReadMessage 阻塞读取一条 JSON 消息。
func (c *Conn) ReadMessage() (*WSMessage, error) {
	_ = c.conn.SetReadDeadline(time.Now().Add(c.readTimeout))
	_, raw, err := c.conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	var msg WSMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// WriteMessage 线程安全地写入一条 JSON 消息。
func (c *Conn) WriteMessage(msg *WSMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_ = c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	return c.conn.WriteJSON(msg)
}

// Close 安全关闭连接（实现 types.WritableConn）。
func (c *Conn) Close(code int, reason string) {
	c.closeOnce.Do(func() {
		close(c.closed)
		msg := websocket.FormatCloseMessage(code, reason)
		_ = c.conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(5*time.Second))
		_ = c.conn.Close()
	})
}

// Closed 返回 chan，连接关闭时收到信号。
func (c *Conn) Closed() <-chan struct{} {
	return c.closed
}

// IsClosed 检查是否已关闭（实现 types.WritableConn）。
func (c *Conn) IsClosed() bool {
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}

// ---------- WebSocket 升级器 ----------

// Upgrader 配置 HTTP → WebSocket 升级。
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  4 * 1024,
	WriteBufferSize: 4 * 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	EnableCompression: true,
}

// Upgrade 将 HTTP 连接升级为 WebSocket。
func Upgrade(w http.ResponseWriter, r *http.Request, reqHeader http.Header) (*websocket.Conn, error) {
	return Upgrader.Upgrade(w, r, reqHeader)
}

// ---------- 辅助 ----------

// NewHeartbeatMsg 构造心跳消息。
func NewHeartbeatMsg() *WSMessage {
	return &WSMessage{
		Type: types.MsgTypeHeartbeat,
		Data: mustMarshal(HeartbeatMsg{Timestamp: time.Now().UnixMilli()}),
	}
}

// NewPushMsg 构造推送消息。
func NewPushMsg(data json.RawMessage, seqID string) *WSMessage {
	return &WSMessage{
		Type:  types.MsgTypePush,
		SeqID: seqID,
		Data:  data,
	}
}

// NewKickMsg 构造踢下线通知。
func NewKickMsg(reason string) *WSMessage {
	return &WSMessage{
		Type: types.MsgTypeKick,
		Data: mustMarshal(KickMsg{Reason: reason}),
	}
}

// NewSyncMsg 构造同步提醒。
func NewSyncMsg(data json.RawMessage) *WSMessage {
	return &WSMessage{
		Type: types.MsgTypeSync,
		Data: data,
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// LogConnInfo 打印连接信息日志。
func LogConnInfo(userID string, platformID int32, connID string, action string) {
	slog.Info("ws_conn",
		"action", action,
		"user_id", userID,
		"platform_id", platformID,
		"conn_id", connID,
	)
}
