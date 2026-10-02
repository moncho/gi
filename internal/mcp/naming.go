package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// maxToolNameLength is the provider tool-name limit Pi respects.
const maxToolNameLength = 64

var nonIdentifier = regexp.MustCompile(`[^A-Za-z0-9_]`)

// Namespace is a server's tool namespace: mcp__<server> with - replaced by _
// (Pi's mcpNamespace).
func Namespace(server string) string { return "mcp__" + strings.ReplaceAll(server, "-", "_") }

// ToolName ports Pi's createMcpToolName: mcp__<server>__<tool> with every
// character outside [A-Za-z0-9_] replaced by _, shortened with an 8-hex
// SHA-256 suffix of "server\x00tool" when longer than 64 characters or when
// isTaken reports the plain name as used by another tool.
func ToolName(server, tool string, isTaken func(string) bool) string {
	name := nonIdentifier.ReplaceAllString("mcp__"+server+"__"+tool, "_")
	if len(name) <= maxToolNameLength && (isTaken == nil || !isTaken(name)) {
		return name
	}
	sum := sha256.Sum256([]byte(server + "\x00" + tool))
	hash := hex.EncodeToString(sum[:])[:8]
	return name[:min(len(name), maxToolNameLength-len(hash)-1)] + "_" + hash // JS slice clamps
}

// ServerToolNames assigns model-facing names to one server's tools the way Pi
// does: every tool whose plain name collides with another of the server's
// tools gets the hash suffix (so the result does not depend on list order),
// as does any name owned by a different server (taken).
func ServerToolNames(server string, tools []string, taken func(string) bool) map[string]string {
	plainCount := map[string]int{}
	seen := map[string]bool{}
	for _, tool := range tools {
		if seen[tool] {
			continue
		}
		seen[tool] = true
		plainCount[ToolName(server, tool, nil)]++
	}
	out := map[string]string{}
	used := map[string]bool{}
	for _, tool := range tools {
		if _, done := out[tool]; done {
			continue
		}
		name := ToolName(server, tool, func(candidate string) bool {
			return (taken != nil && taken(candidate)) || used[candidate] || plainCount[candidate] > 1
		})
		used[name] = true
		out[tool] = name
	}
	return out
}
