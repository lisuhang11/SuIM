package ws

import (
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait = 10 * time.Second
	pongWait  = 30 * time.Second
	pingEvery = (pongWait * 9) / 10
	maxBytes  = 51200
	sendCap   = 256
)

type frame struct {
	typ  int
	data []byte
}

// socket is the connection a client reads and writes.
type socket interface {
	Read() (int, []byte, error)
	Write(typ int, data []byte) error
	Close() error
}

type wsSocket struct {
	conn *websocket.Conn
	send chan frame
	once sync.Once
	done chan struct{}
}

func newSocket(conn *websocket.Conn) *wsSocket {
	s := &wsSocket{
		conn: conn,
		send: make(chan frame, sendCap),
		done: make(chan struct{}),
	}
	conn.SetReadLimit(maxBytes)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	go s.loop()
	go s.ping()
	return s
}

func (s *wsSocket) Read() (int, []byte, error) {
	typ, data, err := s.conn.ReadMessage()
	if err != nil {
		return 0, nil, err
	}
	_ = s.conn.SetReadDeadline(time.Now().Add(pongWait))
	return typ, data, nil
}

func (s *wsSocket) Write(typ int, data []byte) error {
	select {
	case <-s.done:
		return fmt.Errorf("connection closed")
	case s.send <- frame{typ: typ, data: data}:
		return nil
	case <-time.After(writeWait):
		return fmt.Errorf("write buffer full")
	}
}

func (s *wsSocket) Close() error {
	s.once.Do(func() { close(s.done) })
	return s.conn.Close()
}

func (s *wsSocket) loop() {
	for {
		select {
		case <-s.done:
			return
		case frame := <-s.send:
			_ = s.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := s.conn.WriteMessage(frame.typ, frame.data); err != nil {
				_ = s.Close()
				return
			}
		}
	}
}

func (s *wsSocket) ping() {
	ticker := time.NewTicker(pingEvery)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			if err := s.Write(websocket.PingMessage, nil); err != nil {
				_ = s.Close()
				return
			}
		}
	}
}
