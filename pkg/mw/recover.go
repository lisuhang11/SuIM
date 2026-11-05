package mw

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"google.golang.org/grpc"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

// RecoverHandler catches a panic from next, logs it, and writes HTTP 500.
// A broken connection is logged and left unwritten. The process keeps running.
func RecoverHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &wroteWriter{ResponseWriter: w}
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			slog.Error("http panic", "err", rec, "method", r.Method, "path", r.URL.Path, "stack", string(debug.Stack()))
			if brokenPipe(rec) || rw.wrote {
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(struct {
				Code int32  `json:"code"`
				Msg  string `json:"msg"`
			}{Code: int32(errorcode.Internal), Msg: errorcode.ErrInternal.Error()})
		}()
		next.ServeHTTP(rw, r)
	})
}

// UnaryRecover catches a panic from a gRPC handler and returns the internal error.
// The process keeps running.
func UnaryRecover(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		rec := recover()
		if rec == nil {
			return
		}
		method := ""
		if info != nil {
			method = info.FullMethod
		}
		slog.Error("grpc panic", "err", rec, "method", method, "stack", string(debug.Stack()))
		err = errorcode.ErrInternal
	}()
	return handler(ctx, req)
}

func brokenPipe(rec any) bool {
	ne, ok := rec.(*net.OpError)
	if !ok {
		return false
	}
	var se *os.SyscallError
	if !errors.As(ne, &se) {
		return false
	}
	msg := strings.ToLower(se.Error())
	return strings.Contains(msg, "broken pipe") || strings.Contains(msg, "connection reset by peer")
}

// wroteWriter records whether the handler already started the response.
type wroteWriter struct {
	http.ResponseWriter
	wrote bool
}

func (w *wroteWriter) WriteHeader(code int) {
	if w.wrote {
		return
	}
	w.wrote = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *wroteWriter) Write(p []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

func (w *wroteWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *wroteWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijack not supported")
	}
	return h.Hijack()
}

func (w *wroteWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
