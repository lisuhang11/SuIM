package mw

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"google.golang.org/grpc"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

func TestRecoverHandler(t *testing.T) {
	h := RecoverHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/user/get", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", rr.Code)
	}
	var env struct {
		Code int32  `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Code != int32(errorcode.Internal) || env.Msg != errorcode.ErrInternal.Error() {
		t.Fatalf("body %+v", env)
	}

	rr = httptest.NewRecorder()
	RecoverHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rr.Code != http.StatusCreated || rr.Body.String() != "ok" {
		t.Fatalf("status %d body %q", rr.Code, rr.Body.String())
	}
}

func TestRecoverHandlerAfterWrite(t *testing.T) {
	h := RecoverHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("partial"))
		panic("boom")
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/user/get", nil))
	if rr.Code != http.StatusAccepted || rr.Body.String() != "partial" {
		t.Fatalf("status %d body %q", rr.Code, rr.Body.String())
	}
}

func TestRecoverHandlerBrokenPipe(t *testing.T) {
	h := RecoverHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(&net.OpError{Err: &os.SyscallError{Syscall: "write", Err: errors.New("broken pipe")}})
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/user/get", nil))
	if rr.Code == http.StatusInternalServerError || rr.Body.Len() != 0 {
		t.Fatalf("status %d body %q", rr.Code, rr.Body.String())
	}
}

func TestRecoverHandlerHijack(t *testing.T) {
	base := &hijackRecorder{ResponseRecorder: httptest.NewRecorder()}
	var got *hijackRecorder
	h := RecoverHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("hijack missing")
		}
		_, _, err := hj.Hijack()
		if !errors.Is(err, errHijack) {
			t.Fatal(err)
		}
		got = base
	}))
	h.ServeHTTP(base, httptest.NewRequest(http.MethodGet, "/", nil))
	if got == nil || !base.hijacked {
		t.Fatal("hijack was not delegated")
	}
}

func TestUnaryRecover(t *testing.T) {
	resp, err := UnaryRecover(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/suim.Msg/Send"}, func(context.Context, any) (any, error) {
		panic("boom")
	})
	if resp != nil || !errors.Is(err, errorcode.ErrInternal) {
		t.Fatalf("resp %v err %v", resp, err)
	}

	resp, err = UnaryRecover(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/suim.Msg/Send"}, func(context.Context, any) (any, error) {
		return "ok", nil
	})
	if err != nil || resp != "ok" {
		t.Fatalf("resp %v err %v", resp, err)
	}
}

var errHijack = errors.New("hijacked")

type hijackRecorder struct {
	*httptest.ResponseRecorder
	hijacked bool
}

func (h *hijackRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true
	return nil, nil, errHijack
}
