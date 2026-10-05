package web

import (
	"bytes"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

// precompressedHandler serves build-time .br/.gz variants of embedded static
// assets with the matching Content-Encoding (and Vary: Accept-Encoding),
// falling back to the plain file. Content types, paths and cache busters are
// unchanged: the variant is chosen per request from Accept-Encoding.
type precompressedHandler struct {
	root fs.FS
	next http.Handler
}

// encodedVariants lists the encodings tried, best first.
var encodedVariants = []struct{ encoding, suffix string }{{"br", ".br"}, {"gzip", ".gz"}}

func withPrecompressed(root fs.FS, next http.Handler) http.Handler {
	return precompressedHandler{root: root, next: next}
}

func (h precompressedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		h.next.ServeHTTP(w, r)
		return
	}
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "" || strings.HasSuffix(name, ".br") || strings.HasSuffix(name, ".gz") {
		h.next.ServeHTTP(w, r)
		return
	}
	// Opaque-origin widget iframes may import public JavaScript modules. This
	// handler only serves embedded assets; authenticated API routes never pass
	// through it. Do not grant CORS to missing files or non-script assets.
	if ext := path.Ext(name); ext == ".js" || ext == ".mjs" {
		if info, err := fs.Stat(h.root, name); err == nil && !info.IsDir() {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
	}
	var chosen, available []string
	for _, v := range encodedVariants {
		if _, err := fs.Stat(h.root, name+v.suffix); err == nil {
			available = append(available, v.encoding)
			if chosen == nil && acceptsEncoding(r.Header.Get("Accept-Encoding"), v.encoding) {
				chosen = []string{v.encoding, v.suffix}
			}
		}
	}
	if len(available) > 0 {
		w.Header().Add("Vary", "Accept-Encoding")
	}
	if chosen == nil {
		h.next.ServeHTTP(w, r)
		return
	}
	file, err := h.root.Open(name + chosen[1])
	if err != nil {
		h.next.ServeHTTP(w, r)
		return
	}
	defer file.Close()
	// Embedded files implement ReadSeeker; serve directly instead of copying
	// the entire compressed asset into a fresh buffer for every request.
	content, ok := file.(io.ReadSeeker)
	if !ok {
		data, err := io.ReadAll(file)
		if err != nil {
			h.next.ServeHTTP(w, r)
			return
		}
		content = bytes.NewReader(data)
	}
	ctype := mime.TypeByExtension(path.Ext(name))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Encoding", chosen[0])
	http.ServeContent(w, r, name, time.Time{}, content)
}

// acceptsEncoding reports whether an Accept-Encoding header allows enc
// (explicitly or via "*"), honouring q=0 exclusions.
func acceptsEncoding(header, enc string) bool {
	star := false
	for _, part := range strings.Split(header, ",") {
		fields := strings.Split(strings.TrimSpace(part), ";")
		name := strings.ToLower(strings.TrimSpace(fields[0]))
		q := 1.0
		for _, param := range fields[1:] {
			param = strings.TrimSpace(param)
			if strings.HasPrefix(param, "q=") {
				if v, err := strconv.ParseFloat(strings.TrimPrefix(param, "q="), 64); err == nil {
					q = v
				}
			}
		}
		switch name {
		case enc:
			return q > 0
		case "*":
			star = q > 0
		}
	}
	return star
}
