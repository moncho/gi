package mcp

import "net/http"

type httpRequest = http.Request

func captureAuth(next http.Handler, got *string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get("Authorization"); v != "" {
			*got = v
		}
		next.ServeHTTP(w, r)
	})
}
