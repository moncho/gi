// Package mcp is gi's Model Context Protocol client, compatible with Pi's
// MCP support (docs/mcp.md): the same mcp.json format and locations, the
// same validation and value expansion, stdio and streamable-HTTP transports,
// and Pi's server lifecycle. Tool exposure, tool_search and codemode build on
// it; see docs/internal/mcp-codemode-plan.md.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rcarmo/gi/internal/config"
)

// Exposure values, as in Pi. "codemode-deferred" is an alias of codemode.
const (
	ExposureCodemode = "codemode"
	ExposureDeferred = "deferred"
	ExposureDirect   = "direct"
	ExposureHidden   = "hidden"
)

// DefaultTimeout is Pi's default per-request timeout.
const DefaultTimeout = 60 * time.Second

// ServerConfig is one mcpServers entry, validated but not yet expanded:
// ${VAR} and !command values are resolved only when connecting.
type ServerConfig struct {
	Name         string            `json:"-"`
	Transport    string            `json:"-"` // "stdio" or "http"
	Command      string            `json:"command,omitempty"`
	Args         []string          `json:"args,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	Cwd          string            `json:"cwd,omitempty"`
	URL          string            `json:"url,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	OAuth        json.RawMessage   `json:"oauth,omitempty"`
	Timeout      time.Duration     `json:"-"`
	Enabled      bool              `json:"-"`
	Description  string            `json:"description,omitempty"`
	Exposure     string            `json:"-"`
	ToolExposure map[string]string `json:"toolExposure,omitempty"`
	Source       string            `json:"-"` // file that defined the entry
}

// Config is the merged user and (trusted) project configuration.
type Config struct {
	Servers            map[string]ServerConfig
	AutoEnableCodemode bool
	// Errors lists invalid entries; they are skipped, never fatal (Pi).
	Errors []error
}

// Names returns server names in sorted order.
func (c Config) Names() []string {
	names := make([]string, 0, len(c.Servers))
	for name := range c.Servers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type rawFile struct {
	MCPServers         map[string]json.RawMessage `json:"mcpServers"`
	AutoEnableCodemode *bool                      `json:"autoEnableCodemode"`
}

type rawServer struct {
	Type         string            `json:"type"`
	Command      string            `json:"command"`
	Args         []string          `json:"args"`
	Env          map[string]string `json:"env"`
	Cwd          string            `json:"cwd"`
	URL          string            `json:"url"`
	Headers      map[string]string `json:"headers"`
	OAuth        json.RawMessage   `json:"oauth"`
	Timeout      *float64          `json:"timeout"`
	Enabled      *bool             `json:"enabled"`
	Description  string            `json:"description"`
	Exposure     string            `json:"exposure"`
	ToolExposure map[string]string `json:"toolExposure"`
}

var serverNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// UserConfigPath is the user-level MCP configuration: ~/.gi/agent/mcp.json if
// it exists, else Pi's ~/.pi/agent/mcp.json.
func UserConfigPath() string { return config.UserConfigFile("mcp.json") }

// ProjectConfigPath is the project MCP configuration: .gi/mcp.json if it
// exists, else Pi's .pi/mcp.json.
func ProjectConfigPath(workspace string) string { return config.ProjectConfigFile(workspace, "mcp.json") }

// LoadConfig reads the user configuration and, only when projectTrusted, the
// project one; a project entry replaces a user entry with the same name and a
// project autoEnableCodemode overrides the user value (Pi). gi has no project
// trust model yet, so callers pass false until one exists (issue #16).
func LoadConfig(userPath, projectPath string, projectTrusted bool) Config {
	cfg := Config{Servers: map[string]ServerConfig{}, AutoEnableCodemode: true}
	sources := []string{userPath}
	if projectTrusted && projectPath != "" {
		sources = append(sources, projectPath)
	}
	for _, path := range sources {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				cfg.Errors = append(cfg.Errors, fmt.Errorf("%s: %w", path, err))
			}
			continue
		}
		var file rawFile
		if err := json.Unmarshal(data, &file); err != nil {
			cfg.Errors = append(cfg.Errors, fmt.Errorf("%s: invalid JSON: %w", path, err))
			continue
		}
		if file.AutoEnableCodemode != nil {
			cfg.AutoEnableCodemode = *file.AutoEnableCodemode
		}
		names := make([]string, 0, len(file.MCPServers))
		for name := range file.MCPServers {
			names = append(names, name)
		}
		sort.Strings(names)
		seen := map[string]string{} // canonical name -> name, within this file
		for _, name := range names {
			server, err := parseServer(name, file.MCPServers[name], path)
			if err != nil {
				cfg.Errors = append(cfg.Errors, fmt.Errorf("%s: server %q: %w", path, name, err))
				continue
			}
			canon := canonicalName(name)
			if other, dup := seen[canon]; dup {
				cfg.Errors = append(cfg.Errors, fmt.Errorf("%s: server %q duplicates %q (names differing only in - and _ are the same server)", path, name, other))
				continue
			}
			seen[canon] = name
			// A later file (the project) replaces an earlier entry with the
			// same canonical name.
			for existing := range cfg.Servers {
				if canonicalName(existing) == canon {
					delete(cfg.Servers, existing)
				}
			}
			cfg.Servers[name] = server
		}
	}
	return cfg
}

// canonicalName treats names that differ only in - and _ as the same server.
func canonicalName(name string) string { return strings.ReplaceAll(name, "-", "_") }

func parseServer(name string, raw json.RawMessage, source string) (ServerConfig, error) {
	if !serverNamePattern.MatchString(name) {
		return ServerConfig{}, fmt.Errorf("name may contain only letters, digits, _ and -")
	}
	var r rawServer
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&r); err != nil {
		return ServerConfig{}, fmt.Errorf("invalid entry: %w", err)
	}
	s := ServerConfig{Name: name, Command: r.Command, Args: r.Args, Env: r.Env, Cwd: r.Cwd, URL: r.URL,
		Headers: r.Headers, OAuth: r.OAuth, Description: strings.TrimSpace(r.Description), ToolExposure: r.ToolExposure,
		Timeout: DefaultTimeout, Enabled: true, Source: source}
	switch strings.ToLower(strings.TrimSpace(r.Type)) {
	case "":
	case "stdio":
		s.Transport = "stdio"
	case "http", "streamable-http":
		s.Transport = "http"
	case "sse":
		return ServerConfig{}, fmt.Errorf("the legacy SSE transport is not supported; use the server's streamable HTTP endpoint (often /mcp)")
	default:
		return ServerConfig{}, fmt.Errorf("type must be stdio, http or streamable-http")
	}
	hasCommand, hasURL := strings.TrimSpace(r.Command) != "", strings.TrimSpace(r.URL) != ""
	switch {
	case hasCommand && hasURL:
		return ServerConfig{}, fmt.Errorf("use either command or url, not both")
	case !hasCommand && !hasURL:
		return ServerConfig{}, fmt.Errorf("command or url is required")
	case hasCommand:
		if s.Transport == "http" {
			return ServerConfig{}, fmt.Errorf("type %q needs a url", r.Type)
		}
		s.Transport = "stdio"
		if len(r.Headers) > 0 || len(r.OAuth) > 0 {
			return ServerConfig{}, fmt.Errorf("headers and oauth apply only to url servers")
		}
	default:
		if s.Transport == "stdio" {
			return ServerConfig{}, fmt.Errorf("type stdio needs a command")
		}
		s.Transport = "http"
		if !strings.HasPrefix(r.URL, "http://") && !strings.HasPrefix(r.URL, "https://") {
			return ServerConfig{}, fmt.Errorf("url must be http:// or https://")
		}
		if len(r.Args) > 0 || len(r.Env) > 0 || r.Cwd != "" {
			return ServerConfig{}, fmt.Errorf("args, env and cwd apply only to command servers")
		}
	}
	if r.Timeout != nil {
		if *r.Timeout <= 0 {
			return ServerConfig{}, fmt.Errorf("timeout must be a positive number of seconds")
		}
		s.Timeout = time.Duration(*r.Timeout * float64(time.Second))
	}
	if r.Enabled != nil {
		s.Enabled = *r.Enabled
	}
	exposure, err := normalizeExposure(r.Exposure, ExposureCodemode)
	if err != nil {
		return ServerConfig{}, err
	}
	s.Exposure = exposure
	for pattern, value := range r.ToolExposure {
		if strings.TrimSpace(pattern) == "" {
			return ServerConfig{}, fmt.Errorf("toolExposure keys must be tool names or * patterns")
		}
		if _, err := normalizeExposure(value, ""); err != nil {
			return ServerConfig{}, fmt.Errorf("toolExposure %q: %w", pattern, err)
		}
	}
	return s, nil
}

func normalizeExposure(value, fallback string) (string, error) {
	switch v := strings.TrimSpace(value); v {
	case "":
		if fallback == "" {
			return "", fmt.Errorf("exposure must be codemode, deferred, direct or hidden")
		}
		return fallback, nil
	case ExposureCodemode, "codemode-deferred":
		return ExposureCodemode, nil
	case ExposureDeferred, ExposureDirect, ExposureHidden:
		return v, nil
	default:
		return "", fmt.Errorf("exposure must be codemode, deferred, direct or hidden, not %q", v)
	}
}

// ToolExposureFor resolves a tool's exposure: an exact toolExposure name wins,
// then the first matching * pattern (in key order), then the server exposure.
func (s ServerConfig) ToolExposureFor(tool string) string {
	if v, ok := s.ToolExposure[tool]; ok {
		e, _ := normalizeExposure(v, s.Exposure)
		return e
	}
	patterns := make([]string, 0, len(s.ToolExposure))
	for p := range s.ToolExposure {
		if strings.Contains(p, "*") {
			patterns = append(patterns, p)
		}
	}
	sort.Strings(patterns) // JSON object order is not preserved by Go maps
	for _, p := range patterns {
		if globMatch(p, tool) {
			e, _ := normalizeExposure(s.ToolExposure[p], s.Exposure)
			return e
		}
	}
	return s.Exposure
}

func globMatch(pattern, name string) bool {
	re := "^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, ".*") + "$"
	ok, _ := regexp.MatchString(re, name)
	return ok
}

var envReference = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// commandTimeout bounds !command value expansion.
const commandTimeout = 10 * time.Second

// expandValue resolves Pi's value syntax for env and headers: a whole-value
// "!command" runs the command and uses its trimmed stdout; otherwise
// ${VAR} references are replaced from the environment (unset: empty).
func expandValue(ctx context.Context, value string, lookup func(string) (string, bool)) (string, error) {
	if strings.HasPrefix(value, "!") {
		ctx, cancel := context.WithTimeout(ctx, commandTimeout)
		defer cancel()
		out, err := exec.CommandContext(ctx, "sh", "-c", strings.TrimPrefix(value, "!")).Output()
		if err != nil {
			return "", fmt.Errorf("value command failed: %w", err)
		}
		return strings.TrimSpace(string(out)), nil
	}
	return envReference.ReplaceAllStringFunc(value, func(ref string) string {
		v, _ := lookup(envReference.FindStringSubmatch(ref)[1])
		return v
	}), nil
}

// expandHome turns a leading "~/" into the home directory (command, args, cwd).
func expandHome(value string) string {
	if strings.HasPrefix(value, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, value[2:])
		}
	}
	return value
}
