package ws

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"encoding/json"
	"io"
)

const (
	sdkGo = "go"
	sdkJS = "js"

	reqGetNewestSeq      = 1001
	reqPullMsgBySeqList  = 1002
	reqSendMsg           = 1003
	reqSendSignalMsg     = 1004
	reqPullMsg           = 1005
	reqGetConvMaxReadSeq = 1006
	reqPullConvLastMsg   = 1007
	reqPushMsg           = 2001
	reqKickOnlineMsg     = 2002
	reqLogoutMsg         = 2003
	reqSetBackground     = 2004
	reqSubUserOnline     = 2005

	errArgs     = 1001
	errToken    = 1502
	errInternal = 500

	onlineStatus = 1
)

// Req matches the SDK general websocket request. Field names are part of the gob encoding.
type Req struct {
	ReqIdentifier int    `json:"reqIdentifier"`
	Token         string `json:"token"`
	SendID        string `json:"sendID"`
	OperationID   string `json:"operationID"`
	MsgIncr       string `json:"msgIncr"`
	Data          []byte `json:"data"`
}

// Resp matches the SDK general websocket response.
type Resp struct {
	ReqIdentifier int    `json:"reqIdentifier"`
	MsgIncr       string `json:"msgIncr"`
	OperationID   string `json:"operationID"`
	ErrCode       int    `json:"errCode"`
	ErrMsg        string `json:"errMsg"`
	Data          []byte `json:"data"`
}

type Encoder interface {
	Encode(v any) ([]byte, error)
	Decode(b []byte, v any) error
}

type gobEncoder struct{}

func (gobEncoder) Encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (gobEncoder) Decode(b []byte, v any) error {
	return gob.NewDecoder(bytes.NewReader(b)).Decode(v)
}

type jsonEncoder struct{}

func (jsonEncoder) Encode(v any) ([]byte, error) { return json.Marshal(v) }

func (jsonEncoder) Decode(b []byte, v any) error { return json.Unmarshal(b, v) }

func newEncoder(sdkType string) Encoder {
	if sdkType == sdkJS {
		return jsonEncoder{}
	}
	return gobEncoder{}
}

func compress(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(b); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decompress(b []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
