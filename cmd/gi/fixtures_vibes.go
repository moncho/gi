//go:build fixtures_vibes

package main

import (
	"net/url"
	"os"
	"strings"

	goai "github.com/rcarmo/go-ai"
)

// These models exist only in the disposable compliance build. Production Gi
// never registers a provider from an environment-controlled URL.
func init() {
	base := strings.TrimRight(os.Getenv("FIXTURE_MODEL_URL"), "/")
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.Path != "/v1" {
		panic("fixtures_vibes requires a loopback FIXTURE_MODEL_URL ending in /v1")
	}
	for _, id := range []string{"fixture-1", "fixture-2"} {
		goai.RegisterModel(&goai.Model{
			ID: id, Name: "Fixture model " + id, Provider: "fixture-vibes",
			Api: goai.ApiOpenAICompletions, BaseURL: base,
			Input: []string{"text"}, ContextWindow: 32000, MaxTokens: 1024,
		})
	}
}
