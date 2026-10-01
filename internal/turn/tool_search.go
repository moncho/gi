package turn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	gimcp "github.com/rcarmo/gi/internal/mcp"
	"github.com/rcarmo/gi/internal/tools"
	goai "github.com/rcarmo/go-ai"
)

// tool_search (#25 phase 3), ported from Pi: BM25 over deferred tools
// (MCP tools with codemode or deferred exposure) that the session has not
// loaded yet; matches are loaded for the session (persisted in its state, so
// they survive restarts, resume and forks) and declared from the next model
// call. The result marks them with AddedToolNames, which go-ai uses to load
// them at that point for providers with deferred tools.

const loadedToolsStateKey = "loaded_tools"

// sessionLoadedTools returns the tools the session loaded with tool_search.
func (e *Engine) sessionLoadedTools(ctx context.Context, sessionID string) []string {
	if sessionID == "" {
		return nil
	}
	sess, err := e.store.GetSession(ctx, sessionID)
	if err != nil {
		return nil
	}
	raw, _ := sess.State[loadedToolsStateKey].([]any)
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (e *Engine) sessionLoadedTool(ctx context.Context, sessionID, name string) bool {
	for _, n := range e.sessionLoadedTools(ctx, sessionID) {
		if n == name {
			return true
		}
	}
	return false
}

func (e *Engine) addSessionLoadedTools(ctx context.Context, sessionID string, names []string) error {
	current := e.sessionLoadedTools(ctx, sessionID)
	seen := map[string]bool{}
	for _, n := range current {
		seen[n] = true
	}
	for _, n := range names {
		if !seen[n] {
			seen[n] = true
			current = append(current, n)
		}
	}
	return e.store.TouchSessionState(ctx, sessionID, map[string]any{loadedToolsStateKey: current})
}

// declareLoadedTools adds registered definitions for names to the context's
// tools (skipping ones already declared or no longer registered).
func (e *Engine) declareLoadedTools(convCtx *goai.Context, names []string) {
	if convCtx == nil || len(names) == 0 {
		return
	}
	have := map[string]bool{}
	for _, t := range convCtx.Tools {
		have[t.Name] = true
	}
	for _, name := range names {
		if have[name] {
			continue
		}
		if t, ok := e.tools.GetRegistered(name); ok {
			convCtx.Tools = append(convCtx.Tools, t.Definition())
			have[name] = true
		}
	}
}

// toolSearchTool is Pi's tool_search.
func (e *Engine) toolSearchTool() tools.RegisteredTool {
	return tools.RegisteredTool{
		Name: tools.ToolSearchName, Description: tools.ToolSearchDescription, Parameters: json.RawMessage(tools.ToolSearchParameters),
		Source: "builtin", Kind: "read", Weight: "lightweight", ModelOnly: true,
		Executor: func(ctx context.Context, rt tools.ToolRuntime, call goai.ToolCall) (string, error) {
			query, _ := call.Arguments["query"].(string)
			if strings.TrimSpace(query) == "" {
				return "", errors.New("query must not be empty")
			}
			limit := tools.DefaultToolSearchLimit
			if v, ok := call.Arguments["limit"]; ok && v != nil {
				f, ok := v.(float64)
				if !ok || f != float64(int(f)) || f <= 0 {
					return "", errors.New("limit must be a positive integer")
				}
				limit = int(f)
			}
			loaded := map[string]bool{}
			for _, n := range e.sessionLoadedTools(ctx, rt.SessionID) {
				loaded[n] = true
			}
			var candidates []tools.RegisteredTool
			var docs []tools.SearchDocument
			for _, t := range e.tools.DeferredEntries() {
				if loaded[t.Name] {
					continue
				}
				var params any
				_ = json.Unmarshal(t.Parameters, &params)
				candidates = append(candidates, t)
				docs = append(docs, tools.NewSearchDocument(t.Name, t.Description, params, e.mcpNamespaceOf(t.Name)))
			}
			matches := tools.RankBM25(query, docs, limit)
			if len(matches) == 0 {
				return "No matching tools found.", nil
			}
			names := make([]string, len(matches))
			lines := make([]string, len(matches))
			for i, m := range matches {
				names[i] = m.Name
				desc := ""
				for _, c := range candidates {
					if c.Name == m.Name {
						desc = strings.TrimSpace(c.Description)
						break
					}
				}
				first := strings.SplitN(strings.ReplaceAll(desc, "\r\n", "\n"), "\n", 2)[0]
				lines[i] = fmt.Sprintf("- %s: %s", m.Name, first)
			}
			if rt.SessionID != "" {
				if err := e.addSessionLoadedTools(ctx, rt.SessionID, names); err != nil {
					log.Printf("tool_search: record loaded tools: %v", err)
				}
			}
			if rt.AddTools != nil {
				rt.AddTools(names)
			}
			plural := "s"
			if len(names) == 1 {
				plural = ""
			}
			return fmt.Sprintf("Loaded %d tool%s. They are available from your next call:\n%s", len(names), plural, strings.Join(lines, "\n")), nil
		},
	}
}

// mcpNamespaceOf describes the MCP server owning a tool name (nil otherwise).
func (e *Engine) mcpNamespaceOf(name string) *tools.SearchNamespace {
	if e.mcp == nil {
		return nil
	}
	e.mcp.mu.Lock()
	server, ok := e.mcp.owners[name]
	e.mcp.mu.Unlock()
	if !ok {
		return nil
	}
	cfg := e.mcp.manager.Config().Servers[server]
	return &tools.SearchNamespace{Name: gimcp.Namespace(server), Description: cfg.Description, Instructions: e.mcp.manager.Instructions(server)}
}
