package mw

import (
	"compress/gzip"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
)

// OpenIM compressionLevel values.
const (
	gzipOff     = -1
	gzipDefault = 0
	gzipBest    = 1
	gzipFast    = 2
)

// GzipHandler compresses responses the way OpenIM's API gzip middleware does.
// level -1 leaves the handler unchanged. 0, 1, and 2 are default, smallest, and fastest.
// Any other level also leaves the handler unchanged.
// A response is compressed only when the request accepts gzip, and not for websocket
// upgrades, server-sent events, or the image extensions OpenIM skips.
func GzipHandler(level int, next http.Handler) http.Handler {
	gzLevel, ok := gzipWriterLevel(level)
	if !ok {
		return next
	}
	h := &gzipMiddleware{next: next}
	h.pool.New = func() any {
		w, err := gzip.NewWriterLevel(io.Discard, gzLevel)
		if err != nil {
			panic(err)
		}
		return w
	}
	return h
}

func gzipWriterLevel(level int) (int, bool) {
	switch level {
	case gzipDefault:
		return gzip.DefaultCompression, true
	case gzipBest:
		return gzip.BestCompression, true
	case gzipFast:
		return gzip.BestSpeed, true
	default:
		return 0, false
	}
}

type gzipMiddleware struct {
	next http.Handler
	pool sync.Pool
}

func (h *gzipMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !acceptsGzip(r) {
		h.next.ServeHTTP(w, r)
		return
	}
	gz := h.pool.Get().(*gzip.Writer)
	gz.Reset(w)
	defer func() {
		_ = gz.Close()
		gz.Reset(io.Discard)
		h.pool.Put(gz)
	}()
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Vary", "Accept-Encoding")
	h.next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, gz: gz}, r)
}

func acceptsGzip(r *http.Request) bool {
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") ||
		strings.Contains(r.Header.Get("Connection"), "Upgrade") ||
		strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		return false
	}
	switch filepath.Ext(r.URL.Path) {
	case ".png", ".gif", ".jpeg", ".jpg":
		return false
	default:
		return true
	}
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (w *gzipResponseWriter) Write(p []byte) (int, error) {
	w.Header().Del("Content-Length")
	return w.gz.Write(p)
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) Flush() {
	_ = w.gz.Flush()
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
