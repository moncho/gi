package inference

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"

	goai "github.com/rcarmo/go-ai"
)

// PiModelsStorePath is Pi's refreshed per-provider model catalogue
// (~/.pi/agent/models-store.json), read-only here like auth.json.
func PiModelsStorePath() string {
	return filepath.Join(filepath.Dir(AuthFilePath()), "models-store.json")
}

type piModelsStoreEntry struct {
	Models []json.RawMessage `json:"models"`
}

// registerPiModelsStore adds models from Pi's refreshed catalogue that the
// compiled-in go-ai catalogue does not know yet (e.g. a newly released Copilot
// model), so gi offers the same models Pi does. Existing go-ai definitions are
// never replaced. A missing or unreadable store is not an error.
func registerPiModelsStore(path string) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var store map[string]piModelsStoreEntry
	if err := json.Unmarshal(raw, &store); err != nil {
		log.Printf("models: ignoring unreadable %s: %v", path, err)
		return 0
	}
	added := 0
	for provider, entry := range store {
		for _, rawModel := range entry.Models {
			model, ok := decodePiStoreModel(provider, rawModel)
			if !ok || goai.GetModel(model.Provider, model.ID) != nil {
				continue
			}
			goai.RegisterModel(model)
			added++
		}
	}
	return added
}

// decodePiStoreModel maps Pi's model JSON onto goai.Model. The field names
// match except Pi's single "compat" object, which go-ai splits per API.
func decodePiStoreModel(provider string, raw json.RawMessage) (*goai.Model, bool) {
	var model goai.Model
	if err := json.Unmarshal(raw, &model); err != nil {
		return nil, false
	}
	if strings.TrimSpace(string(model.Provider)) == "" {
		model.Provider = goai.Provider(provider)
	}
	if strings.TrimSpace(model.ID) == "" || model.Api == "" || string(model.Provider) != provider {
		return nil, false
	}
	var extra struct {
		Compat json.RawMessage `json:"compat"`
	}
	if err := json.Unmarshal(raw, &extra); err == nil && len(extra.Compat) > 0 {
		switch model.Api {
		case goai.ApiOpenAIResponses:
			model.ResponsesCompat = &goai.OpenAIResponsesCompat{}
			_ = json.Unmarshal(extra.Compat, model.ResponsesCompat)
		case goai.ApiOpenAICompletions:
			model.CompletionsCompat = &goai.OpenAICompletionsCompat{}
			_ = json.Unmarshal(extra.Compat, model.CompletionsCompat)
		case goai.ApiAnthropicMessages:
			model.AnthropicCompat = &goai.AnthropicMessagesCompat{}
			_ = json.Unmarshal(extra.Compat, model.AnthropicCompat)
		}
	}
	return &model, true
}
