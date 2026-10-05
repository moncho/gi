package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

// Opaque-origin widgets can import public modules, including compressed ones.
// File existence is checked before CORS is granted; APIs never use this handler.
func TestStaticJavaScriptCORS(t *testing.T) {
	root := fstest.MapFS{
		"module.js":    {Data: []byte("export const value = 1;")},
		"module.js.br": {Data: []byte("compressed")},
		"module.js.gz": {Data: []byte("compressed")},
		"module.mjs":   {Data: []byte("export default 1;")},
		"styles.css":   {Data: []byte("body {}")},
	}
	handler := withPrecompressed(root, http.FileServer(http.FS(root)))
	for _, tc := range []struct{ method, path, encoding, cors string }{
		{http.MethodGet, "/module.js", "", "*"},
		{http.MethodGet, "/module.js?v=1", "br", "*"},
		{http.MethodGet, "/module.js", "gzip", "*"},
		{http.MethodHead, "/module.js", "br", "*"},
		{http.MethodGet, "/module.mjs", "", "*"},
		{http.MethodGet, "/styles.css", "", ""},
		{http.MethodGet, "/missing.js", "", ""},
		{http.MethodPost, "/module.js", "", ""},
		{http.MethodOptions, "/module.js", "", ""},
	} {
		t.Run(tc.method+tc.path+tc.encoding, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, nil)
			r.Header.Set("Origin", "null")
			r.Header.Set("Accept-Encoding", tc.encoding)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if got := w.Header().Get("Access-Control-Allow-Origin"); got != tc.cors {
				t.Fatalf("CORS=%q want=%q status=%d", got, tc.cors, w.Code)
			}
			if w.Header().Get("Access-Control-Allow-Credentials") != "" {
				t.Fatal("static asset grants credentialed cross-origin access")
			}
		})
	}
}

func TestStaticJavaScriptCORSDoesNotExposeAuthenticatedAPIs(t *testing.T) {
	srv := authSessionServer(t)
	for _, path := range []string{"/api/runtime/config", "/api/sessions", "/api/sessions/other/plan", "/api/sessions/other/widgets/widget"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("Origin", "null")
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized || w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatalf("%s: status=%d CORS=%q", path, w.Code, w.Header().Get("Access-Control-Allow-Origin"))
		}
	}
	for _, enc := range []string{"", "br", "gzip"} {
		r := httptest.NewRequest(http.MethodGet, "/js/vendor/katex.min.js", nil)
		r.Header.Set("Origin", "null")
		r.Header.Set("Accept-Encoding", enc)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, r)
		if w.Code != http.StatusOK || w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Fatalf("static encoding=%s: status=%d CORS=%q", enc, w.Code, w.Header().Get("Access-Control-Allow-Origin"))
		}
	}
}
