package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// `gi mcp`: add, remove and check MCP servers without starting a session,
// ported from Pi's `pi mcp` (extensions/mcp/cli.js). Agents run it through
// bash to configure servers and verify an mcp.json they wrote.

const cliHelp = `Usage:
  gi mcp add <server> [options] -- <command> [args...]
  gi mcp add <server> [options] --url <url>
  gi mcp remove <server> [-l]
  gi mcp list [--json]
  gi mcp login <server> [--timeout <seconds>]
  gi mcp logout <server>

Configure and check MCP servers and sign in to OAuth servers without starting a session.
Reads ~/.gi/agent/mcp.json (else ~/.pi/agent/mcp.json); project .gi/mcp.json and .pi/mcp.json
are not read until gi has project trust.

Commands:
  add <server>            Add or replace a server in mcp.json
  remove <server>         Remove a server from mcp.json
  list                    Show state, tools, and errors (exits 1 on failure)
  login <server>          Sign in through the browser
  logout <server>         Delete the stored OAuth credentials

Options for add and remove:
  -l, --local             Use the project's mcp.json instead of the global file

Options for add:
  --url <url>             Streamable HTTP server URL (instead of a command)
  --env <KEY=VALUE>       Environment variable for a stdio server (repeatable)
  --cwd <dir>             Working directory for a stdio server
  --header <KEY=VALUE>    HTTP header (repeatable)
  --bearer-token-env-var <NAME>
                          Send "Authorization: Bearer ${NAME}"
  --oauth-client-id <id>  Pre-registered OAuth client id
  --oauth-client-secret <secret>
                          OAuth client secret (may be ${NAME} or !command)
  --oauth-callback-port <port>
                          Fixed OAuth callback port
  --oauth-client-name <name>
                          Client name sent when registering with the OAuth server
  --exposure <mode>       codemode (default), deferred, direct, or hidden
  --description <text>    What the server offers, shown in the system prompt

Other options:
  --json                  Print the list as JSON
  --timeout <seconds>     How long login waits for the browser (default: 300)`

const cliHelpHint = `Use "gi mcp --help" for usage.`

// CLIOptions configure RunCommand.
type CLIOptions struct {
	Cwd             string
	UserPath        string // user mcp.json (written by add/remove)
	ProjectPath     string // project mcp.json (add/remove -l)
	LogPath         string
	CredentialsPath string           // mcp-auth.json
	Credentials     *CredentialStore // overrides CredentialsPath (tests)
	Log, Error      func(string)
	// OpenURL opens the authorization page (default: the system browser);
	// when set, login does not read a pasted redirect URL from stdin.
	OpenURL func(string)
}

const defaultLoginTimeoutSeconds = 300

// cliLogin ports Pi's login: connect first (already signed in, or the
// server's challenge), then the browser flow, then reconnect.
func cliLogin(name string, cfg Config, credentials *CredentialStore, timeout time.Duration, opts CLIOptions, logf, errf func(string)) int {
	m := NewManager(cfg, opts.Cwd, opts.LogPath)
	m.SetCredentials(credentials)
	defer m.Close()
	ctx := context.Background()
	connectCtx, cancel := context.WithTimeout(ctx, cfg.Servers[name].Timeout+5*time.Second)
	tools, err := m.Tools(connectCtx, name)
	cancel()
	if err == nil {
		logf(fmt.Sprintf("Already signed in to MCP server %q (%d tools).", name, len(tools)))
		return 0
	}
	if st := m.serverStatus(name); st.State != StateNeedsAuth {
		msg := st.Error
		if msg == "" {
			msg = "unknown error"
		}
		errf(fmt.Sprintf("MCP server %q failed to connect: %s", name, msg))
		return 1
	}
	openURL := opts.OpenURL
	interactive := openURL == nil && stdinIsTerminal()
	if openURL == nil {
		openURL = OpenBrowser
	}
	signCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	prompt := SignInPrompt{ShowAuthorizationURL: func(u string) {
		logf(fmt.Sprintf("Sign in to MCP server %q in your browser:\n%s", name, u))
		openURL(u)
	}}
	if interactive {
		prompt.PromptForRedirectURL = readRedirectURL
	}
	if err := m.SignIn(signCtx, name, prompt); err != nil {
		if errors.Is(err, ErrSignInCancelled) || errors.Is(err, context.DeadlineExceeded) {
			errf(fmt.Sprintf("Sign-in to MCP server %q was cancelled or not completed within %d seconds.", name, int(timeout.Round(time.Second)/time.Second)))
		} else {
			errf(fmt.Sprintf("Sign-in to MCP server %q failed: %v", name, err))
		}
		return 1
	}
	reconnectCtx, cancel2 := context.WithTimeout(ctx, cfg.Servers[name].Timeout+5*time.Second)
	defer cancel2()
	if err := m.Reconnect(reconnectCtx, name); err != nil {
		errf("Signed in, but " + err.Error())
		return 1
	}
	logf(fmt.Sprintf("Signed in to MCP server %q (%d tools).", name, m.serverStatus(name).Tools))
	return 0
}

func (m *Manager) serverStatus(name string) Status {
	for _, st := range m.Status() {
		if st.Name == name {
			return st
		}
	}
	return Status{}
}

// readRedirectURL reads a pasted redirect URL from stdin (Pi's prompt for
// browsers that cannot reach this machine).
func readRedirectURL(ctx context.Context) string {
	fmt.Fprint(os.Stderr, "If the browser cannot reach this machine, paste the URL it was redirected to: ")
	line := make(chan string, 1)
	go func() {
		var buf [8192]byte
		n, _ := os.Stdin.Read(buf[:])
		line <- string(buf[:n])
	}()
	select {
	case <-ctx.Done():
		return ""
	case l := <-line:
		return strings.TrimSpace(l)
	}
}

func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// OpenBrowser opens u in the system browser, best effort.
func OpenBrowser(u string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	if err := cmd.Start(); err == nil {
		go func() { _ = cmd.Wait() }()
	}
}

type parsedOptions struct {
	positional []string
	values     map[string]string
	flags      map[string]bool
	lists      map[string][]string
}

// parseCLIOptions ports Pi's parseOptions: "--" ends options, as does
// reaching maxPositionals positional arguments.
func parseCLIOptions(args []string, known map[string]string, errf func(string), maxPositionals int) *parsedOptions {
	p := &parsedOptions{values: map[string]string{}, flags: map[string]bool{}, lists: map[string][]string{}}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-l" {
			arg = "--local"
		}
		if arg == "--" || (maxPositionals >= 0 && len(p.positional) >= maxPositionals) {
			if arg == "--" {
				p.positional = append(p.positional, args[i+1:]...)
			} else {
				p.positional = append(p.positional, args[i:]...)
			}
			break
		}
		if !strings.HasPrefix(arg, "--") {
			p.positional = append(p.positional, arg)
			continue
		}
		name := arg[2:]
		kind := known[name]
		if kind == "" {
			errf("Unknown option " + arg + ".\n" + cliHelpHint)
			return nil
		}
		if kind == "flag" {
			p.flags[name] = true
			continue
		}
		i++
		if i >= len(args) {
			errf(arg + " needs a value.")
			return nil
		}
		if kind == "list" {
			p.lists[name] = append(p.lists[name], args[i])
		} else {
			p.values[name] = args[i]
		}
	}
	return p
}

// RunCommand runs `gi mcp <args>` and returns the exit code.
func RunCommand(args []string, opts CLIOptions) int {
	logf, errf := opts.Log, opts.Error
	if logf == nil {
		logf = func(s string) { fmt.Println(s) }
	}
	if errf == nil {
		errf = func(s string) { fmt.Fprintln(os.Stderr, s) }
	}
	if len(args) == 0 || args[0] == "help" || contains(args, "--help") || contains(args, "-h") {
		logf(cliHelp)
		return 0
	}
	command, rest := args[0], args[1:]
	switch command {
	case "add":
		return cliAdd(rest, opts, logf, errf)
	case "remove":
		return cliRemove(rest, opts, logf, errf)
	}
	cfg := LoadConfig(opts.UserPath, opts.ProjectPath, false)
	untrustedNote := ""
	if _, err := os.Stat(opts.ProjectPath); err == nil {
		untrustedNote = opts.ProjectPath + " is ignored because gi does not read project MCP configuration until it has project trust."
	}
	switch command {
	case "list":
		p := parseCLIOptions(rest, map[string]string{"json": "flag"}, errf, -1)
		if p == nil {
			return 1
		}
		if len(p.positional) > 0 {
			errf("Usage: gi mcp list [--json]\n" + cliHelpHint)
			return 1
		}
		return cliList(cfg, p.flags["json"], untrustedNote, opts, logf)
	case "login", "logout":
		known := map[string]string{}
		if command == "login" {
			known["timeout"] = "value"
		}
		p := parseCLIOptions(rest, known, errf, -1)
		if p == nil {
			return 1
		}
		if len(p.positional) != 1 {
			errf("Usage: gi mcp " + command + " <server>\n" + cliHelpHint)
			return 1
		}
		name := p.positional[0]
		if _, ok := cfg.Servers[name]; !ok {
			note := ""
			if untrustedNote != "" {
				note = " " + untrustedNote
			}
			configured := strings.Join(cfg.Names(), ", ")
			if configured == "" {
				configured = "none"
			}
			errf(fmt.Sprintf("No MCP server named %q.%s Configured: %s.", name, note, configured))
			return 1
		}
		entry := cfg.Servers[name]
		if !entry.UsesOAuth() {
			errf(fmt.Sprintf("MCP server %q does not use OAuth. Only HTTP servers without an Authorization header do.", name))
			return 1
		}
		credentials := opts.Credentials
		if credentials == nil {
			credentials = NewCredentialStore(opts.CredentialsPath)
		}
		if command == "logout" {
			if credentials.Remove(name, entry.URL) {
				logf(fmt.Sprintf("Signed out of MCP server %q.", name))
			} else {
				logf(fmt.Sprintf("No stored credentials for MCP server %q.", name))
			}
			return 0
		}
		timeout := float64(defaultLoginTimeoutSeconds)
		if v, ok := p.values["timeout"]; ok {
			n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil || n <= 0 {
				errf("--timeout must be a positive number of seconds.")
				return 1
			}
			timeout = n
		}
		return cliLogin(name, cfg, credentials, time.Duration(timeout*float64(time.Second)), opts, logf, errf)
	}
	errf(fmt.Sprintf("Unknown mcp command %q.\n%s", command, cliHelpHint))
	return 1
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// orderedObject is a JSON object that keeps its key order (JS semantics).
type orderedObject struct {
	keys   []string
	values map[string]json.RawMessage
}

func (o *orderedObject) set(key string, value json.RawMessage) bool {
	_, existed := o.values[key]
	if !existed {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
	return existed
}

func (o *orderedObject) remove(key string) bool {
	if _, ok := o.values[key]; !ok {
		return false
	}
	delete(o.values, key)
	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			break
		}
	}
	return true
}

func (o *orderedObject) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(o.values[k])
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func parseOrderedObject(data []byte) (*orderedObject, bool) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, false
	}
	o := &orderedObject{values: map[string]json.RawMessage{}}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, false
		}
		key, _ := tok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, false
		}
		o.set(key, raw) // JSON.parse: the last duplicate wins
	}
	return o, true
}

// editServers ports Pi's editMcpServers: read an mcp.json (empty when
// missing), let edit change its mcpServers, and write it back with its
// indentation when edit returns true. Other content is kept.
func editServers(path string, edit func(servers *orderedObject) bool) error {
	text, err := os.ReadFile(path)
	missing := errors.Is(err, os.ErrNotExist)
	if err != nil && !missing {
		return err
	}
	root := &orderedObject{values: map[string]json.RawMessage{}}
	if !missing {
		if !json.Valid(text) {
			var v any
			return json.Unmarshal(text, &v)
		}
		var ok bool
		if root, ok = parseOrderedObject(text); !ok {
			return fmt.Errorf("%s: expected an object with an \"mcpServers\" object", path)
		}
	}
	servers := &orderedObject{values: map[string]json.RawMessage{}}
	if raw, ok := root.values["mcpServers"]; ok {
		if servers, ok = parseOrderedObject(raw); !ok {
			return fmt.Errorf("%s: expected an object with an \"mcpServers\" object", path)
		}
	}
	if !edit(servers) {
		return nil
	}
	encoded, _ := servers.MarshalJSON()
	root.set("mcpServers", encoded)
	compact, _ := root.MarshalJSON()
	indent := "  "
	for _, line := range strings.Split(string(text), "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed != "" && trimmed != line {
			indent = line[:len(line)-len(trimmed)]
			break
		}
	}
	var out bytes.Buffer
	if err := json.Indent(&out, compact, "", indent); err != nil {
		return err
	}
	out.WriteByte('\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

func parsePairs(option string, pairs []string, errf func(string)) (*orderedObject, bool) {
	record := &orderedObject{values: map[string]json.RawMessage{}}
	for _, pair := range pairs {
		sep := strings.Index(pair, "=")
		if sep <= 0 {
			errf(fmt.Sprintf("--%s expects KEY=VALUE, got %q.", option, pair))
			return nil, false
		}
		v, _ := json.Marshal(pair[sep+1:])
		record.set(pair[:sep], v)
	}
	return record, true
}

func jsonValue(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func cliAdd(args []string, opts CLIOptions, logf, errf func(string)) int {
	usage := "Usage: gi mcp add <server> [options] (--url <url> | -- <command> [args...])\n" + cliHelpHint
	p := parseCLIOptions(args, map[string]string{
		"local": "flag", "url": "value", "env": "list", "cwd": "value", "header": "list",
		"bearer-token-env-var": "value", "oauth-client-id": "value", "oauth-client-secret": "value",
		"oauth-callback-port": "value", "oauth-client-name": "value", "exposure": "value", "description": "value",
	}, errf, 2)
	if p == nil {
		return 1
	}
	if len(p.positional) == 0 {
		errf(usage)
		return 1
	}
	name, command := p.positional[0], p.positional[1:]
	url, hasURL := p.values["url"]
	if hasURL == (len(command) > 0) {
		errf(usage)
		return 1
	}
	has := func(o string) bool { _, v := p.values[o]; _, l := p.lists[o]; return v || l }
	misplacedSet := []string{"env", "cwd"}
	where := "stdio servers"
	if !hasURL {
		misplacedSet = []string{"header", "bearer-token-env-var", "oauth-client-id", "oauth-client-secret", "oauth-callback-port", "oauth-client-name"}
		where = "HTTP servers (--url)"
	}
	for _, o := range misplacedSet {
		if has(o) {
			errf(fmt.Sprintf("--%s only applies to %s.", o, where))
			return 1
		}
	}
	cfg := &orderedObject{values: map[string]json.RawMessage{}}
	if hasURL {
		headers, ok := parsePairs("header", p.lists["header"], errf)
		if !ok {
			return 1
		}
		if bearer, ok := p.values["bearer-token-env-var"]; ok {
			headers.set("Authorization", jsonValue("Bearer ${"+bearer+"}"))
		}
		oauth := &orderedObject{values: map[string]json.RawMessage{}}
		if v, ok := p.values["oauth-client-id"]; ok {
			oauth.set("clientId", jsonValue(v))
		}
		if v, ok := p.values["oauth-client-secret"]; ok {
			oauth.set("clientSecret", jsonValue(v))
		}
		if v, ok := p.values["oauth-callback-port"]; ok {
			n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				oauth.set("callbackPort", json.RawMessage("null")) // Number("x") is NaN, serialized as null
			} else {
				oauth.set("callbackPort", jsonValue(n))
			}
		}
		if v, ok := p.values["oauth-client-name"]; ok {
			oauth.set("clientName", jsonValue(v))
		}
		cfg.set("url", jsonValue(url))
		if len(headers.keys) > 0 {
			b, _ := headers.MarshalJSON()
			cfg.set("headers", b)
		}
		if len(oauth.keys) > 0 {
			b, _ := oauth.MarshalJSON()
			cfg.set("oauth", b)
		}
	} else {
		env, ok := parsePairs("env", p.lists["env"], errf)
		if !ok {
			return 1
		}
		cfg.set("command", jsonValue(command[0]))
		if len(command) > 1 {
			cfg.set("args", jsonValue(command[1:]))
		}
		if len(env.keys) > 0 {
			b, _ := env.MarshalJSON()
			cfg.set("env", b)
		}
		if v, ok := p.values["cwd"]; ok {
			cfg.set("cwd", jsonValue(v))
		}
	}
	if v, ok := p.values["exposure"]; ok {
		cfg.set("exposure", jsonValue(v))
	}
	if v, ok := p.values["description"]; ok {
		cfg.set("description", jsonValue(v))
	}
	encoded, _ := cfg.MarshalJSON()
	if !serverNamePattern.MatchString(name) {
		errf(fmt.Sprintf("invalid server name %q (use letters, digits, \"_\" and \"-\")", name))
		return 1
	}
	if _, err := parseServer(name, encoded, ""); err != nil {
		errf(fmt.Sprintf("server %q: %v", name, err))
		return 1
	}
	project := p.flags["local"]
	path, scope := opts.UserPath, "global"
	if project {
		path, scope = opts.ProjectPath, "project"
	}
	replaced := false
	if err := editServers(path, func(servers *orderedObject) bool {
		replaced = servers.set(name, encoded)
		return true
	}); err != nil {
		errf(fmt.Sprintf("Could not update %s: %v", path, err))
		return 1
	}
	verb := "Added"
	if replaced {
		verb = "Replaced"
	}
	logf(fmt.Sprintf("%s %s MCP server %q in %s.", verb, scope, name, path))
	if project {
		logf(fmt.Sprintf("gi does not read project MCP configuration until it has project trust, so %s is ignored for now.", path))
	}
	mayNeedSignIn := hasURL
	for _, k := range headerKeys(cfg) {
		if strings.EqualFold(k, "authorization") {
			mayNeedSignIn = false
		}
	}
	hint := ""
	if mayNeedSignIn {
		hint = ". If it requires sign-in: gi mcp login " + name
	}
	logf("Check it with: gi mcp list" + hint)
	return 0
}

func headerKeys(cfg *orderedObject) []string {
	raw, ok := cfg.values["headers"]
	if !ok {
		return nil
	}
	h, ok := parseOrderedObject(raw)
	if !ok {
		return nil
	}
	return h.keys
}

func cliRemove(args []string, opts CLIOptions, logf, errf func(string)) int {
	p := parseCLIOptions(args, map[string]string{"local": "flag"}, errf, -1)
	if p == nil {
		return 1
	}
	if len(p.positional) != 1 {
		errf("Usage: gi mcp remove <server> [-l]\n" + cliHelpHint)
		return 1
	}
	name := p.positional[0]
	project := p.flags["local"]
	path, scope := opts.UserPath, "global"
	if project {
		path, scope = opts.ProjectPath, "project"
	}
	removed := false
	if _, err := os.Stat(path); err == nil {
		if err := editServers(path, func(servers *orderedObject) bool {
			removed = servers.remove(name)
			return removed
		}); err != nil {
			errf(fmt.Sprintf("Could not update %s: %v", path, err))
			return 1
		}
	}
	if removed {
		logf(fmt.Sprintf("Removed %s MCP server %q from %s.", scope, name, path))
		return 0
	}
	other := ""
	otherPath, otherHint := opts.ProjectPath, "; use --local"
	if project {
		otherPath, otherHint = opts.UserPath, "; omit --local"
	}
	if s, ok := LoadConfig(otherPath, "", false).Servers[name]; ok {
		other = fmt.Sprintf(" It is defined in %s%s.", s.Source, otherHint)
	}
	errf(fmt.Sprintf("No %s MCP server named %q in %s.%s", scope, name, path, other))
	return 1
}

type listReport struct {
	Name              string            `json:"name"`
	Scope             string            `json:"scope"`
	Source            string            `json:"source"`
	Enabled           bool              `json:"enabled"`
	Exposure          string            `json:"exposure"`
	Transport         string            `json:"transport"`
	State             string            `json:"state"`
	Tools             []string          `json:"tools"`
	ToolExposure      map[string]string `json:"toolExposure,omitempty"`
	Resources         *int              `json:"resources,omitempty"`
	ResourceTemplates *int              `json:"resourceTemplates,omitempty"`
	Error             string            `json:"error,omitempty"`
}

func describeTransport(s ServerConfig) string {
	if s.Transport == "http" {
		return s.URL
	}
	return strings.Join(append([]string{s.Command}, s.Args...), " ")
}

// cliList connects to every enabled server and prints its state, tools and
// errors; it fails when an entry is invalid or a server did not connect.
func cliList(cfg Config, asJSON bool, untrustedNote string, opts CLIOptions, logf func(string)) int {
	m := NewManager(cfg, opts.Cwd, opts.LogPath)
	if opts.Credentials != nil {
		m.SetCredentials(opts.Credentials)
	} else if opts.CredentialsPath != "" {
		m.SetCredentials(NewCredentialStore(opts.CredentialsPath))
	}
	defer m.Close()
	names := cfg.Names()
	reports := make([]listReport, len(names))
	var wg sync.WaitGroup
	for i, name := range names {
		s := cfg.Servers[name]
		reports[i] = listReport{Name: name, Scope: "global", Source: s.Source, Enabled: s.Enabled, Exposure: s.Exposure,
			Transport: describeTransport(s), State: StateDisabled, Tools: []string{}}
		if !s.Enabled {
			continue
		}
		wg.Add(1)
		go func(r *listReport, s ServerConfig) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), s.Timeout+5*time.Second)
			defer cancel()
			tools, _ := m.Tools(ctx, r.Name)
			for _, t := range tools {
				r.Tools = append(r.Tools, t.Name)
				if exposure := s.ToolExposureFor(t.Name); exposure != r.Exposure {
					if r.ToolExposure == nil {
						r.ToolExposure = map[string]string{}
					}
					r.ToolExposure[t.Name] = exposure
				}
			}
			if m.HasResources(r.Name) {
				resources, templates := 0, 0
				if res, err := m.ListResources(ctx, r.Name, ""); err == nil {
					resources = len(res.Resources)
				}
				if res, err := m.ListResourceTemplates(ctx, r.Name, ""); err == nil {
					templates = len(res.ResourceTemplates)
				}
				r.Resources, r.ResourceTemplates = &resources, &templates
			}
		}(&reports[i], s)
	}
	wg.Wait()
	for _, st := range m.Status() {
		for i := range reports {
			if reports[i].Name == st.Name && reports[i].Enabled {
				reports[i].State = st.State
				if st.State != StateConnected {
					reports[i].Error = st.Error
				}
			}
		}
	}
	errs := make([]string, len(cfg.Errors))
	for i, err := range cfg.Errors {
		errs[i] = err.Error()
	}
	failed := len(errs) > 0
	for _, r := range reports {
		if r.Enabled && r.State != StateConnected {
			failed = true
		}
	}
	code := 0
	if failed {
		code = 1
	}
	if asJSON {
		out := struct {
			Servers []listReport `json:"servers"`
			Errors  []string     `json:"errors"`
			Note    string       `json:"note,omitempty"`
		}{reports, errs, untrustedNote}
		b, _ := json.MarshalIndent(out, "", "  ")
		logf(string(b))
		return code
	}
	if len(reports) == 0 && len(errs) == 0 {
		logf(fmt.Sprintf("No MCP servers configured. Add them to %s.", opts.UserPath))
	}
	for _, r := range reports {
		state := r.State
		if r.State == StateConnected {
			plural := "s"
			if len(r.Tools) == 1 {
				plural = ""
			}
			state = fmt.Sprintf("connected, %d tool%s", len(r.Tools), plural)
		} else if r.State == StateNeedsAuth {
			state = "needs sign-in"
		}
		logf(fmt.Sprintf("%s: %s (%s, %s)", r.Name, state, r.Exposure, r.Scope))
		logf("  " + r.Transport)
		if r.State == StateNeedsAuth {
			logf("  sign in with: gi mcp login " + r.Name)
		}
		if len(r.Tools) > 0 {
			tools := make([]string, len(r.Tools))
			for i, t := range r.Tools {
				if e, ok := r.ToolExposure[t]; ok {
					tools[i] = t + " [" + e + "]"
				} else {
					tools[i] = t
				}
			}
			logf("  tools: " + strings.Join(tools, ", "))
		}
		if r.Resources != nil {
			logf(fmt.Sprintf("  resources: %d, URI templates: %d", *r.Resources, *r.ResourceTemplates))
		}
		if r.Error != "" {
			logf("  " + strings.ReplaceAll(r.Error, "\n", "\n  "))
		}
	}
	for _, e := range errs {
		logf("config error: " + e)
	}
	if untrustedNote != "" {
		logf(untrustedNote)
	}
	return code
}
