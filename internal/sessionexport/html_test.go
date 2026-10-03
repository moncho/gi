package sessionexport

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// piExportGolden is scripts/golden-export-html.mjs's output.
type piExportGolden struct {
	Themes map[string]struct {
		Colors [][2]string       `json:"colors"`
		Export map[string]string `json:"export"`
		Root   string            `json:"root"`
	} `json:"themes"`
	Session struct {
		Theme       string `json:"theme"`
		Page        string `json:"page"`
		SessionData any    `json:"sessionData"`
	} `json:"session"`
	Replacements [][2]string `json:"replacements"`
}

func readExportGolden(t *testing.T) piExportGolden {
	t.Helper()
	data, err := os.ReadFile("testdata/pi-export-html.json")
	if err != nil {
		t.Fatal(err)
	}
	var g piExportGolden
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

func goldenTheme(g piExportGolden, name string) Theme {
	th := g.Themes[name]
	return Theme{Colors: th.Colors, PageBg: th.Export["pageBg"], CardBg: th.Export["cardBg"], InfoBg: th.Export["infoBg"]}
}

// Pi's page for the same session and theme: the template, the theme's CSS
// variables, the inlined libraries and the session data.
func TestRenderHTMLMatchesPi(t *testing.T) {
	g := readExportGolden(t)
	raw, err := os.ReadFile("testdata/pi-session.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	var lines []Entry
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, e)
	}
	out, err := RenderHTML(lines[0], lines[1:], goldenTheme(g, g.Session.Theme))
	if err != nil {
		t.Fatal(err)
	}
	page := string(out)
	for _, part := range [][2]string{{"{{JS}}", "template.js"}, {"{{MARKED_JS}}", "vendor/marked.min.js"}, {"{{HIGHLIGHT_JS}}", "vendor/highlight.min.js"}} {
		body, _ := templateFS.ReadFile("template/" + part[1])
		inserted := jsReplace(part[0], part[0], string(body))
		if strings.Count(page, inserted) != 1 {
			t.Fatalf("%s not inserted once", part[1])
		}
		page = strings.Replace(page, inserted, part[0], 1)
	}
	m := regexp.MustCompile(`id="session-data"[^>]*>([A-Za-z0-9+/=]+)<`).FindStringSubmatch(page)
	if m == nil {
		t.Fatal("no session data")
	}
	decoded, _ := base64.StdEncoding.DecodeString(m[1])
	var data any
	if err := json.Unmarshal(decoded, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data, g.Session.SessionData) {
		t.Fatalf("session data\n got %s\nwant %v", decoded, g.Session.SessionData)
	}
	page = strings.Replace(page, m[1], "{{SESSION_DATA}}", 1)
	if page != g.Session.Page {
		a, b := page, g.Session.Page
		i := 0
		for i < len(a) && i < len(b) && a[i] == b[i] {
			i++
		}
		t.Fatalf("page differs at %d:\n got %.200q\nwant %.200q", i, a[i:], b[i:])
	}
}

// The CSS variables Pi writes for each theme, with export colours explicit
// or derived from userMessageBg.
func TestRenderHTMLThemeVariablesMatchPi(t *testing.T) {
	g := readExportGolden(t)
	for name, th := range g.Themes {
		out, err := RenderHTML(Entry{"type": "session"}, nil, goldenTheme(g, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(out), th.Root) {
			start := strings.Index(string(out), ":root {")
			t.Fatalf("%s: root\n got %s\nwant %s", name, string(out)[start:start+len(th.Root)], th.Root)
		}
	}
}

// Pi assembles the page with JavaScript's String.replace.
func TestJSReplaceMatchesJavaScript(t *testing.T) {
	for _, c := range readExportGolden(t).Replacements {
		if got := jsReplace("L{{X}}R", "{{X}}", c[0]); got != c[1] {
			t.Errorf("%q: got %q, want %q", c[0], got, c[1])
		}
	}
}
