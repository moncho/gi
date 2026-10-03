package inference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/rcarmo/go-ai/oauth"
)

// Copilot credentials as Pi keeps them (pi-ai models.js
// resolveRefreshCredential, oauth/github-copilot.js): the Copilot token in
// "access" is used until "expires"; then, holding auth.json's lock, the
// entry is read again (another gi or Pi may have refreshed it) and, if still
// expired, refreshed. A refresh also fetches the account's model list and
// stores its picker-enabled models as availableModelIds, which filter the
// Copilot model listing.

// refreshCopilotCredentials is go-ai's port of Pi's refreshGitHubCopilotToken
// (tests replace it).
var refreshCopilotCredentials = oauth.RefreshGitHubCopilotTokenContext

const copilotRefreshTimeout = 30 * time.Second

func copilotAuth(entry authEntry) (token, baseURL string, err error) {
	if entry.Access != "" && time.Now().UnixMilli() < entry.Expires {
		return entry.Access, oauth.GetGitHubCopilotBaseURL(entry.Access, entry.EnterpriseURL), nil
	}
	var refreshed authEntry
	_, err = updateCredentials("", func(entries map[string]json.RawMessage) error {
		fields := map[string]json.RawMessage{}
		if raw := entries["github-copilot"]; raw != nil {
			_ = json.Unmarshal(raw, &fields)
		}
		var current authEntry
		if raw := entries["github-copilot"]; raw != nil {
			_ = json.Unmarshal(raw, &current)
		}
		if current.Access != "" && time.Now().UnixMilli() < current.Expires {
			refreshed = current
			return errCredentialUnchanged
		}
		refreshToken := current.Refresh
		if refreshToken == "" {
			refreshToken = entry.Refresh
		}
		if refreshToken == "" {
			return errors.New("no refresh token for github-copilot")
		}
		ctx, cancel := context.WithTimeout(context.Background(), copilotRefreshTimeout)
		defer cancel()
		creds, err := refreshCopilotCredentials(ctx, refreshToken, current.EnterpriseURL)
		if err != nil {
			return err
		}
		ids, _ := creds.Extra["availableModelIds"].([]string)
		if ids == nil {
			ids = []string{}
		}
		refreshed = authEntry{Type: "oauth", Refresh: creds.Refresh, Access: creds.Access, Expires: creds.Expires,
			EnterpriseURL: current.EnterpriseURL, AvailableModelIDs: ids}
		set := func(key string, value any) { fields[key], _ = json.Marshal(value) }
		set("type", "oauth")
		set("refresh", creds.Refresh)
		set("access", creds.Access)
		set("expires", creds.Expires)
		set("availableModelIds", ids)
		entries["github-copilot"], _ = json.Marshal(fields)
		return nil
	})
	if err != nil {
		return "", "", fmt.Errorf("refresh copilot token: %w", err)
	}
	return refreshed.Access, oauth.GetGitHubCopilotBaseURL(refreshed.Access, refreshed.EnterpriseURL), nil
}

// RefreshExpiredCopilotCredentials refreshes stored Copilot credentials
// whose token expired, as Pi's startup model refresh does, so the account's
// model list is current before the first request.
func RefreshExpiredCopilotCredentials() error {
	entries, err := loadAuthEntries()
	if err != nil {
		return err
	}
	entry, ok := entries["github-copilot"]
	if !ok || entry.Type != "oauth" || (entry.Access != "" && time.Now().UnixMilli() < entry.Expires) {
		return nil
	}
	_, _, err = copilotAuth(entry)
	return err
}
