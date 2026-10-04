package turn

import (
	"context"
	"fmt"

	"github.com/rcarmo/gi/internal/logutil"
	goai "github.com/rcarmo/go-ai"
)

const planContextPrompt = `## Plan Sidebar
The current session has an editable Plan sidebar. Use the plan tool to read its current Markdown and update it as work progresses. Prefer action=patch for checklist changes; action=update replaces the checklist, action=edit applies exact text edits, and action=write replaces the Markdown. At most one checklist item may be in_progress. The current checklist is supplied separately from this system prompt.`

func (e *Engine) registerSessionPlanContext() {
	_, err := e.RegisterHook(HookBeforeAgentStart, "plan-sidebar", func(ctx context.Context, request HookRequest) (HookResponse, error) {
		saved, err := e.store.SessionPlan(ctx, request.SessionID)
		if err != nil {
			return HookResponse{}, err
		}
		if saved.UpdatedAt == nil || saved.Markdown == "" {
			return HookResponse{}, nil
		}
		return HookResponse{SystemPrompt: request.SystemPrompt + "\n\n" + planContextPrompt}, nil
	})
	logutil.WarnIfErr("register Plan instructions hook", err)
	_, err = e.RegisterHook(HookBeforeProviderRequest, "plan-sidebar", func(ctx context.Context, request HookRequest) (HookResponse, error) {
		if request.Payload["stage"] != "context" {
			return HookResponse{}, nil
		}
		saved, err := e.store.SessionPlan(ctx, request.SessionID)
		if err != nil {
			return HookResponse{}, err
		}
		if saved.UpdatedAt == nil || saved.Markdown == "" {
			return HookResponse{}, nil
		}
		// Decorate the request copy only. Durable compaction must compare the
		// unchanged conversation against its persisted history snapshot.
		message := goai.UserMessage(fmt.Sprintf("Current Plan Sidebar checklist for %s:\n\n%s", saved.ChatJID, saved.Markdown))
		return HookResponse{Messages: append([]goai.Message{message}, request.Messages...)}, nil
	})
	logutil.WarnIfErr("register Plan provider-context hook", err)
}
