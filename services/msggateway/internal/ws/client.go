package ws

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
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
	encoder    Encoder
	conn       socket
	server     *Server
	closed     atomic.Bool
	subs       map[string]struct{}
}

func (c *Client) readLoop() {
	defer c.close()
	for {
		_, data, err := c.conn.Read()
		if err != nil {
			return
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
