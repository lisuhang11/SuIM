package ws

import "testing"

type panicSocket struct{}

func (panicSocket) Read() (int, []byte, error) { panic("boom") }
func (panicSocket) Write(int, []byte) error    { return nil }
func (panicSocket) Close() error               { return nil }

func TestReadLoopPanicCloses(t *testing.T) {
	c := &Client{
		userID: "a",
		conn:   panicSocket{},
		server: &Server{hub: NewHub()},
	}
	c.readLoop()
	if !c.closed.Load() {
		t.Fatal("connection stayed open")
	}
}
