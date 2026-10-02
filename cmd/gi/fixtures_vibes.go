//go:build fixtures_vibes

package main

import (
	"net/url"
	"os"
	"strings"

	goai "github.com/rcarmo/go-ai"
)

// This model exists only in the disposable compliance build. Production Gi
// never registers a provider from an environment-controlled URL.
func init() {
	base := strings.TrimRight(os.Getenv("FIXTURE_MODEL_URL"), "/")
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.Path != "/v1" {
		panic("fixtures_vibes requires a loopback FIXTURE_MODEL_URL ending in /v1")
	}
	goai.RegisterModel(&goai.Model{
		ID: "fixture-1", Name: "Fixture model", Provider: "fixture-vibes",
		Api: goai.ApiOpenAICompletions, BaseURL: base,
		Input: []string{"text"}, ContextWindow: 32000, MaxTokens: 1024,
	})
}
