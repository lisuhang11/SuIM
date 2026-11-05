// Package mw holds HTTP middleware shared by the gateways.
package mw

import (
	"encoding/json"
	"net/http"
	"strings"
)

// CorsHandler matches OpenIM mw.CorsHandler: any origin, method, and header.
// A websocket upgrade is left without a preset Content-Type so the handshake stays intact.
func CorsHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "*")
		h.Set("Access-Control-Allow-Headers", "*")
		h.Set("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers,Cache-Control,Content-Language,Content-Type,Expires,Last-Modified,Pragma,FooBar")
		h.Set("Access-Control-Max-Age", "172800")
		h.Set("Access-Control-Allow-Credentials", "false")
		if !websocketUpgrade(r) {
			h.Set("Content-Type", "application/json")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode("Options Request!")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func websocketUpgrade(r *http.Request) bool {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return false
	}
	for _, part := range strings.Split(r.Header.Get("Connection"), ",") {
		if strings.EqualFold(strings.TrimSpace(part), "upgrade") {
			return true
		}
	}
	return false
}
