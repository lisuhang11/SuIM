package ws

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

var errLogout = errors.New("logout")

// Client is one logged-in websocket connection.
type Client struct {
	id         string
	userID     string
	platformID int
	token      string
	compress   bool
	background bool
	sdkVersion string
	encoder    Encoder
	conn       socket
	server     *Server
	closed     atomic.Bool
	subs       map[string]struct{}
}

func (c *Client) readLoop() {
	defer c.close()
	for {
		typ, data, err := c.conn.Read()
		if err != nil {
			return
		}
		// 1. 文本帧只处理心跳。业务帧仍是二进制。
		if typ == websocket.TextMessage {
			if err := c.pongText(data); err != nil {
				return
			}
			continue
		}
		if typ != websocket.BinaryMessage {
			continue
		}
		if c.compress {
			data, err = decompress(data)
			if err != nil {
				return
			}
		}
		var req Req
		if err := c.encoder.Decode(data, &req); err != nil {
			return
		}
		if err := c.server.dispatch(c, &req); err != nil {
			return
		}
	}
}

func (c *Client) pongText(data []byte) error {
	var msg struct {
		Type string          `json:"type"`
		Body json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}
	switch msg.Type {
	case textPong:
		return nil
	case textPing:
		msg.Type = textPong
		body, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		return c.conn.Write(websocket.TextMessage, body)
	default:
		return fmt.Errorf("unsupported text message %s", msg.Type)
	}
}

func (c *Client) reply(req *Req, code int, msg string, data []byte) error {
	resp := Resp{ErrCode: code, ErrMsg: msg, Data: data}
	if req != nil {
		resp.ReqIdentifier = req.ReqIdentifier
		resp.MsgIncr = req.MsgIncr
		resp.OperationID = req.OperationID
	}
	return c.write(resp)
}

func (c *Client) write(resp Resp) error {
	if c.closed.Load() {
		return errors.New("connection closed")
	}
	body, err := c.encoder.Encode(resp)
	if err != nil {
		return err
	}
	if c.compress {
		body, err = compress(body)
		if err != nil {
			return err
		}
	}
	return c.conn.Write(websocket.BinaryMessage, body)
}

// Kick tells the SDK this login was replaced, then closes the socket.
func (c *Client) Kick() {
	_ = c.write(Resp{ReqIdentifier: reqKickOnlineMsg})
	c.close()
}

func (c *Client) close() {
	if !c.closed.CompareAndSwap(false, true) {
		return
	}
	c.server.hub.Remove(c)
	_ = c.conn.Close()
	c.server.notifyOnline(c.userID)
}

func newConnID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
