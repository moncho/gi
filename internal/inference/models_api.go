package inference

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	goai "github.com/rcarmo/go-ai"
	"github.com/rcarmo/go-ai/images"
	_ "github.com/rcarmo/go-ai/images/openrouter" // registers the OpenRouter image API
)

// Typed model catalog and non-chat model calls for codemode's models API
// (Pi's ModelRegistry getModelsOfType/getAvailableOfType/getModelOfType,
// classify and generateImages).

var typedModelsOnce sync.Once

// initTypedModels registers the image and classifier catalogs next to the
// chat models.
func initTypedModels() {
	Init()
	typedModelsOnce.Do(func() {
		goai.RegisterBuiltinImageModels()
		goai.RegisterBuiltinClassifierModels()
		images.RegisterBuiltinImageModels()
	})
}

// modelInfo is a catalog entry for scripts: the model's JSON with its type
// and without headers (models.json headers can carry credentials).
func modelInfo(model any, typ goai.ModelType) map[string]any {
	raw, err := json.Marshal(model)
	if err != nil {
		return nil
	}
	var info map[string]any
	if json.Unmarshal(raw, &info) != nil {
		return nil
	}
	delete(info, "headers")
	delete(info, "apiKey")
	info["type"] = string(typ)
	return info
}

// ModelsOfType lists every known model of a type ("chat", "image",
// "classifier"), optionally for one provider.
func ModelsOfType(typ, provider string) []map[string]any {
	initTypedModels()
	out := []map[string]any{}
	for _, m := range goai.ListTypedModels(goai.ModelType(typ), provider) {
		if info := modelInfo(m, goai.ModelType(typ)); info != nil {
			out = append(out, info)
		}
	}
	return out
}

// ModelOfType is one catalog entry, or nil.
func ModelOfType(typ, provider, id string) map[string]any {
	initTypedModels()
	m := goai.GetTypedModel(goai.TypedModelRef{Type: goai.ModelType(typ), Provider: provider, ID: id})
	if m == nil || isNilModel(m) {
		return nil
	}
	return modelInfo(m, goai.ModelType(typ))
}

func isNilModel(m any) bool {
	switch v := m.(type) {
	case *goai.Model:
		return v == nil
	case *goai.ImageModel:
		return v == nil
	case *goai.ClassifierModel:
		return v == nil
	}
	return false
}

// providerCredential is the provider's key: auth.json, else its
// environment variable. ok reports whether the provider is usable.
func providerCredential(provider string) (string, bool) {
	if provider == "opencode-zen" {
		return "", true
	}
	if key, _, err := loadAuth(provider); err == nil && key != "" {
		return key, true
	}
	if key := goai.GetEnvAPIKey(goai.Provider(provider)); key != "" {
		return key, true
	}
	return "", false
}

// providerConfigured reports credentials without network calls (Copilot
// token exchange happens only when the model is used).
func providerConfigured(provider string) bool {
	if provider == "opencode-zen" {
		return true
	}
	if entries, err := loadAuthEntries(); err == nil {
		if entry, ok := entries[provider]; ok && entry.Type != "" {
			return true
		}
	}
	return goai.GetEnvAPIKey(goai.Provider(provider)) != ""
}

// AvailableOfType lists models of a type whose provider has working
// credentials (chat models as the model picker shows them).
func AvailableOfType(typ, provider string) []map[string]any {
	initTypedModels()
	out := []map[string]any{}
	if goai.ModelType(typ) == goai.ModelTypeChat || typ == "" {
		_, options := ListRuntimeOptions("", "", nil)
		for _, o := range options {
			if !o.Authenticated || (provider != "" && o.Provider != provider) {
				continue
			}
			if m := goai.GetModel(goai.Provider(o.Provider), o.ID); m != nil {
				out = append(out, modelInfo(m, goai.ModelTypeChat))
			}
		}
		return out
	}
	for _, info := range ModelsOfType(typ, provider) {
		if p, _ := info["provider"].(string); providerConfigured(p) {
			out = append(out, info)
		}
	}
	return out
}

// Classify runs a classifier model with the provider's credentials. Errors
// are results (stopReason "error"), never Go errors, like Pi's.
func Classify(ctx context.Context, provider, id string, classifierContext json.RawMessage) json.RawMessage {
	initTypedModels()
	model := goai.GetClassifierModel(goai.ClassifierProvider(provider), id)
	fail := func(msg string) json.RawMessage {
		raw, _ := json.Marshal(map[string]any{"provider": provider, "model": id, "answers": map[string]any{}, "stopReason": "error", "errorMessage": msg})
		return raw
	}
	if model == nil {
		return fail("unknown classifier model")
	}
	var cc goai.ClassifierContext
	if err := json.Unmarshal(classifierContext, &cc); err != nil {
		return fail(err.Error())
	}
	key, ok := providerCredential(provider)
	if !ok {
		return fail("no credentials for provider " + provider)
	}
	res, err := goai.Classify(model, cc, &goai.ClassifierOptions{APIKey: key, Context: ctx})
	if err != nil {
		return fail(err.Error())
	}
	if ctx.Err() != nil && res.StopReason == goai.StopReasonError {
		res.StopReason = goai.StopReasonAborted
	}
	raw, _ := json.Marshal(res)
	return raw
}

// GenerateImages runs an image model with the provider's credentials.
// Errors are results, like Pi's.
func GenerateImages(ctx context.Context, provider, id string, imagesContext json.RawMessage) json.RawMessage {
	initTypedModels()
	model := images.GetImageModel(images.ImagesProvider(provider), id)
	fail := func(msg string) json.RawMessage {
		raw, _ := json.Marshal(map[string]any{"provider": provider, "model": id, "output": []any{}, "stopReason": "error", "errorMessage": msg})
		return raw
	}
	if model == nil {
		return fail("unknown image model")
	}
	var ic images.ImagesContext
	if err := json.Unmarshal(imagesContext, &ic); err != nil {
		return fail(err.Error())
	}
	key, ok := providerCredential(provider)
	if !ok {
		return fail("no credentials for provider " + provider)
	}
	res, err := images.GenerateImages(model, ic, &images.ImagesOptions{APIKey: key, Context: ctx})
	if err != nil {
		return fail(err.Error())
	}
	raw, _ := json.Marshal(res)
	return raw
}

// ModelTypes are the types scripts may name.
var ModelTypes = []string{"chat", "image", "classifier"}

// IsModelType reports a known model type.
func IsModelType(typ string) bool {
	for _, t := range ModelTypes {
		if t == strings.TrimSpace(typ) {
			return true
		}
	}
	return false
}
