package web

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	gimcp "github.com/rcarmo/gi/internal/mcp"
)

// /mcp in the web UI: Pi's /mcp without a TUI (formatStatus, login, logout,
// reconnect). Replies are system messages in the session's timeline, which
// are not model context; slow actions reply when they finish.

// webMCPSignIn is a sign-in waiting for its redirect: the browser cannot
// always reach gi's loopback callback, so the URL it was sent to can be
// pasted with /mcp login <server> <url> (Pi asks with ctx.ui.input).
type webMCPSignIn struct {
	cancel context.CancelFunc
	paste  chan string
}

type webMCPSignIns struct {
	mu      sync.Mutex
	pending map[string]*webMCPSignIn // server name
}

func (s *Server) handleMCPCommand(w http.ResponseWriter, r *http.Request, sessionID, prompt string) bool {
	fields := strings.Fields(prompt)
	if len(fields) == 0 || fields[0] != "/mcp" {
		return false
	}
	if _, err := s.store.GetSession(r.Context(), sessionID); err != nil {
		writeJSON(w, 404, map[string]any{"error": err.Error()})
		return true
	}
	post := func(text string) {
		_, _ = s.turns.PostSystemMessage(context.Background(), sessionID, text, map[string]any{"kind": "mcp", "command": prompt})
	}
	if reply := s.mcpCommand(sessionID, fields[1:], post); reply != "" {
		post(reply)
	}
	writeJSON(w, 200, map[string]any{"thread_id": nil, "ui_only": true})
	return true
}

// mcpCommand runs /mcp and returns its immediate reply; later replies go to
// post.
func (s *Server) mcpCommand(sessionID string, args []string, post func(string)) string {
	if len(args) == 0 {
		statuses, errs := s.turns.MCPStatus()
		return gimcp.FormatStatus(statuses, errs)
	}
	action, name := args[0], ""
	if len(args) > 1 {
		name = args[1]
	}
	if action == "login" && len(args) == 3 {
		return s.mcpPasteRedirect(name, args[2])
	}
	if len(args) > 2 || (action != "login" && action != "logout" && action != "reconnect") {
		return gimcp.CommandUsage
	}
	server, candidates, message := s.turns.MCPPickServer(action, name)
	switch {
	case message != "":
		return message
	case server == "":
		// Pi asks with ctx.ui.select, which the web UI does not have.
		return fmt.Sprintf("Several MCP servers qualify; run /mcp %s <server> with one of: %s.", action, strings.Join(candidates, ", "))
	}
	switch action {
	case "logout":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return s.turns.MCPLogoutCommand(ctx, server)
	case "reconnect":
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			post(s.turns.MCPReconnectCommand(ctx, server))
		}()
		return ""
	}
	return s.startMCPSignIn(server, post)
}

func (s *Server) startMCPSignIn(server string, post func(string)) string {
	s.mcpSignIns.mu.Lock()
	defer s.mcpSignIns.mu.Unlock()
	if s.mcpSignIns.pending[server] != nil {
		return fmt.Sprintf("Already signing in to MCP server %q.", server)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	state := &webMCPSignIn{cancel: cancel, paste: make(chan string, 1)}
	if s.mcpSignIns.pending == nil {
		s.mcpSignIns.pending = map[string]*webMCPSignIn{}
	}
	s.mcpSignIns.pending[server] = state
	prompt := gimcp.SignInPrompt{
		ShowAuthorizationURL: func(u string) {
			post(fmt.Sprintf("Sign in to MCP server %q in your browser:\n%s\n\nIf the browser cannot reach this machine, run /mcp login %s <the URL it was redirected to>.", server, u, server))
		},
		PromptForRedirectURL: func(ctx context.Context) string {
			select {
			case u := <-state.paste:
				return u
			case <-ctx.Done():
				return ""
			}
		},
	}
	go func() {
		err := s.turns.MCPSignIn(ctx, server, prompt)
		cancel()
		s.mcpSignIns.mu.Lock()
		delete(s.mcpSignIns.pending, server)
		s.mcpSignIns.mu.Unlock()
		post(s.turns.MCPSignInResult(server, err))
	}()
	return ""
}

func (s *Server) mcpPasteRedirect(server, redirect string) string {
	s.mcpSignIns.mu.Lock()
	state := s.mcpSignIns.pending[server]
	s.mcpSignIns.mu.Unlock()
	if state == nil {
		return fmt.Sprintf("No sign-in to MCP server %q is waiting for a redirect URL.", server)
	}
	select {
	case state.paste <- redirect:
	default:
	}
	return ""
}
