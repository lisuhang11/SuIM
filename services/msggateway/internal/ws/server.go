// Package ws 提供 WebSocket 服务端：连接升级、认证、读写泵。
package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"msggateway/internal/connmgr"
	"msggateway/internal/types"

	"github.com/gorilla/websocket"
)

// Server WebSocket 服务端。
type Server struct {
	cfg     ServerConfig
	connMgr *connmgr.Manager
	authFn  types.AuthFunc
	onlineChg types.OnlineChangeHook
}

// ServerConfig WebSocket 服务端配置。
type ServerConfig struct {
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PingInterval time.Duration
}

// NewServer 创建 WebSocket 服务端。
func NewServer(cfg ServerConfig, mgr *connmgr.Manager) *Server {
	return &Server{
		cfg:     cfg,
		connMgr: mgr,
	}
}

// SetAuthFunc 设置认证回调。
func (s *Server) SetAuthFunc(fn types.AuthFunc) { s.authFn = fn }

// SetOnlineChangeHook 设置上线/下线通知回调。
func (s *Server) SetOnlineChangeHook(fn types.OnlineChangeHook) { s.onlineChg = fn }

// ServeHTTP 处理 WebSocket 升级请求。
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	var userID string
	var platformID int32
	if s.authFn != nil {
		var err error
		userID, platformID, err = s.authFn(token)
		if err != nil {
			slog.Warn("ws auth failed", "error", err)
			http.Error(w, "authentication failed", http.StatusUnauthorized)
			return
		}
	} else {
		userID = token
		platformID = 1
	}

	wsConn, err := Upgrade(w, r, nil)
	if err != nil {
		slog.Error("ws upgrade failed", "error", err, "user_id", userID)
		return
	}

	conn := NewConn(userID, platformID, token, wsConn, s.cfg.ReadTimeout, s.cfg.WriteTimeout)

	if !s.connMgr.Add(userID, platformID, token, conn) {
		conn.Close(types.CloseCodeNormal, "connection limit exceeded")
		slog.Warn("connection rejected: limit exceeded", "user_id", userID)
		return
	}
	LogConnInfo(userID, platformID, conn.ID(), "connected")

	if s.onlineChg != nil {
		s.onlineChg(userID, platformID, true)
	}

	go s.writePump(conn)
	go s.readPump(conn)
}

// readPump 从客户端读取消息。
func (s *Server) readPump(conn *Conn) {
	defer func() {
		s.connMgr.Remove(conn.UserID, conn.PlatformID, conn.ID())
		s.connMgr.RemoveToken(conn.UserID, conn.PlatformID, conn.Token)
		conn.Close(types.CloseCodeGoingAway, "")
		LogConnInfo(conn.UserID, conn.PlatformID, conn.ID(), "disconnected")

		if len(s.connMgr.GetUserConns(conn.UserID)) == 0 && s.onlineChg != nil {
			s.onlineChg(conn.UserID, conn.PlatformID, false)
		}
	}()

	conn.conn.SetPongHandler(func(string) error {
		_ = conn.conn.SetReadDeadline(time.Now().Add(s.cfg.ReadTimeout))
		return nil
	})

	for {
		msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				slog.Warn("ws read error", "error", err, "user_id", conn.UserID, "conn_id", conn.ID())
			}
			return
		}

		switch msg.Type {
		case types.MsgTypeHeartbeat:
			_ = conn.WriteMessage(NewHeartbeatMsg())
		case types.MsgTypeAck:
			slog.Debug("ws ack received",
				"user_id", conn.UserID,
				"conn_id", conn.ID(),
				"seq_id", msg.SeqID,
			)
		default:
			slog.Debug("ws unknown message type", "type", msg.Type, "user_id", conn.UserID)
		}
	}
}

// writePump 向客户端发送心跳 ping。
func (s *Server) writePump(conn *Conn) {
	ticker := time.NewTicker(s.cfg.PingInterval)
	defer func() {
		ticker.Stop()
		conn.Close(types.CloseCodeGoingAway, "")
	}()

	for {
		select {
		case <-conn.Closed():
			return
		case <-ticker.C:
			_ = conn.conn.WriteControl(websocket.PingMessage, nil,
				time.Now().Add(s.cfg.WriteTimeout))
		}
	}
}

// PushToUser 向指定用户的所有平台推送消息。
func (s *Server) PushToUser(userID string, msg json.RawMessage) int {
	conns := s.connMgr.GetUserConns(userID)
	seqID := generateSeqID()
	count := 0
	for _, conn := range conns {
		if conn.IsClosed() {
			continue
		}
		pushMsg := NewPushMsg(msg, seqID)
		if c, ok := conn.(*Conn); ok {
			if err := c.WriteMessage(pushMsg); err != nil {
				slog.Warn("push failed", "error", err, "user_id", userID)
				continue
			}
			count++
		}
	}
	return count
}

// PushToUserPlatform 向指定用户的指定平台推送消息。
func (s *Server) PushToUserPlatform(userID string, platformID int32, msg json.RawMessage) int {
	conns := s.connMgr.GetUserPlatformConns(userID, platformID)
	seqID := generateSeqID()
	count := 0
	for _, conn := range conns {
		if conn.IsClosed() {
			continue
		}
		pushMsg := NewPushMsg(msg, seqID)
		if c, ok := conn.(*Conn); ok {
			if err := c.WriteMessage(pushMsg); err != nil {
				slog.Warn("push to platform failed", "error", err,
					"user_id", userID, "platform_id", platformID)
				continue
			}
			count++
		}
	}
	return count
}

// KickUser 踢用户下线。
func (s *Server) KickUser(userID string, platformID int32) int {
	kicked := s.connMgr.KickUser(userID, platformID)
	kickMsg := NewKickMsg("kicked by server")
	conns := s.connMgr.GetUserConns(userID)
	for _, conn := range conns {
		if conn.IsClosed() {
			continue
		}
		if platformID == 0 {
			if c, ok := conn.(*Conn); ok {
				_ = c.WriteMessage(kickMsg)
			}
		}
	}
	return kicked
}

// ConnMgr 暴露连接管理器（供 handler 查询状态）。
func (s *Server) ConnMgr() *connmgr.Manager { return s.connMgr }

// ---------- 工具 ----------

var seqIDCounter uint64

func generateSeqID() string {
	seqIDCounter++
	return time.Now().Format("20060102150405") + "-" +
		itoa(int(seqIDCounter % 100000))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [10]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
