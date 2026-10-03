package turn

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rcarmo/gi/internal/codemode"
	"github.com/rcarmo/gi/internal/config"
	gimcp "github.com/rcarmo/gi/internal/mcp"
	"github.com/rcarmo/gi/internal/tools"
	goai "github.com/rcarmo/go-ai"
)

// MCP integration (#25 phase 2). Servers come from Pi's mcp.json (.gi first,
// then .pi). Tools with direct exposure are registered like built-in tools,
// so the tool pipeline, hooks and permissions apply; codemode and deferred
// tools are catalogued for codemode and tool_search (later phases); hidden
// tools are unreachable. The mcp_servers prompt section lists servers whose
// tools are not declared.

// mcpDirectWait bounds how long the first prompt waits for servers with
// direct tools (Pi's startupWaitMs, 10 seconds). A variable for tests.
var mcpDirectWait = 10 * time.Second

// mcpOutputNamespace is the VFS namespace for full MCP outputs and binary
// resources, readable with the read tool (Pi uses temp files).
const mcpOutputNamespace = "mcp-output"

// mcpOutputRetention bounds how long saved outputs are kept; pruned at MCP
// start and every mcpOutputPruneEvery (Pi's temp files are left to the OS).
const (
	mcpOutputRetention  = 7 * 24 * time.Hour
	mcpOutputPruneEvery = 6 * time.Hour
)

// pruneMCPOutput deletes saved MCP outputs older than the retention period.
func (e *Engine) pruneMCPOutput(ctx context.Context) {
	for _, ns := range []string{mcpOutputNamespace, codemodeOutputNamespace} {
		if n, err := e.store.PruneVFSNamespace(ctx, ns, time.Now().Add(-mcpOutputRetention)); err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "database is closed") {
				return // the engine is shutting down
			}
			log.Printf("prune %s: %v", ns, err)
		} else if n > 0 {
			log.Printf("pruned %d saved outputs in %s older than %s", n, ns, mcpOutputRetention)
		}
	}
}

func (e *Engine) runMCPOutputPruner(ctx context.Context) {
	e.pruneMCPOutput(ctx)
	ticker := time.NewTicker(mcpOutputPruneEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.pruneMCPOutput(ctx)
		}
	}
}

// mcpTool is one MCP tool with its model-facing name and exposure.
type mcpTool struct {
	Name     string // mcp__server__tool
	Server   string
	Tool     *sdk.Tool
	Exposure string
}

type mcpState struct {
	mu        sync.Mutex
	manager   *gimcp.Manager
	byServer  map[string][]mcpTool // current tools per server
	owners    map[string]string    // model-facing name -> server
	ready     map[string]chan struct{}
	resources bool // resource tools registered
}

// EnableMCP loads the MCP configuration and starts connecting enabled
// servers in the background. Callers (the TUI and web entry points) opt in;
// engines built for tests never read the user's mcp.json.
func (e *Engine) EnableMCP() {
	cfg := gimcp.LoadConfig(gimcp.UserConfigPath(), gimcp.ProjectConfigPath(e.runtimeCfg.WorkspaceRoot), false)
	for _, err := range cfg.Errors {
		log.Printf("mcp: %v", err)
	}
	if len(cfg.Servers) == 0 {
		// Pi reports config errors even when no server is left to start.
		e.mcpConfigErrors = cfg.Errors
		e.mcpNotices.post("warning", gimcp.ProblemReport(nil, cfg.Errors))
		return
	}
	// gi writes its own log (Pi's is ~/.pi/agent/mcp.log); OAuth credentials
	// are shared with Pi (mcp-auth.json).
	e.EnableMCPConfig(cfg, gimcp.NewCredentialStore(config.UserConfigFile("mcp-auth.json")), config.UserConfigCandidates("mcp.log")[0])
}

// EnableMCPConfig enables MCP with an explicit configuration and OAuth
// credential store (EnableMCP reads the user's; tests pass their own).
func (e *Engine) EnableMCPConfig(cfg gimcp.Config, credentials *gimcp.CredentialStore, logPath string) {
	m := gimcp.NewManager(cfg, e.runtimeCfg.WorkspaceRoot, logPath)
	m.SetCredentials(credentials)
	e.enableMCPWith(m)
}

// enableMCPWith installs a manager (tests pass their own).
func (e *Engine) enableMCPWith(m *gimcp.Manager) {
	st := &mcpState{manager: m, byServer: map[string][]mcpTool{}, owners: map[string]string{}, ready: map[string]chan struct{}{}}
	for _, name := range m.Config().Names() {
		st.ready[name] = make(chan struct{})
	}
	e.mcp = st
	st.mu.Lock()
	e.updateToolSearchLocked() // from config, before servers connect (Pi)
	st.mu.Unlock()
	e.autoEnableCodemode(m.Config())
	m.SetOnToolsChanged(func(server string) { e.refreshMCPServer(e.backgroundContext(), server) })
	for _, name := range m.Config().Names() {
		if !m.Config().Servers[name].Enabled {
			close(st.ready[name])
			continue
		}
		go e.refreshMCPServer(e.backgroundContext(), name)
	}
	go e.reportMCPStartup(e.backgroundContext(), st)
}

// reportMCPStartup posts Pi's one-time "need attention" report (config
// errors, failed servers, required sign-ins) once every enabled server has
// finished its first connection attempt.
func (e *Engine) reportMCPStartup(ctx context.Context, st *mcpState) {
	st.mu.Lock()
	ready := make([]chan struct{}, 0, len(st.ready))
	for _, ch := range st.ready {
		ready = append(ready, ch)
	}
	st.mu.Unlock()
	for _, ch := range ready {
		select {
		case <-ch:
		case <-ctx.Done():
			return
		}
	}
	e.mcpNotices.post("warning", gimcp.ProblemReport(st.manager.Status(), st.manager.Config().Errors))
}

// mcpNotices holds MCP notices for the UI (Pi's ctx.ui.notify) until a
// notifier is set, so startup reports are not lost.
type mcpNotices struct {
	mu       sync.Mutex
	notify   func(level, text string)
	pending  [][2]string
	onChange func()
}

func (n *mcpNotices) changed() {
	n.mu.Lock()
	fn := n.onChange
	n.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func (n *mcpNotices) post(level, text string) {
	if text == "" {
		return
	}
	n.mu.Lock()
	notify := n.notify
	if notify == nil {
		n.pending = append(n.pending, [2]string{level, text})
	}
	n.mu.Unlock()
	if notify != nil {
		notify(level, text)
	} else {
		log.Printf("mcp: %s", text)
	}
}

// SetMCPNotifier receives MCP notices (level "info" or "warning"), including
// ones posted before it was set. The TUI shows them; without a notifier they
// are only logged.
func (e *Engine) SetMCPNotifier(fn func(level, text string)) {
	e.mcpNotices.mu.Lock()
	e.mcpNotices.notify = fn
	pending := e.mcpNotices.pending
	e.mcpNotices.pending = nil
	e.mcpNotices.mu.Unlock()
	for _, n := range pending {
		fn(n[0], n[1])
	}
}

// MCPManager exposes the manager for /mcp and later phases (nil when off).
func (e *Engine) MCPManager() *gimcp.Manager {
	if e.mcp == nil {
		return nil
	}
	return e.mcp.manager
}

func (e *Engine) closeMCP() {
	if e.mcp != nil {
		e.mcp.manager.Close()
	}
}

// refreshMCPServer lists a server's tools and (re)registers them.
func (e *Engine) refreshMCPServer(ctx context.Context, server string) {
	st := e.mcp
	defer func() {
		st.mu.Lock()
		if ch := st.ready[server]; ch != nil {
			select {
			case <-ch:
			default:
				close(ch)
			}
		}
		st.mu.Unlock()
	}()
	defer e.mcpNotices.changed()
	list, err := st.manager.Tools(ctx, server)
	if err != nil {
		log.Printf("mcp: %v", err)
		e.registerMCPTools(server, nil)
		return
	}
	e.registerMCPTools(server, list)
}

// registerMCPTools assigns Pi names, registers direct tools and catalogues
// the rest; withdrawn tools are unregistered.
func (e *Engine) registerMCPTools(server string, list []*sdk.Tool) {
	st := e.mcp
	cfg := st.manager.Config().Servers[server]
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, old := range st.byServer[server] {
		e.tools.Unregister(old.Name)
		delete(st.owners, old.Name)
	}
	names := make([]string, 0, len(list))
	for _, t := range list {
		if t != nil && t.Name != "" {
			names = append(names, t.Name)
		}
	}
	assigned := gimcp.ServerToolNames(server, names, func(candidate string) bool {
		owner, ok := st.owners[candidate]
		return ok && owner != server
	})
	var current []mcpTool
	for _, t := range list {
		if t == nil || t.Name == "" {
			continue
		}
		mt := mcpTool{Name: assigned[t.Name], Server: server, Tool: t, Exposure: cfg.ToolExposureFor(t.Name)}
		st.owners[mt.Name] = server
		current = append(current, mt)
		if mt.Exposure == gimcp.ExposureHidden {
			continue // registered nowhere: unreachable
		}
		// codemode/deferred tools are registered deferred: executable, but
		// declared only once loaded (tool_search) or called from codemode.
		reg := e.mcpRegisteredTool(mt)
		reg.Deferred = mt.Exposure != gimcp.ExposureDirect
		if err := e.tools.Register(reg); err != nil {
			log.Printf("mcp: register %s: %v", mt.Name, err)
		}
	}
	st.byServer[server] = current
	e.updateMCPResourceToolsLocked()
	e.updateToolSearchLocked()
}

func (e *Engine) mcpRegisteredTool(mt mcpTool) tools.RegisteredTool {
	desc := strings.TrimSpace(mt.Tool.Description)
	if desc == "" {
		desc = strings.TrimSpace(mt.Tool.Title)
	}
	if desc == "" && mt.Tool.Annotations != nil {
		desc = strings.TrimSpace(mt.Tool.Annotations.Title)
	}
	if desc == "" {
		desc = fmt.Sprintf("MCP tool %s from server %s", mt.Tool.Name, mt.Server)
	}
	params := json.RawMessage(`{"type":"object","properties":{}}`)
	if mt.Tool.InputSchema != nil {
		if raw, err := json.Marshal(mt.Tool.InputSchema); err == nil && len(raw) > 2 {
			params = raw
		}
	}
	kind := "mixed"
	if a := mt.Tool.Annotations; a != nil && a.ReadOnlyHint {
		kind = "read"
	}
	server, toolName := mt.Server, mt.Tool.Name
	var structured json.RawMessage
	if mt.Tool.OutputSchema != nil {
		structured, _ = json.Marshal(mt.Tool.OutputSchema)
	}
	return tools.RegisteredTool{
		Name: mt.Name, Description: desc, Parameters: params, Source: "mcp:" + server, Kind: kind,
		// Scripts receive the whole CallToolResult (without _meta), as in Pi.
		OutputSchema: codemode.MCPResultSchema(structured),
		StructuredExecutor: func(ctx context.Context, _ tools.ToolRuntime, call goai.ToolCall) (json.RawMessage, bool, error) {
			result, err := e.mcp.manager.CallTool(ctx, server, toolName, nonNilArgs(call.Arguments))
			if err != nil {
				return nil, false, err
			}
			raw, err := json.Marshal(result)
			if err != nil {
				return nil, false, err
			}
			var obj map[string]any
			if err := json.Unmarshal(raw, &obj); err == nil {
				delete(obj, "_meta")
				raw, _ = json.Marshal(obj)
			}
			return raw, result.IsError, nil
		},
		Executor: func(ctx context.Context, rt tools.ToolRuntime, call goai.ToolCall) (string, error) {
			result, err := e.mcp.manager.CallTool(ctx, server, toolName, nonNilArgs(call.Arguments))
			if err != nil {
				return "", err
			}
			converted := gimcp.ConvertResult(server, toolName, result, gimcp.ConvertOptions{
				Save: e.mcpSaver(ctx, rt.SessionID), ReadableResources: e.mcp.manager.HasResources(server),
				AttachImages: rt.AttachImage != nil,
			})
			for _, img := range converted.Images {
				rt.AttachImage(img.MIMEType, img.Data)
			}
			if converted.IsError {
				return "", errors.New(converted.Text)
			}
			return converted.Text, nil
		},
	}
}

func nonNilArgs(args map[string]any) map[string]any {
	if args == nil {
		return map[string]any{}
	}
	return args
}

// mcpSaver stores full outputs and binary resources in the managed VFS.
func (e *Engine) mcpSaver(ctx context.Context, sessionID string) gimcp.SaveFunc {
	return func(data []byte, ext string) (string, error) {
		var id [8]byte
		if _, err := rand.Read(id[:]); err != nil {
			return "", err
		}
		if sessionID == "" {
			sessionID = "shared"
		}
		path := sessionID + "/" + hex.EncodeToString(id[:]) + ext
		contentType := "application/octet-stream"
		if ext == ".txt" {
			contentType = "text/plain; charset=utf-8"
		}
		if _, err := e.store.SaveVFSFile(ctx, mcpOutputNamespace, path, contentType, data, map[string]any{"source": "mcp"}); err != nil {
			return "", err
		}
		return "vfs://" + mcpOutputNamespace + "/" + path, nil
	}
}

// awaitDirectMCPTools ports Pi's waitForDirectServers: the first prompt
// waits (at most mcpDirectWait) for servers that can give tools direct
// exposure, so they are part of its admitted tool set. Later prompts do not
// wait; a server that connects later declares its tools from then on.
func (e *Engine) awaitDirectMCPTools(ctx context.Context) {
	if e.mcp == nil || e.mcpWaited.Swap(true) {
		return
	}
	deadline := time.NewTimer(mcpDirectWait)
	defer deadline.Stop()
	for _, name := range e.mcp.manager.Config().Names() {
		cfg := e.mcp.manager.Config().Servers[name]
		if !cfg.Enabled || !cfg.HasDirectTools() {
			continue
		}
		e.mcp.mu.Lock()
		ch := e.mcp.ready[name]
		e.mcp.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			return
		case <-deadline.C:
			e.mcpNotices.post("info", gimcp.StillConnectingNotice)
			return
		}
	}
}

// mcpServersSection renders the mcp_servers system-prompt section.
func (e *Engine) mcpServersSection() string {
	if e.mcp == nil {
		return ""
	}
	cfg := e.mcp.manager.Config()
	servers := make([]gimcp.SectionServer, 0, len(cfg.Servers))
	for _, name := range cfg.Names() {
		servers = append(servers, gimcp.SectionServer{Config: cfg.Servers[name], Instructions: e.mcp.manager.Instructions(name)})
	}
	section := gimcp.RenderServersSection(servers)
	if section == "" {
		return ""
	}
	return "<" + gimcp.ServersSection + ">\n" + section + "\n</" + gimcp.ServersSection + ">"
}

// resourceServersLocked lists connected, enabled, non-hidden servers offering
// resources, and the widest exposure among them (direct beats indirect).
func (e *Engine) resourceServersLocked() ([]string, string) {
	cfg := e.mcp.manager.Config()
	var out []string
	widest := ""
	for _, name := range cfg.Names() {
		sc := cfg.Servers[name]
		if !sc.Enabled || sc.Exposure == gimcp.ExposureHidden || !e.mcp.manager.HasResources(name) {
			continue
		}
		out = append(out, name)
		switch {
		case sc.Exposure == gimcp.ExposureDirect:
			widest = gimcp.ExposureDirect
		case widest == "":
			widest = sc.Exposure
		}
	}
	return out, widest
}

// updateMCPResourceToolsLocked registers Pi's resource tools while a server
// with resources has direct exposure (indirect exposure reaches them through
// codemode and tool_search in later phases).
func (e *Engine) updateMCPResourceToolsLocked() {
	servers, widest := e.resourceServersLocked()
	want := len(servers) > 0 && widest == gimcp.ExposureDirect
	if want == e.mcp.resources {
		return
	}
	e.mcp.resources = want
	names := []string{"list_mcp_resources", "list_mcp_resource_templates", gimcp.ReadResourceTool}
	if !want {
		for _, n := range names {
			e.tools.Unregister(n)
		}
		return
	}
	for _, t := range e.mcpResourceTools() {
		if err := e.tools.Register(t); err != nil {
			log.Printf("mcp: register %s: %v", t.Name, err)
		}
	}
}

func (e *Engine) mcpResourceTools() []tools.RegisteredTool {
	listParams := json.RawMessage(`{"type":"object","properties":{"server":{"type":"string","description":"Server name; omit to list every server"},"cursor":{"type":"string","description":"Cursor from a previous page (with server)"}}}`)
	readParams := json.RawMessage(`{"type":"object","properties":{"server":{"type":"string"},"uri":{"type":"string"}},"required":["server","uri"]}`)
	return []tools.RegisteredTool{
		{Name: "list_mcp_resources", Description: "List resources offered by MCP servers.", Parameters: listParams, Source: "mcp", Kind: "read",
			Executor: func(ctx context.Context, _ tools.ToolRuntime, call goai.ToolCall) (string, error) {
				return e.listMCPResources(ctx, call.Arguments, false)
			}},
		{Name: "list_mcp_resource_templates", Description: "List resource URI templates offered by MCP servers.", Parameters: listParams, Source: "mcp", Kind: "read",
			Executor: func(ctx context.Context, _ tools.ToolRuntime, call goai.ToolCall) (string, error) {
				return e.listMCPResources(ctx, call.Arguments, true)
			}},
		{Name: gimcp.ReadResourceTool, Description: "Read an MCP resource by server and URI.", Parameters: readParams, Source: "mcp", Kind: "read",
			Executor: func(ctx context.Context, rt tools.ToolRuntime, call goai.ToolCall) (string, error) {
				server, _ := call.Arguments["server"].(string)
				uri, _ := call.Arguments["uri"].(string)
				if server == "" || uri == "" {
					return "", errors.New("server and uri are required")
				}
				res, err := e.mcp.manager.ReadResource(ctx, server, uri)
				if err != nil {
					return "", err
				}
				blocks := make([]sdk.Content, 0, len(res.Contents))
				for _, c := range res.Contents {
					blocks = append(blocks, &sdk.EmbeddedResource{Resource: c})
				}
				converted := gimcp.ConvertResult(server, gimcp.ReadResourceTool, &sdk.CallToolResult{Content: blocks}, gimcp.ConvertOptions{Save: e.mcpSaver(ctx, rt.SessionID), AttachImages: rt.AttachImage != nil})
				for _, img := range converted.Images {
					rt.AttachImage(img.MIMEType, img.Data)
				}
				return converted.Text, nil
			}},
	}
}

// listMCPResources ports Pi's list tools: with server, one page; without,
// every resource from every server.
func (e *Engine) listMCPResources(ctx context.Context, args map[string]any, templates bool) (string, error) {
	server, _ := args["server"].(string)
	cursor, _ := args["cursor"].(string)
	e.mcp.mu.Lock()
	servers, _ := e.resourceServersLocked()
	e.mcp.mu.Unlock()
	if server != "" {
		servers = []string{server}
	}
	key := "resources"
	if templates {
		key = "resourceTemplates"
	}
	var items []map[string]any
	out := map[string]any{}
	for _, name := range servers {
		pageCursor := cursor
		for page := 0; page < 64; page++ {
			var raw any
			var next string
			if templates {
				res, err := e.mcp.manager.ListResourceTemplates(ctx, name, pageCursor)
				if err != nil {
					return "", err
				}
				raw, next = res.ResourceTemplates, res.NextCursor
			} else {
				res, err := e.mcp.manager.ListResources(ctx, name, pageCursor)
				if err != nil {
					return "", err
				}
				raw, next = res.Resources, res.NextCursor
			}
			var entries []map[string]any
			if b, err := json.Marshal(raw); err == nil {
				_ = json.Unmarshal(b, &entries)
			}
			for _, entry := range entries {
				entry["server"] = name
				items = append(items, entry)
			}
			if server != "" { // one page with an explicit server
				if next != "" {
					out["nextCursor"] = next
				}
				break
			}
			if next == "" {
				break
			}
			pageCursor = next
		}
	}
	if server != "" {
		out["server"] = server
	}
	if items == nil {
		items = []map[string]any{}
	}
	sort.SliceStable(items, func(i, j int) bool { return fmt.Sprint(items[i]["server"]) < fmt.Sprint(items[j]["server"]) })
	out[key] = items
	b, err := json.MarshalIndent(out, "", "  ")
	return string(b), err
}

// updateToolSearchLocked registers tool_search while any enabled server can
// give its tools deferred exposure (Pi activates tool_search for them).
func (e *Engine) updateToolSearchLocked() {
	want := false
	cfg := e.mcp.manager.Config()
	for _, name := range cfg.Names() {
		sc := cfg.Servers[name]
		if sc.Enabled && sc.HasDeferredTools() {
			want = true
			break
		}
	}
	_, have := e.tools.GetRegistered(tools.ToolSearchName)
	switch {
	case want && !have:
		if err := e.tools.Register(e.toolSearchTool()); err != nil {
			log.Printf("mcp: register tool_search: %v", err)
		}
	case !want && have:
		e.tools.Unregister(tools.ToolSearchName)
	}
}

// MCPStatus reports the configured MCP servers (nil without MCP).
func (e *Engine) MCPStatus() ([]gimcp.Status, []error) {
	if e.mcp == nil {
		return nil, e.mcpConfigErrors
	}
	return e.mcp.manager.Status(), e.mcp.manager.Config().Errors
}

// MCPReconnect reconnects a server and re-registers its tools.
func (e *Engine) MCPReconnect(ctx context.Context, name string) error {
	if e.mcp == nil {
		return fmt.Errorf("No MCP servers configured.")
	}
	if err := e.mcp.manager.Reconnect(ctx, name); err != nil {
		return err
	}
	e.refreshMCPServer(ctx, name)
	return nil
}

// MCPUsesOAuth reports an enabled HTTP server without an Authorization
// header (Pi's usesOAuth), when OAuth credentials are available.
func (e *Engine) MCPUsesOAuth(name string) bool {
	if e.mcp == nil || e.mcp.manager.Credentials() == nil {
		return false
	}
	sc, ok := e.mcp.manager.Config().Servers[name]
	return ok && sc.Enabled && sc.UsesOAuth()
}

// MCPSignIn runs the browser sign-in for a server (Pi's signIn) and
// reconnects it; the error text follows Pi's messages.
func (e *Engine) MCPSignIn(ctx context.Context, name string, prompt gimcp.SignInPrompt) error {
	if !e.MCPUsesOAuth(name) {
		return fmt.Errorf("MCP server %q does not use OAuth.", name)
	}
	if err := e.mcp.manager.SignIn(ctx, name, prompt); err != nil {
		if errors.Is(err, gimcp.ErrSignInCancelled) || errors.Is(err, context.Canceled) {
			return gimcp.ErrSignInCancelled
		}
		return fmt.Errorf("Sign-in failed: %v", err)
	}
	if err := e.MCPReconnect(ctx, name); err != nil {
		return fmt.Errorf("Signed in, but %v", err)
	}
	return nil
}

// MCPSignOut deletes a server's stored credentials and drops its connection,
// so it shows needs-auth (Pi's signOut); it reports whether any were stored.
func (e *Engine) MCPSignOut(ctx context.Context, name string) bool {
	if !e.MCPUsesOAuth(name) {
		return false
	}
	removed := e.mcp.manager.Credentials().Remove(name, e.mcp.manager.Config().Servers[name].URL)
	_ = e.MCPReconnect(ctx, name) // fails with needs-auth now
	return removed
}

// MCPSetEnabled ports Pi's setEnabled: the change is saved to the mcp.json
// that defines the server. A disabled server's tools are withdrawn; an
// enabled one connects now (its connection error shows in its state).
func (e *Engine) MCPSetEnabled(ctx context.Context, name string, enabled bool) error {
	if e.mcp == nil {
		return fmt.Errorf("No MCP servers configured.")
	}
	if err := e.mcp.manager.UpdateServer(name, gimcp.ServerPatch{Enabled: &enabled}); err != nil {
		return err
	}
	if enabled {
		e.autoEnableCodemode(e.mcp.manager.Config())
		e.refreshMCPServer(ctx, name)
	} else {
		e.registerMCPTools(name, nil)
	}
	e.mcpNotices.changed()
	return nil
}

// MCPSetExposure ports Pi's setExposure: saved to the server's mcp.json, and
// a connected server's tools are registered again with the new exposure.
func (e *Engine) MCPSetExposure(ctx context.Context, name, exposure string) error {
	if e.mcp == nil {
		return fmt.Errorf("No MCP servers configured.")
	}
	if err := e.mcp.manager.UpdateServer(name, gimcp.ServerPatch{Exposure: exposure}); err != nil {
		return err
	}
	e.autoEnableCodemode(e.mcp.manager.Config())
	for _, st := range e.mcp.manager.Status() {
		if st.Name == name && st.State == gimcp.StateConnected {
			e.refreshMCPServer(ctx, name)
		}
	}
	e.mcpNotices.changed()
	return nil
}

// MCPTool is one tool a server offers, with its effective exposure.
type MCPTool struct {
	Name, Description, Exposure string
}

// MCPServerTools lists the tools a server offers, in the server's order.
func (e *Engine) MCPServerTools(name string) []MCPTool {
	if e.mcp == nil {
		return nil
	}
	e.mcp.mu.Lock()
	defer e.mcp.mu.Unlock()
	var out []MCPTool
	for _, mt := range e.mcp.byServer[name] {
		out = append(out, MCPTool{Name: mt.Tool.Name, Description: mt.Tool.Description, Exposure: mt.Exposure})
	}
	return out
}

// SetMCPChangeListener is called (from any goroutine) when a server's
// connection or settings change, so open views can redraw (Pi's subscribe).
func (e *Engine) SetMCPChangeListener(fn func()) {
	e.mcpNotices.mu.Lock()
	e.mcpNotices.onChange = fn
	e.mcpNotices.mu.Unlock()
}

// MCPServerConfig returns a configured server's settings.
func (e *Engine) MCPServerConfig(name string) (gimcp.ServerConfig, bool) {
	if e.mcp == nil {
		return gimcp.ServerConfig{}, false
	}
	sc, ok := e.mcp.manager.Config().Servers[name]
	return sc, ok
}

// MCPPickServer ports Pi's pickServer for a /mcp action (login, logout,
// reconnect): the named server, else the only eligible one, else the only
// preferred one. Otherwise it returns the candidates to choose from; a
// message means there is nothing to pick.
func (e *Engine) MCPPickServer(action, name string) (server string, candidates []string, message string) {
	eligible := func(st gimcp.Status) bool { return e.MCPUsesOAuth(st.Name) }
	preferred := func(st gimcp.Status) bool { return st.State == gimcp.StateNeedsAuth }
	none := "No enabled MCP server uses OAuth. Only HTTP servers without an Authorization header do."
	if action == "reconnect" {
		eligible = func(st gimcp.Status) bool { return st.State != gimcp.StateDisabled }
		preferred = func(st gimcp.Status) bool {
			return st.State == gimcp.StateFailed || st.State == gimcp.StateDisconnected
		}
		none = "No enabled MCP server to reconnect."
	}
	statuses, _ := e.MCPStatus()
	if name != "" {
		for _, st := range statuses {
			if st.Name == name {
				if !eligible(st) {
					return "", nil, none
				}
				return name, nil, ""
			}
		}
		return "", nil, fmt.Sprintf("No MCP server named %q.", name)
	}
	var wanted []string
	for _, st := range statuses {
		if eligible(st) {
			candidates = append(candidates, st.Name)
			if preferred(st) {
				wanted = append(wanted, st.Name)
			}
		}
	}
	switch {
	case len(candidates) == 0:
		return "", nil, none
	case len(candidates) == 1:
		return candidates[0], nil, ""
	case len(wanted) == 1:
		return wanted[0], nil, ""
	}
	return "", candidates, ""
}

// MCPLogoutCommand and MCPReconnectCommand run /mcp logout and reconnect on
// a resolved server and return the message to show.
func (e *Engine) MCPLogoutCommand(ctx context.Context, server string) string {
	if e.MCPSignOut(ctx, server) {
		return fmt.Sprintf("Signed out of MCP server %q.", server)
	}
	return fmt.Sprintf("No stored credentials for MCP server %q.", server)
}

func (e *Engine) MCPReconnectCommand(ctx context.Context, server string) string {
	if err := e.MCPReconnect(ctx, server); err != nil {
		return err.Error()
	}
	statuses, _ := e.MCPStatus()
	for _, st := range statuses {
		if st.Name == server {
			return fmt.Sprintf("Reconnected to MCP server %q (%s).", server, gimcp.DescribeState(st))
		}
	}
	return fmt.Sprintf("Reconnected to MCP server %q.", server)
}

// MCPSignInResult is the message after a /mcp login sign-in finishes.
func (e *Engine) MCPSignInResult(server string, err error) string {
	switch {
	case errors.Is(err, gimcp.ErrSignInCancelled):
		return "Sign-in cancelled."
	case err != nil:
		return err.Error()
	}
	tools := 0
	statuses, _ := e.MCPStatus()
	if st, ok := findMCPStatus(statuses, server); ok {
		tools = st.Tools
	}
	return fmt.Sprintf("Signed in to MCP server %q (%d tools).", server, tools)
}

func findMCPStatus(statuses []gimcp.Status, name string) (gimcp.Status, bool) {
	for _, st := range statuses {
		if st.Name == name {
			return st, true
		}
	}
	return gimcp.Status{}, false
}
