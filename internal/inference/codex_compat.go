package inference

import (
	"encoding/json"
	"fmt"

	goai "github.com/rcarmo/go-ai"
)

// codexPayloadHook preserves user hooks, then removes the output limit rejected
// by the ChatGPT Codex endpoint. Piclaw 3.2.4's Codex builder omits this field;
// our pinned go-ai builder adds it even when no token limit was requested.
// Other Responses APIs still support it and must keep their original payload.
func codexPayloadHook(next func(any, *goai.Model) (any, error)) func(any, *goai.Model) (any, error) {
	return func(payload any, model *goai.Model) (any, error) {
		if next != nil {
			replacement, err := next(payload, model)
			if err != nil {
				return nil, err
			}
			if replacement != nil {
				payload = replacement
			}
		}
		if model == nil || model.Api != goai.ApiOpenAICodexResponses {
			return payload, nil
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode Codex request: %w", err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
			return nil, fmt.Errorf("Codex request payload must be an object")
		}
		delete(fields, "max_output_tokens")
		return fields, nil
	}
}
