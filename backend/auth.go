package main

import (
	"crypto/subtle"
	"net/http"
)

// BasicAuth wraps a handler with HTTP Basic authentication. It gates every
// route — including the terminal WebSocket upgrade, which arrives as an HTTP
// request through the same handler chain. Constant-time comparison on both
// fields.
func BasicAuth(user, pass string, next http.Handler) http.Handler {
	userBytes := []byte(user)
	passBytes := []byte(pass)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		// Bitwise & so both comparisons always run (no timing shortcut).
		if !ok || subtle.ConstantTimeCompare([]byte(u), userBytes)&subtle.ConstantTimeCompare([]byte(p), passBytes) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="workshop"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
