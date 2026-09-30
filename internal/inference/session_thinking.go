package inference

import (
	"context"
	"fmt"

	"github.com/rcarmo/gi/internal/store"
	goai "github.com/rcarmo/go-ai"
)

func ThinkingLevels(modelID string) []string {
	Init()
	provider, name := splitModelID(modelID)
	model := goai.GetModel(goai.Provider(provider), name)
	if model == nil || !model.Reasoning || provider == "opencode-zen" {
		return nil
	}
	levels := goai.GetSupportedThinkingLevels(model)
	out := make([]string, 0, len(levels))
	for _, level := range levels {
		out = append(out, string(level))
	}
	return out
}
func ValidateThinking(modelID, level string) error {
	levels := ThinkingLevels(modelID)
	if len(levels) == 0 {
		return fmt.Errorf("model does not advertise thinking support")
	}
	if level == "" {
		return nil
	}
	for _, allowed := range levels {
		if level == allowed {
			return nil
		}
	}
	return fmt.Errorf("thinking level is not supported by this model")
}

// Only selections made through the validated web API are applied to inference.
// Legacy/TUI pass-through strings retain their old behaviour until validated.
func CapturedSessionThinking(s *store.Session, modelID string) string {
	bound, _ := s.State["thinking_model"].(string)
	level, _ := s.State["thinking_level"].(string)
	if bound == "" || level == "" || bound != modelID || ValidateThinking(modelID, level) != nil {
		return ""
	}
	return level
}
func SelectThinking(ctx context.Context, s *store.Store, id, expected, model, level string, options []ModelOption, fallback SessionModelChoice) (*store.Session, error) {
	selected, err := ResolveSessionModel(options, model)
	if err != nil {
		return nil, err
	}
	if selected.Label != model {
		return nil, fmt.Errorf("canonical selected model is required")
	}
	session, err := s.GetSession(ctx, id)
	if err != nil {
		return nil, err
	}
	if SessionModel(session.State, fallback).Label() != model {
		return nil, store.ErrContextChanged
	}
	if err = ValidateThinking(model, level); err != nil {
		return nil, err
	}
	if err = s.SelectSessionThinking(ctx, id, expected, model, level); err != nil {
		return nil, err
	}
	return s.GetSession(ctx, id)
}

// ModelReasoning reports whether the model registry knows modelID and, if so,
// whether it supports reasoning. Pi omits the footer thinking level and uses
// the "off" editor border for non-reasoning models.
func ModelReasoning(modelID string) (known, reasoning bool) {
	Init()
	provider, name := splitModelID(modelID)
	model := goai.GetModel(goai.Provider(provider), name)
	if model == nil {
		return false, false
	}
	return true, model.Reasoning
}

// EffectiveThinking is Pi's session thinking level for modelID: the given
// level (or Pi's default "medium"), clamped to the levels the model supports.
// It returns "" when the model does not reason, or when gi cannot send a
// level on its transport; known reports whether the registry knows modelID.
func EffectiveThinking(modelID, level string) (effective string, known bool) {
	Init()
	provider, name := splitModelID(modelID)
	model := goai.GetModel(goai.Provider(provider), name)
	if model == nil {
		return "", false
	}
	if !model.Reasoning || provider == "opencode-zen" {
		return "", true
	}
	if level == "" {
		level = "medium"
	}
	return string(goai.ClampThinkingLevel(model, goai.ModelThinkingLevel(level))), true
}
