package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rcarmo/gi/internal/plan"
	goai "github.com/rcarmo/go-ai"
)

func PlanTool() RegisteredTool {
	return RegisteredTool{Name: "plan", Source: "builtin", Kind: "mixed", Weight: "lightweight", Activation: "default",
		Description:   "Read or update the current session's Plan sidebar. Prefer patch for checklist add/update/remove, update for full structured replacement, edit for exact atomic text replacements, or write for full Markdown. At most one item may be in_progress. Targets are session-scoped.",
		PromptSnippet: "plan: session checklist. Prefer action=patch for multi-item changes; action=read for current Markdown. At most one item may be in_progress.",
		Parameters:    json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"action":{"type":"string","enum":["read","write","edit","patch","update"]},"markdown":{"type":"string","maxLength":262144},"chat_jid":{"type":"string"},"explanation":{"type":"string"},"plan":{"type":"array","maxItems":1000,"items":{"type":"object","additionalProperties":false,"properties":{"step":{"type":"string"},"status":{"type":"string","enum":["pending","in_progress","completed"]}},"required":["step","status"]}},"edits":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"properties":{"operation":{"type":"string","enum":["replace","delete","insert_after","insert_before","append","prepend"]},"oldText":{"type":"string"},"newText":{"type":"string"},"anchorText":{"type":"string"},"text":{"type":"string"}}}},"patches":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"properties":{"operation":{"type":"string","enum":["add","update","remove"]},"index":{"type":"integer","minimum":1},"status":{"type":"string","enum":["pending","in_progress","completed"]},"position":{"type":"string","enum":["start","end"]},"match":{"type":"string"},"step":{"type":"string"},"before":{"type":"string"},"after":{"type":"string"}},"required":["operation"]}}},"required":["action"]}`), Executor: ExecutePlan}
}

func ExecutePlan(ctx context.Context, rt ToolRuntime, call goai.ToolCall) (string, error) {
	if rt.Store == nil || rt.SessionID == "" {
		return "", fmt.Errorf("plan: runtime session is required")
	}
	raw, err := json.Marshal(call.Arguments)
	if err != nil || len(raw) > 2*plan.MaxBytes {
		return "", fmt.Errorf("plan: invalid or oversized arguments")
	}
	var mutation plan.Mutation
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&mutation); err != nil {
		return "", fmt.Errorf("plan: invalid arguments: %w", err)
	}
	if mutation.ChatJID != "" && strings.TrimSpace(mutation.ChatJID) != "gi:"+rt.SessionID {
		return "", fmt.Errorf("plan: only the current session is allowed")
	}
	// Piclaw prepareArguments accepts get/set and inferred structured actions.
	switch mutation.Action {
	case "get":
		mutation.Action = "read"
	case "set":
		mutation.Action = "write"
	case "":
		if mutation.Plan != nil {
			mutation.Action = "update"
		} else if mutation.Patches != nil {
			mutation.Action = "patch"
		}
	}
	if mutation.Action == "read" {
		saved, err := rt.Store.SessionPlan(ctx, rt.SessionID)
		if err != nil {
			return "", err
		}
		if rt.SetDetails != nil {
			details := map[string]any{}
			raw, _ := json.Marshal(saved)
			_ = json.Unmarshal(raw, &details)
			rt.SetDetails(details)
		}
		markdown := saved.Markdown
		if markdown == "" {
			markdown = "(empty)"
		}
		return "Plan for " + saved.ChatJID + ":\n\n" + markdown, nil
	}
	if mutation.Action == "reset" {
		return "", fmt.Errorf("plan: reset is an API-only operation")
	}
	saved, err := rt.Store.MutateSessionPlan(ctx, rt.SessionID, mutation)
	if err != nil {
		return "", err
	}
	if rt.SetDetails != nil {
		details := map[string]any{}
		raw, _ := json.Marshal(saved)
		_ = json.Unmarshal(raw, &details)
		rt.SetDetails(details)
	}
	if rt.PublishPlan != nil {
		rt.PublishPlan(saved, "tool", mutation.Action)
	}
	switch mutation.Action {
	case "update":
		return "Plan updated.", nil
	case "edit":
		return "Edited plan for " + saved.ChatJID + ".", nil
	case "patch":
		return "Patched plan for " + saved.ChatJID + ".", nil
	default:
		return "Updated plan for " + saved.ChatJID + ".", nil
	}
}
