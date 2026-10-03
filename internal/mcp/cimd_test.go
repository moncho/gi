package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// oauth settings validate, and Client ID Metadata Documents are chosen, as
// Pi's own validateMcpServerConfig, callbackId and clientMetadataDocument
// do (scripts/golden-mcp-cimd.mjs).
func TestOAuthSettingsAndCIMDMatchPi(t *testing.T) {
	raw, err := os.ReadFile("testdata/pi-mcp-cimd.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Validation []struct {
			OAuth json.RawMessage
			Error *string
		}
		CallbackIDs []struct{ URL, ID string } `json:"callbackIds"`
		Documents   []struct {
			Metadata *authServerMeta
			Document *struct{ URL, RedirectURL string }
			Error    *string
		}
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	for _, v := range g.Validation {
		entry := fmt.Sprintf(`{"url": "https://mcp.example.com/mcp", "oauth": %s}`, v.OAuth)
		_, err := parseServer("s", json.RawMessage(entry), "mcp.json")
		got := "<nil>"
		if err != nil {
			got = `server "s": ` + err.Error()
		}
		want := "<nil>"
		if v.Error != nil {
			want = *v.Error
		}
		if got != want {
			t.Errorf("oauth %s: %s, want %s", v.OAuth, got, want)
		}
	}
	for _, c := range g.CallbackIDs {
		if got := callbackID(c.URL); got != c.ID {
			t.Errorf("callbackID(%s) = %s, want %s", c.URL, got, c.ID)
		}
	}
	for _, d := range g.Documents {
		doc, err := pickClientMetadataDocument("https://mcp.example.com/mcp", "http://127.0.0.1:8123/callback", d.Metadata)
		switch {
		case d.Error != nil && (err == nil || err.Error() != *d.Error):
			t.Errorf("metadata %+v: %v, want %q", d.Metadata, err, *d.Error)
		case d.Error == nil && (err != nil || doc.url != d.Document.URL || doc.redirectURL != d.Document.RedirectURL):
			t.Errorf("metadata %+v: %+v %v, want %+v", d.Metadata, doc, err, *d.Document)
		}
	}
}
