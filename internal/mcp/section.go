package mcp

import (
	"fmt"
	"sort"
	"strings"
)

// ServersSection is the name of the system-prompt section listing servers
// whose tools are not declared to the model (Pi's mcp_servers).
const ServersSection = "mcp_servers"

const (
	maxServerDescriptionChars = 250
	maxServersSectionChars    = 4096
	serversSectionIntro       = "MCP servers whose tools are not declared to you. Call the tools of `codemode` servers from codemode scripts: find them with `searchTools(query, { namespace })` and read a server's instructions and tool names with `describeNamespace(name)`. Load the tools of `tool_search` servers with `tool_search`."
)

// configuredExposures is the set of exposures a server's config can give its
// tools: the server exposure plus every toolExposure value.
func (s ServerConfig) configuredExposures() map[string]bool {
	out := map[string]bool{s.Exposure: true}
	for _, v := range s.ToolExposure {
		if e, err := normalizeExposure(v, s.Exposure); err == nil {
			out[e] = true
		}
	}
	return out
}

// HasIndirectTools reports whether the config can give the server codemode or
// deferred tools.
func (s ServerConfig) HasIndirectTools() bool {
	e := s.configuredExposures()
	return e[ExposureCodemode] || e[ExposureDeferred]
}

// HasCodemodeTools reports whether the config can give the server codemode
// tools.
func (s ServerConfig) HasCodemodeTools() bool { return s.configuredExposures()[ExposureCodemode] }

// HasDeferredTools reports whether the config can give the server deferred
// tools.
func (s ServerConfig) HasDeferredTools() bool { return s.configuredExposures()[ExposureDeferred] }

// HasDirectTools reports whether the config can give the server direct tools.
func (s ServerConfig) HasDirectTools() bool { return s.configuredExposures()[ExposureDirect] }

// SectionServer is one server as seen by the section renderer.
type SectionServer struct {
	Config       ServerConfig
	Instructions string // initialize instructions, once connected
}

func truncateChars(text string, max int) string {
	r := []rune(text)
	if len(r) <= max {
		return text
	}
	if max <= 1 {
		return ""
	}
	return strings.TrimRight(string(r[:max-1]), " \t\n") + "…"
}

func serverSummary(s SectionServer) string {
	text := s.Config.Description
	if text == "" {
		text = s.Instructions
	}
	return strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])
}

// RenderServersSection ports Pi's renderServersSection: enabled servers with
// codemode or deferred tools, how their tools are reached and a one-line
// summary, within 4096 characters (descriptions shrink; the last servers are
// counted in a closing line when even bare lines do not fit). Empty when no
// server qualifies.
func RenderServersSection(servers []SectionServer) string {
	var listed []SectionServer
	for _, s := range servers {
		if s.Config.Enabled && s.Config.HasIndirectTools() {
			listed = append(listed, s)
		}
	}
	if len(listed) == 0 {
		return ""
	}
	sort.Slice(listed, func(i, j int) bool { return listed[i].Config.Name < listed[j].Config.Name })
	heads := make([]string, len(listed))
	for i, s := range listed {
		reach := "tool_search"
		if s.Config.configuredExposures()[ExposureCodemode] {
			reach = "codemode"
		}
		heads[i] = fmt.Sprintf("- %s (%s)", Namespace(s.Config.Name), reach)
	}
	omitted := func(count int) []string {
		if count <= 0 {
			return nil
		}
		plural := "s"
		if count == 1 {
			plural = ""
		}
		return []string{fmt.Sprintf("- … %d more server%s; find their tools with searchTools()", count, plural)}
	}
	size := func(kept int) int {
		lines := append([]string{serversSectionIntro}, heads[:kept]...)
		lines = append(lines, omitted(len(listed)-kept)...)
		return len([]rune(strings.Join(lines, "\n")))
	}
	kept := len(listed)
	for kept > 0 && size(kept) > maxServersSectionChars {
		kept--
	}
	perServer := 0
	if kept > 0 {
		perServer = min(maxServerDescriptionChars, (maxServersSectionChars-size(kept))/kept-2)
	}
	lines := []string{serversSectionIntro}
	for i := 0; i < kept; i++ {
		summary := ""
		if perServer > 0 {
			summary = truncateChars(serverSummary(listed[i]), perServer)
		}
		if summary != "" {
			lines = append(lines, heads[i]+": "+summary)
		} else {
			lines = append(lines, heads[i])
		}
	}
	lines = append(lines, omitted(len(listed)-kept)...)
	return strings.Join(lines, "\n")
}
