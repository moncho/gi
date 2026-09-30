package web

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcarmo/gi/internal/config"
)

// Static assets are served from build-time .br/.gz variants when the client
// accepts them, with unchanged content types and Vary: Accept-Encoding; plain
// bytes otherwise. index.html (templated per request) is never pre-compressed.
func TestStaticAssetsServePrecompressedVariants(t *testing.T) {
	srv := New(nil, nil, config.RuntimeConfig{WorkspaceRoot: t.TempDir()})
	get := func(url, accept string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, url, nil)
		if accept != "" {
			r.Header.Set("Accept-Encoding", accept)
		}
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, r)
		return w
	}
	const asset = "/js/vendor/katex.min.js"
	plain := get(asset, "")
	if plain.Code != 200 || plain.Header().Get("Content-Encoding") != "" || plain.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatalf("plain: %d enc=%q vary=%q", plain.Code, plain.Header().Get("Content-Encoding"), plain.Header().Get("Vary"))
	}
	br := get(asset, "gzip, deflate, br")
	if br.Header().Get("Content-Encoding") != "br" || !strings.HasPrefix(br.Header().Get("Content-Type"), "text/javascript") || br.Body.Len() >= plain.Body.Len() {
		t.Fatalf("br: enc=%q type=%q size=%d plain=%d", br.Header().Get("Content-Encoding"), br.Header().Get("Content-Type"), br.Body.Len(), plain.Body.Len())
	}
	gz := get(asset+"?v=cachebuster", "gzip")
	if gz.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("gzip: enc=%q", gz.Header().Get("Content-Encoding"))
	}
	zr, err := gzip.NewReader(gz.Body)
	if err != nil {
		t.Fatal(err)
	}
	decoded, _ := io.ReadAll(zr)
	if !bytes.Equal(decoded, plain.Body.Bytes()) {
		t.Fatal("gzip variant does not decode to the plain asset")
	}
	if w := get(asset, "br;q=0, gzip;q=0"); w.Header().Get("Content-Encoding") != "" {
		t.Fatalf("q=0 ignored: %q", w.Header().Get("Content-Encoding"))
	}
	if w := get("/", "br, gzip"); w.Header().Get("Content-Encoding") != "" || !strings.Contains(w.Body.String(), "<html") {
		t.Fatalf("index must stay uncompressed and templated: enc=%q", w.Header().Get("Content-Encoding"))
	}
	if w := get("/fonts/vendor/firacode-nerd-font-mono-regular.woff2", "br, gzip"); w.Code != 200 || w.Header().Get("Content-Encoding") != "" {
		t.Fatalf("woff2 font: %d enc=%q", w.Code, w.Header().Get("Content-Encoding"))
	}
}
