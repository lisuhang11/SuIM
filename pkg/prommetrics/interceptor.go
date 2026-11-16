package prommetrics

import (
	"context"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor counts every unary RPC by service, method and gRPC code.
// Signature is compatible with grpc.ChainUnaryInterceptor alongside mw.UnaryRecover.
func UnaryServerInterceptor(serviceName string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		RPCCall(serviceName, info.FullMethod, int(status.Code(err)))
		return resp, err
	}
}

// HTTPMiddleware counts HTTP requests by path, method and status code.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(rec, r)
		HTTPCall(r.URL.Path, r.Method, rec.code)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (w *statusRecorder) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusRecorder) Write(b []byte) (int, error) {
	// 若未显式 WriteHeader，则假定 200。
	return w.ResponseWriter.Write(b)
}

func (w *statusRecorder) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// APIHTTPMiddleware counts API calls (including business response codes in the
// body for apigateway). Code is supplied by handlers via APICall where needed.
func APIHTTPMiddleware(next http.Handler) http.Handler {
	return HTTPMiddleware(next)
}
