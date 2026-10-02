package codemode

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

//go:embed vendor/declarations.js
var declarationsSource string

// Declaration is what a script sees of a tool (Pi's CodemodeToolDeclaration).
type Declaration struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"inputSchema,omitempty"`
	OutputSchema json.RawMessage `json:"outputSchema,omitempty"`
}

// TextOutputSchema is the output schema of tools that resolve to text.
var TextOutputSchema = json.RawMessage(`{"type":"string"}`)

// MCPResultSchema ports Pi's createMcpResultSchema: the CallToolResult scripts
// receive from MCP tools, with the tool's own output schema as
// structuredContent.
func MCPResultSchema(structured json.RawMessage) json.RawMessage {
	props := `"content":{"type":"array","items":{"type":"object"}},`
	if len(structured) > 0 && string(structured) != "null" {
		props += `"structuredContent":` + string(structured) + `,`
	}
	props += `"isError":{"type":"boolean"},"_meta":{"type":"object"}`
	return json.RawMessage(`{"type":"object","properties":{` + props + `},"required":["content"]}`)
}

// Rendered holds Pi-rendered declarations.
type Rendered struct {
	Samples    map[string]string // tool name -> renderToolSample
	MCPResult  map[string]bool   // output schema is an MCP CallToolResult
	Outputs    map[string]string // what a script call resolves to (Pi's describeOutput)
	MCPPrelude string            // MCP_TYPESCRIPT_PREAMBLE
}

// describeOutputJS is Pi's describeOutput (codemode/tool.js), run next to
// the vendored declarations.
const describeOutputJS = `function __describeOutput(schema) {
    const type = __decl.renderToolOutputType(schema);
    if (type === "string")
        return "a string";
    const object = typeof schema === "object" ? schema : undefined;
    const properties = object?.properties;
    if (object?.type === "object" &&
        typeof properties === "object" &&
        properties !== null &&
        __decl.mcpStructuredContentSchema(schema) === undefined) {
        const required = new Set(Array.isArray(object.required) ? object.required : []);
        const fields = Object.keys(properties).map((name) => (required.has(name) ? name : ` + "`${name}?`" + `));
        return ` + "`\\`{ ${fields.join(\", \")} }\\``" + `;
    }
    return ` + "`\\`${type.replace(/\\s+/g, \" \")}\\``" + `;
}
`

// ScriptCallDescription ports Pi's describeScriptCall: a declared tool's
// description followed by how scripts call it and what the call resolves
// to (codemode mode "on").
func ScriptCallDescription(d Declaration, r Rendered) string {
	return strings.TrimSpace(d.Description) + "\n\nCodemode: `tools." + Identifier(d.Name) + "(args)` resolves to " + r.Outputs[d.Name] + "."
}

// RenderDeclarations renders tool samples with Pi's own declarations.js,
// evaluated in QuickJS, so model-facing declarations match Pi exactly.
func (e *Engine) RenderDeclarations(ctx context.Context, decls []Declaration) (Rendered, error) {
	type in struct {
		Name         string          `json:"name"`
		Description  string          `json:"description"`
		InputSchema  json.RawMessage `json:"inputSchema"`
		OutputSchema json.RawMessage `json:"outputSchema"`
	}
	items := make([]in, len(decls))
	for i, d := range decls {
		items[i] = in{d.Name, d.Description, nullIfEmpty(d.InputSchema), nullIfEmpty(d.OutputSchema)}
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return Rendered{}, err
	}
	code := declarationsSource + "\n" + describeOutputJS + "const __in = " + string(raw) + ";\n" +
		"return { samples: __in.map((d) => __decl.renderToolSample({ name: d.name, description: d.description, inputSchema: d.inputSchema ?? undefined, outputSchema: d.outputSchema ?? undefined })), " +
		"mcp: __in.map((d) => __decl.mcpStructuredContentSchema(d.outputSchema ?? undefined) !== undefined), " +
		"outputs: __in.map((d) => __describeOutput(d.outputSchema ?? undefined)), preamble: __decl.MCP_TYPESCRIPT_PREAMBLE };"
	res := e.Execute(ctx, code, Options{})
	if !res.OK {
		return Rendered{}, fmt.Errorf("render declarations: %s", res.Error.Message)
	}
	var out struct {
		Samples  []string `json:"samples"`
		MCP      []bool   `json:"mcp"`
		Outputs  []string `json:"outputs"`
		Preamble string   `json:"preamble"`
	}
	if err := json.Unmarshal(res.Value, &out); err != nil || len(out.Samples) != len(decls) || len(out.Outputs) != len(decls) {
		return Rendered{}, fmt.Errorf("render declarations: unexpected result")
	}
	r := Rendered{Samples: map[string]string{}, MCPResult: map[string]bool{}, Outputs: map[string]string{}, MCPPrelude: out.Preamble}
	for i, d := range decls {
		r.Samples[d.Name] = out.Samples[i]
		r.MCPResult[d.Name] = out.MCP[i]
		r.Outputs[d.Name] = out.Outputs[i]
	}
	return r, nil
}

func nullIfEmpty(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("null")
	}
	return raw
}

//go:embed vendor/codemode-texts.json
var textsJSON []byte

// ReferencePath is gi's codemode script reference (adapted from Pi's
// docs/codemode.md), shipped as docs/internal/codemode-scripts.md in the
// read-only vfs://reference tree. The models line points the model there.
const ReferencePath = "vfs://reference/codemode-scripts.md"

// Texts are Pi's model-facing codemode texts, produced by Pi's codemode
// extension (scripts/vendor-codemode.mjs): the description without tools
// (intro and globals; no models API), the code parameter text, and the
// system-prompt snippet and guidelines.
var Texts = func() (t struct {
	BaseDescription string `json:"baseDescription"`
	// BaseDescriptionModels includes the models API line, with
	// {{CODEMODE_DOCS}} where Pi names its codemode reference.
	BaseDescriptionModels string   `json:"baseDescriptionModels"`
	CodeDescription       string   `json:"codeDescription"`
	PromptSnippet         string   `json:"promptSnippet"`
	PromptGuidelines      []string `json:"promptGuidelines"`
}) {
	if err := json.Unmarshal(textsJSON, &t); err != nil {
		panic("codemode: vendored texts: " + err.Error())
	}
	return t
}()

// DefaultInlineBudget is Pi's default codemode.inlineBudget (estimated tokens).
const DefaultInlineBudget = 3000

// Namespace of a nested tool (an MCP server).
type Namespace struct {
	Name, Description, Instructions string
}

// DescriptionOptions control Description.
type DescriptionOptions struct {
	Namespaces   map[string]Namespace // tool name -> namespace
	Deferred     map[string]bool      // tools never listed
	InlineBudget *int                 // nil: no limit
	// Models adds the models API line, pointing at DocsPath (Pi's
	// createCodemodeDescription models option).
	Models   bool
	DocsPath string
}

type catalogEntry struct {
	name, section string
	cost          int
}

type catalogGroup struct {
	namespace *Namespace
	entries   []catalogEntry
}

// Description ports Pi's createCodemodeDescription (without the models API):
// the intro and globals, the shared MCP types when a listed tool needs
// them, and one section per listed tool grouped by namespace, limited to
// the inline budget.
func Description(listed []Declaration, rendered Rendered, opts DescriptionOptions) string {
	var decls []Declaration
	for _, d := range listed {
		if !opts.Deferred[d.Name] {
			decls = append(decls, d)
		}
	}
	groups := map[string]*catalogGroup{"": {}}
	keys := []string{""}
	for _, d := range decls {
		key := ""
		var ns *Namespace
		if n, ok := opts.Namespaces[d.Name]; ok {
			nn := n
			ns, key = &nn, "ns:"+n.Name
		}
		g, ok := groups[key]
		if !ok {
			g = &catalogGroup{namespace: ns}
			groups[key] = g
			keys = append(keys, key)
		}
		id := Identifier(d.Name)
		heading := "### `" + id + "`"
		if id != d.Name {
			heading = "### `" + id + "` (`" + d.Name + "`)"
		}
		section := heading + "\n" + strings.TrimSpace(rendered.Samples[d.Name])
		g.entries = append(g.entries, catalogEntry{name: d.Name, section: section, cost: int(math.Ceil(float64(utf16Len(section)) / 4))})
	}
	ordered := make([]*catalogGroup, 0, len(keys))
	for _, k := range keys {
		ordered = append(ordered, groups[k])
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i].namespace, ordered[j].namespace
		if a == nil {
			return b != nil
		}
		if b == nil {
			return false
		}
		return a.Name < b.Name
	})
	shown := selectCatalog(ordered, opts.InlineBudget)
	base := Texts.BaseDescription
	if opts.Models {
		base = strings.ReplaceAll(Texts.BaseDescriptionModels, "{{CODEMODE_DOCS}}", opts.DocsPath)
	}
	sections := []string{base}
	for _, d := range decls {
		if shown[d.Name] && rendered.MCPResult[d.Name] {
			sections = append(sections, "Shared MCP Types:\n```ts\n"+rendered.MCPPrelude+"\n```")
			break
		}
	}
	if len(decls) == 0 {
		return strings.Join(sections, "\n\n")
	}
	toolSections := []string{"Nested tools:"}
	for _, g := range ordered {
		var visible []catalogEntry
		for _, e := range g.entries {
			if shown[e.name] {
				visible = append(visible, e)
			}
		}
		if g.namespace != nil {
			listing := ""
			switch {
			case len(visible) == len(g.entries):
			case len(visible) == 0:
				listing = " (tools not listed)"
			default:
				listing = " (some tools not listed)"
			}
			head := "## " + g.namespace.Name + listing
			if desc := strings.TrimSpace(g.namespace.Description); desc != "" {
				head += "\n" + desc
			}
			toolSections = append(toolSections, head)
		}
		for _, e := range visible {
			toolSections = append(toolSections, e.section)
		}
	}
	sections = append(sections, strings.Join(toolSections, "\n\n"))
	return strings.Join(sections, "\n\n")
}

// selectCatalog ports Pi's selectCatalog: each round, every group places its
// cheapest remaining tool; a group whose next tool does not fit drops out.
func selectCatalog(groups []*catalogGroup, budget *int) map[string]bool {
	shown := map[string]bool{}
	if budget == nil {
		for _, g := range groups {
			for _, e := range g.entries {
				shown[e.name] = true
			}
		}
		return shown
	}
	queues := make([][]catalogEntry, 0, len(groups))
	for _, g := range groups {
		q := append([]catalogEntry(nil), g.entries...)
		sort.SliceStable(q, func(i, j int) bool { return q[i].cost < q[j].cost })
		if len(q) > 0 {
			queues = append(queues, q)
		}
	}
	remaining := *budget
	for len(queues) > 0 {
		var next [][]catalogEntry
		for _, q := range queues {
			if q[0].cost > remaining {
				continue
			}
			remaining -= q[0].cost
			shown[q[0].name] = true
			if q = q[1:]; len(q) > 0 {
				next = append(next, q)
			}
		}
		queues = next
	}
	return shown
}

// utf16Len counts UTF-16 code units, as JavaScript's String.length does.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// SourceOptions are the options of a // @options: line.
type SourceOptions struct {
	MaxOutputTokens *int
	TimeoutMs       *int
}

const (
	optionsPrefix       = "// @options:"
	supportedFieldsText = "`max_output_tokens` and `timeout_ms`"
	maxTimeoutMs        = 2_147_483_647
)

// ParseSource ports Pi's parseCodemodeSource: an optional first-line
// "// @options: {...}" with max_output_tokens and timeout_ms. The options
// line is replaced by an empty line so line numbers still match.
func ParseSource(input string) (string, SourceOptions, error) {
	var opts SourceOptions
	if strings.TrimSpace(input) == "" {
		return "", opts, fmt.Errorf("Expected JavaScript source text (non-empty). Provide JS only, optionally with a first line `// @options: {\"max_output_tokens\": 1000}`.")
	}
	newline := strings.IndexByte(input, '\n')
	first := input
	if newline >= 0 {
		first = input[:newline]
	}
	first = strings.TrimSuffix(first, "\r")
	trimmed := strings.TrimLeft(first, " \t\r\n\f\v")
	if !strings.HasPrefix(trimmed, optionsPrefix) {
		return input, opts, nil
	}
	code := ""
	if newline >= 0 {
		code = input[newline:]
	}
	if strings.TrimSpace(code) == "" {
		return "", opts, fmt.Errorf("The @options line must be followed by JavaScript source on subsequent lines")
	}
	directive := strings.TrimSpace(trimmed[len(optionsPrefix):])
	if directive == "" {
		return "", opts, fmt.Errorf("@options must be a JSON object with supported fields %s", supportedFieldsText)
	}
	var value any
	if err := json.Unmarshal([]byte(directive), &value); err != nil {
		return "", opts, fmt.Errorf("@options must be valid JSON with supported fields %s: %v", supportedFieldsText, err)
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return "", opts, fmt.Errorf("@options must be a JSON object with supported fields %s", supportedFieldsText)
	}
	for key := range fields {
		if key != "max_output_tokens" && key != "timeout_ms" {
			return "", opts, fmt.Errorf("@options only supports %s; got `%s`", supportedFieldsText, key)
		}
	}
	safeInt := func(v any) (int, bool) {
		f, ok := v.(float64)
		if !ok || f < 0 || f != math.Trunc(f) || f > 9007199254740991 {
			return 0, false
		}
		return int(f), true
	}
	if v, ok := fields["max_output_tokens"]; ok {
		n, ok := safeInt(v)
		if !ok {
			return "", opts, fmt.Errorf("@options field `max_output_tokens` must be a non-negative safe integer")
		}
		opts.MaxOutputTokens = &n
	}
	if v, ok := fields["timeout_ms"]; ok {
		n, ok := safeInt(v)
		if !ok || n == 0 || n > maxTimeoutMs {
			return "", opts, fmt.Errorf("@options field `timeout_ms` must be a positive integer up to %d", maxTimeoutMs)
		}
		opts.TimeoutMs = &n
	}
	return code, opts, nil
}
