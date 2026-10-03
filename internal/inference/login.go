package inference

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	goai "github.com/rcarmo/go-ai"
	"github.com/rcarmo/go-ai/oauth"
)

// Pi's /login and /logout provider options (interactive-mode.js
// getLoginProviderOptions and getLogoutProviderOptions). The providers are
// Pi's (login_providers_gen.go); OAuth sign-in is offered where go-ai has
// the provider's login flow.

type piLoginProvider struct {
	ID, Name                    string
	OAuth, Subscription, APIKey bool
}

// LoginOption is one sign-in method of a provider.
type LoginOption struct {
	ID, Name     string
	AuthType     string // "oauth" or "api_key"
	Subscription bool
	// StatusType is how the provider is configured ("oauth", "api_key"),
	// "" when it is not; StatusSource says where from.
	StatusType, StatusSource string
}

// LoginOptions are Pi's login options for authType ("" for both), sorted
// by name.
func LoginOptions(authType string) []LoginOption {
	Init()
	entries, _ := loadAuthEntries()
	var out []LoginOption
	for _, p := range piLoginProviders {
		statusType, source := "", ""
		if entry, ok := entries[p.ID]; ok {
			statusType, source = entryAuthType(entry), "stored credential"
		} else if goai.GetEnvAPIKey(goai.Provider(p.ID)) != "" {
			statusType, source = "api_key", "environment"
		}
		option := LoginOption{ID: p.ID, Name: p.Name, Subscription: p.Subscription, StatusType: statusType, StatusSource: source}
		if (authType == "" || authType == "oauth") && p.OAuth && oauth.GetProvider(p.ID) != nil {
			option.AuthType = "oauth"
			out = append(out, option)
		}
		if (authType == "" || authType == "api_key") && p.APIKey {
			option.AuthType = "api_key"
			out = append(out, option)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return LocaleCompare(out[i].Name, out[j].Name) < 0 })
	return out
}

// LogoutOptions are Pi's: the stored credentials, sorted by name.
func LogoutOptions() ([]LoginOption, error) {
	Init()
	entries, err := loadAuthEntries()
	if err != nil {
		return nil, err
	}
	var out []LoginOption
	for id, entry := range entries {
		kind := entryAuthType(entry)
		option := LoginOption{ID: id, Name: id, AuthType: kind, StatusType: kind, StatusSource: "stored credential"}
		for _, p := range piLoginProviders {
			if p.ID == id {
				option.Name, option.Subscription = p.Name, p.Subscription
			}
		}
		out = append(out, option)
	}
	sort.SliceStable(out, func(i, j int) bool { return LocaleCompare(out[i].Name, out[j].Name) < 0 })
	return out, nil
}

func entryAuthType(entry authEntry) string {
	if entry.Type == "api_key" || entry.Type == "api-key" || entry.Type == "" && (entry.Key != "" || entry.APIKey != "") {
		return "api_key"
	}
	return "oauth"
}

// OAuthLogin runs go-ai's sign-in flow for provider.
func OAuthLogin(provider string, callbacks oauth.LoginCallbacks) (*oauth.Credentials, error) {
	Init()
	p := oauth.GetProvider(provider)
	if p == nil {
		return nil, fmt.Errorf("no OAuth sign-in for %q", provider)
	}
	return p.Login(callbacks)
}

// SaveOAuthLogin stores credentials as Pi does: {"type": "oauth", refresh,
// access, expires, and the provider's extra fields}.
func SaveOAuthLogin(provider string, creds *oauth.Credentials) error {
	if creds == nil {
		return fmt.Errorf("no credentials")
	}
	entry := map[string]any{}
	for k, v := range creds.Extra {
		entry[k] = v
	}
	entry["type"], entry["refresh"], entry["access"], entry["expires"] = "oauth", creds.Refresh, creds.Access, creds.Expires
	return saveAuthEntry(provider, entry)
}

// SaveAPIKeyLogin stores a key as Pi does: {"type": "api_key", "key": key}.
func SaveAPIKeyLogin(provider, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("API key cannot be empty")
	}
	return saveAuthEntry(provider, map[string]any{"type": "api_key", "key": key})
}

func saveAuthEntry(provider string, entry map[string]any) error {
	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = updateCredentials("", func(entries map[string]json.RawMessage) error {
		entries[provider] = raw
		return nil
	})
	return err
}

// LocaleCompare approximates JavaScript's localeCompare for names: case is
// ignored first, then lower case sorts before upper.
func LocaleCompare(a, b string) int {
	if c := strings.Compare(strings.ToLower(a), strings.ToLower(b)); c != 0 {
		return c
	}
	return -strings.Compare(a, b)
}
