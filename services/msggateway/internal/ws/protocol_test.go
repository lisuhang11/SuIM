package ws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"

	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

func TestTextPingRepliesPong(t *testing.T) {
	client := &Client{conn: &memSocket{}}
	if err := client.pongText([]byte(`{"type":"ping","body":"x"}`)); err != nil {
		t.Fatal(err)
	}
	writes := client.conn.(*memSocket).writes
	if len(writes) != 1 || string(writes[0]) != `{"type":"pong","body":"x"}` {
		t.Fatalf("pong %s", writes)
	}
	if err := client.pongText([]byte(`{"type":"pong"}`)); err != nil {
		t.Fatal(err)
	}
	if len(client.conn.(*memSocket).writes) != 1 {
		t.Fatal("pong should not be answered")
	}
}

func TestParseDialSendResponse(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?token=t&sendID=a&platformID=5&operationID=op&isMsgResp=true&sdkVersion=1.2", nil)
	info, code, msg := parseDial(req)
	if code != 0 || !info.SendResponse || info.SDKVersion != "1.2" {
		t.Fatalf("query %+v code %d %s", info, code, msg)
	}

	body, err := json.Marshal(dialInfo{
		Token: "t", UserID: "a", PlatformID: 5, OperationID: "op",
		SendResponse: true, SDKVersion: "js-sdk",
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := base64.RawURLEncoding.EncodeToString(body)
	req = httptest.NewRequest(http.MethodGet, "/?v="+raw, nil)
	info, code, msg = parseDial(req)
	if code != 0 || !info.SendResponse || info.SDKVersion != "js-sdk" {
		t.Fatalf("blob %+v code %d %s", info, code, msg)
	}
}

func TestDeliverSkipsBackgroundIOS(t *testing.T) {
	server := &Server{hub: NewHub()}
	ios := &Client{
		userID: "a", platformID: platformIOS, background: true,
		encoder: jsonEncoder{}, conn: &memSocket{}, server: server,
	}
	web := &Client{
		userID: "a", platformID: 5,
		encoder: jsonEncoder{}, conn: &memSocket{}, server: server,
	}
	server.hub.Add(ios)
	server.hub.Add(web)
	server.Deliver(context.Background(), &msgpb.MsgData{
		SendId: "a", RecvId: "b", SessionType: 1, ContentType: 101, Content: []byte(`{"content":"hi"}`),
	}, "si_a_b")
	if n := len(ios.conn.(*memSocket).writes); n != 0 {
		t.Fatalf("ios background writes %d", n)
	}
	if n := len(web.conn.(*memSocket).writes); n != 1 {
		t.Fatalf("web writes %d", n)
	}
}

func TestFailDialWritesTextWhenRequested(t *testing.T) {
	server := &Server{upgrader: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.failDial(w, r, true, http.StatusBadRequest, errArgs, "platformID is invalid")
	}))
	defer srv.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+srv.URL[4:], nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatal(err)
	}
	if resp["errCode"] != float64(errArgs) || resp["errMsg"] != "platformID is invalid" {
		t.Fatalf("resp %s", data)
	}
}
