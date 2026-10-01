package tools

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// Tool discovery ported from Pi's tool-search extension: a BM25 ranker over
// tool metadata, shared by tool_search and (later) codemode's searchTools().

// ToolSearchName is the tool_search tool name.
const ToolSearchName = "tool_search"

// DefaultToolSearchLimit is Pi's default number of results.
const DefaultToolSearchLimit = 8

// ToolSearchDescription is Pi's tool_search description. It does not list
// the searchable tools, so it stays the same while servers connect.
const ToolSearchDescription = "# Tool discovery\n\nSearches over deferred tool metadata with BM25 and exposes matching tools for the next model call.\n\nSome of the tools, such as tools of MCP servers, may not have been provided to you upfront, and you should use this tool (`tool_search`) to search for the required tools. For MCP tool discovery, always use `tool_search`."

// ToolSearchParameters is Pi's tool_search schema.
const ToolSearchParameters = `{"type":"object","properties":{"query":{"type":"string","description":"Search query for deferred tools."},"limit":{"type":"number","description":"Maximum number of tools to return. Defaults to 8."}},"required":["query"]}`

var searchStopWords = map[string]bool{"a": true, "an": true, "and": true, "are": true, "as": true, "at": true, "be": true, "by": true, "for": true,
	"from": true, "in": true, "is": true, "it": true, "of": true, "on": true, "or": true, "that": true, "the": true, "this": true, "to": true, "with": true}

var (
	camelLower  = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	camelUpper  = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
	nonAlnum    = regexp.MustCompile(`[^a-z0-9]+`)
	esPluralEnd = regexp.MustCompile(`(ches|shes|sses|xes|zes)$`)
)

// stem is Pi's naive singular form.
func stem(term string) string {
	switch {
	case len(term) > 4 && strings.HasSuffix(term, "ies"):
		return term[:len(term)-3] + "y"
	case len(term) > 4 && esPluralEnd.MatchString(term):
		return term[:len(term)-2]
	case len(term) > 3 && strings.HasSuffix(term, "s") && !strings.HasSuffix(term, "ss"):
		return term[:len(term)-1]
	}
	return term
}

// Tokenize ports Pi's tokenize: lowercase terms split at camelCase
// boundaries and non-alphanumerics, without stop words, stemmed.
func Tokenize(text string) []string {
	text = camelLower.ReplaceAllString(text, "$1 $2")
	text = camelUpper.ReplaceAllString(text, "$1 $2")
	var out []string
	for _, term := range nonAlnum.Split(strings.ToLower(text), -1) {
		if term != "" && !searchStopWords[term] {
			out = append(out, stem(term))
		}
	}
	return out
}

// SearchDocument is the search text of one tool.
type SearchDocument struct {
	Name string
	Text string
}

// SearchNamespace describes a tool's namespace (an MCP server).
type SearchNamespace struct {
	Name, Description, Instructions string
}

// NewSearchDocument ports Pi's createToolSearchDocument: name, name with _
// as spaces, description, schema descriptions and property names, and the
// namespace with its description and instructions.
func NewSearchDocument(name, description string, parameters any, ns *SearchNamespace) SearchDocument {
	parts := []string{name, strings.ReplaceAll(name, "_", " "), description}
	schemaText(parameters, &parts)
	if ns != nil {
		parts = append(parts, ns.Name, ns.Description, ns.Instructions)
	}
	kept := parts[:0]
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			kept = append(kept, p)
		}
	}
	return SearchDocument{Name: name, Text: strings.Join(kept, " ")}
}

func schemaText(schema any, parts *[]string) {
	m, ok := schema.(map[string]any)
	if !ok {
		return
	}
	if d, ok := m["description"].(string); ok {
		*parts = append(*parts, d)
	}
	if props, ok := m["properties"].(map[string]any); ok {
		names := make([]string, 0, len(props))
		for name := range props {
			names = append(names, name)
		}
		sort.Strings(names) // JSON object order is not kept; BM25 is order-independent
		for _, name := range names {
			*parts = append(*parts, name)
			schemaText(props[name], parts)
		}
	}
	schemaText(m["items"], parts)
	for _, key := range []string{"anyOf", "oneOf", "allOf"} {
		if variants, ok := m[key].([]any); ok {
			for _, v := range variants {
				schemaText(v, parts)
			}
		}
	}
}

// SearchMatch is a ranked tool.
type SearchMatch struct {
	Name  string
	Score float64
}

// RankBM25 ports Pi's Bm25Ranker (k1 = 1.2, b = 0.75); ties keep document
// order.
func RankBM25(query string, docs []SearchDocument, limit int) []SearchMatch {
	seen := map[string]bool{}
	var terms []string
	for _, t := range Tokenize(query) {
		if !seen[t] {
			seen[t] = true
			terms = append(terms, t)
		}
	}
	if len(terms) == 0 || len(docs) == 0 || limit <= 0 {
		return nil
	}
	const k1, b = 1.2, 0.75
	counts := make([]map[string]int, len(docs))
	lengths := make([]int, len(docs))
	total := 0
	for i, d := range docs {
		counts[i] = map[string]int{}
		for _, t := range Tokenize(d.Text) {
			counts[i][t]++
			lengths[i]++
		}
		total += lengths[i]
	}
	avg := float64(total) / float64(len(docs))
	if avg == 0 {
		avg = 1
	}
	idf := map[string]float64{}
	for _, t := range terms {
		freq := 0
		for _, c := range counts {
			if c[t] > 0 {
				freq++
			}
		}
		idf[t] = math.Log(1 + (float64(len(docs)-freq)+0.5)/(float64(freq)+0.5))
	}
	var matches []SearchMatch
	for i, d := range docs {
		score := 0.0
		for _, t := range terms {
			c := counts[i][t]
			if c == 0 {
				continue
			}
			norm := k1 * (1 - b + b*float64(lengths[i])/avg)
			score += idf[t] * (float64(c) * (k1 + 1)) / (float64(c) + norm)
		}
		if score > 0 {
			matches = append(matches, SearchMatch{Name: d.Name, Score: score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].Score > matches[j].Score })
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}
