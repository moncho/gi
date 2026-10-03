package inference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	goai "github.com/rcarmo/go-ai"

	"github.com/rcarmo/gi/internal/lockdir"
)

// Model catalogue refresh, a port of Pi's withRemoteCatalog
// (pi-coding-agent core/remote-catalog-provider.js) and FileModelsStore:
// each provider's catalogue is fetched from <base>/api/models/providers/<id>
// at most every four hours, revalidated with its ETag, and kept in
// models-store.json, whose models gi registers (registerPiModelsStore).
// Pi always uses https://pi.dev; gi refreshes only when modelCatalogUrl is
// set in the user settings.

const (
	catalogRefreshInterval = 4 * time.Hour
	catalogAttemptTimeout  = 4 * time.Second
	catalogMaxRetries      = 2 // Pi's fetchWithRetry default
)

// catalogModelTypes is Pi's REMOTE_CATALOG_MODEL_TYPES.
var catalogModelTypes = []string{"chat", "image", "classifier"}

// catalogRetryStatus is Pi's RETRYABLE_STATUS_CODES.
var catalogRetryStatus = []int{408, 425, 429, 500, 502, 503, 504}

var modelsStoreMu sync.Mutex

// catalogEntry is a models-store.json entry (Pi's StoredModelCatalog).
type catalogEntry struct {
	Models       []json.RawMessage `json:"models"`
	CheckedAt    *int64            `json:"checkedAt,omitempty"`
	LastModified *int64            `json:"lastModified,omitempty"`
	ETag         string            `json:"etag,omitempty"`
}

// RefreshModelCatalog refreshes the stored catalogue of every provider with
// credentials that is due (all of them with force) and registers models the
// store adds.
func RefreshModelCatalog(ctx context.Context, baseURL string, force bool) error {
	Init() // the built-in providers
	path := PiModelsStorePath()
	stored := readModelsStore(path)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error
	for _, provider := range goai.ListProviders() {
		id := string(provider)
		// Pi does not overlay radius, and fetches only for providers whose
		// credentials resolve.
		if id == "radius" || !providerConfigured(id) {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := refreshProviderCatalog(ctx, path, baseURL, id, stored[id], force); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	registerPiModelsStore(path)
	return errors.Join(errs...)
}

func readModelsStore(path string) map[string]*catalogEntry {
	out := map[string]*catalogEntry{}
	raw, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var entries map[string]json.RawMessage
	if json.Unmarshal(raw, &entries) != nil {
		return out
	}
	for id, rawEntry := range entries {
		var entry catalogEntry
		if json.Unmarshal(rawEntry, &entry) == nil {
			out[id] = &entry
		}
	}
	return out
}

func refreshProviderCatalog(ctx context.Context, path, baseURL, provider string, stored *catalogEntry, force bool) error {
	now := func() int64 { return time.Now().UnixMilli() }
	if !force && stored != nil && stored.CheckedAt != nil && stored.LastModified != nil &&
		now()-*stored.CheckedAt < catalogRefreshInterval.Milliseconds() {
		return nil
	}
	// Only revalidate when a cached body backs the validator, so a 304 can
	// never leave the overlay empty.
	validator := ""
	if stored != nil && len(stored.Models) > 0 {
		validator = stored.ETag
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	u = u.ResolveReference(&url.URL{Path: "/api/models/providers/" + url.PathEscape(provider)})
	u.RawQuery = url.Values{"types": {strings.Join(catalogModelTypes, ",")}}.Encode()
	resp, err := fetchCatalog(ctx, u.String(), validator)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	checkedAt := now()
	keep := func() catalogEntry {
		if stored != nil {
			return *stored
		}
		return catalogEntry{Models: []json.RawMessage{}}
	}
	switch {
	case resp.StatusCode == http.StatusNotModified && stored != nil:
		entry := keep()
		entry.CheckedAt = &checkedAt
		return writeModelsStoreEntry(path, provider, entry)
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNotImplemented:
		entry := keep()
		zero := int64(0)
		entry.CheckedAt, entry.LastModified, entry.ETag = &checkedAt, &zero, ""
		return writeModelsStoreEntry(path, provider, entry)
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		// Transient: the cached body and its validator stay valid.
		entry := keep()
		entry.CheckedAt = &checkedAt
		_ = writeModelsStoreEntry(path, provider, entry)
		return fmt.Errorf("model catalog request failed for %s: %d", provider, resp.StatusCode)
	}
	var body json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return fmt.Errorf("model catalog for %s: %w", provider, err)
	}
	models, err := parseCatalog(provider, body)
	if err != nil {
		return err
	}
	lastModified := int64(0)
	if t, err := http.ParseTime(resp.Header.Get("Last-Modified")); err == nil {
		lastModified = t.UnixMilli()
	}
	return writeModelsStoreEntry(path, provider, catalogEntry{Models: models, CheckedAt: &checkedAt, LastModified: &lastModified, ETag: resp.Header.Get("ETag")})
}

// fetchCatalog is Pi's fetchWithRetry: retryable statuses and network
// errors are retried at once, each attempt limited to four seconds.
func fetchCatalog(ctx context.Context, target, validator string) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, catalogAttemptTimeout)
		req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, target, nil)
		if err != nil {
			cancel()
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "gi")
		if validator != "" {
			req.Header.Set("If-None-Match", validator)
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil && (!slices.Contains(catalogRetryStatus, resp.StatusCode) || attempt >= catalogMaxRetries) {
			resp.Body = cancelOnClose{resp.Body, cancel}
			return resp, nil
		}
		if err == nil {
			resp.Body.Close()
		}
		cancel()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err != nil && attempt >= catalogMaxRetries {
			return nil, err
		}
	}
}

type cancelOnClose struct {
	body   io.ReadCloser
	cancel context.CancelFunc
}

func (c cancelOnClose) Read(p []byte) (int, error) { return c.body.Read(p) }
func (c cancelOnClose) Close() error {
	defer c.cancel()
	return c.body.Close()
}

// parseCatalog is Pi's parseCatalog: an array, {models: [...]}, or an
// object of models (in key order); entries with an id and a supported type,
// tagged with the provider. Model fields are kept as served.
func parseCatalog(provider string, body json.RawMessage) ([]json.RawMessage, error) {
	var entries []json.RawMessage
	if json.Unmarshal(body, &entries) != nil {
		var object map[string]json.RawMessage
		if json.Unmarshal(body, &object) != nil || object == nil {
			return nil, fmt.Errorf("Invalid model catalog for provider %q", provider)
		}
		if json.Unmarshal(object["models"], &entries) != nil || entries == nil {
			entries = nil
			for _, key := range slices.Sorted(maps.Keys(object)) {
				entries = append(entries, object[key])
			}
		}
	}
	tag, _ := json.Marshal(provider)
	out := []json.RawMessage{}
	for _, entry := range entries {
		var model map[string]json.RawMessage
		if json.Unmarshal(entry, &model) != nil || model == nil || model["id"] == nil {
			continue
		}
		if t, has := model["type"]; has {
			var s string
			if json.Unmarshal(t, &s) != nil || !slices.Contains(catalogModelTypes, s) {
				continue
			}
		}
		model["provider"] = tag
		if raw, err := json.Marshal(model); err == nil {
			out = append(out, raw)
		}
	}
	return out, nil
}

// writeModelsStoreEntry is FileModelsStore.write: under Pi's lock, replace
// one provider's entry and keep the others.
func writeModelsStoreEntry(path, provider string, entry catalogEntry) error {
	modelsStoreMu.Lock() // concurrent refreshes queue here rather than poll Pi's lock
	defer modelsStoreMu.Unlock()
	return lockdir.With(path+".lock", piAuthLockStale, piAuthLockStale, func() error {
		current := map[string]json.RawMessage{}
		if raw, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(raw)) != "" {
			if err := json.Unmarshal(raw, &current); err != nil {
				return fmt.Errorf("models store %s: %w", path, err)
			}
		}
		rawEntry, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		current[provider] = rawEntry
		body, err := json.MarshalIndent(current, "", "  ")
		if err != nil {
			return err
		}
		temp, err := os.CreateTemp(filepath.Dir(path), ".models-store-*.tmp")
		if err != nil {
			return err
		}
		defer os.Remove(temp.Name())
		if _, err = temp.Write(body); err == nil {
			err = temp.Chmod(0o600)
		}
		if closeErr := temp.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
		return os.Rename(temp.Name(), path)
	})
}
