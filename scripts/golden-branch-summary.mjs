// Golden data for gi's port of Pi's branch summaries
// (internal/compaction/branch_summary.go), computed by Pi's own
// generateBranchSummary with a stub model that records the prompt.
//   bun scripts/golden-branch-summary.mjs [node_modules dir]
import { existsSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";

const candidates = [process.argv[2], `${homedir()}/.bun/install/global/node_modules`, "/workspace/.cache/pi-ref/node_modules"];
const root = candidates.find((dir) => dir && existsSync(`${dir}/@earendil-works/pi-coding-agent/dist/index.js`));
if (!root) throw new Error("pi-coding-agent not found");
const b = await import(`${root}/@earendil-works/pi-coding-agent/dist/core/compaction/branch-summarization.js`);

const message = (m) => ({ type: "message", id: `e${Math.random()}`, parentId: null, timestamp: "2026-10-03T21:12:00.000Z", message: { timestamp: 1, ...m } });
const user = (text) => message({ role: "user", content: [{ type: "text", text }] });
const assistant = (blocks) => message({ role: "assistant", content: blocks, stopReason: "stop" });
const toolResult = (text) => message({ role: "toolResult", toolCallId: "t", toolName: "read", content: [{ type: "text", text }], isError: false });
const previous = "Earlier branch.\n\n<read-files>\nREADME.md\n</read-files>\n\n<modified-files>\ndocs/a.md\n</modified-files>";
// Tool-call arguments in key order: gi's serialization orders keys (Go maps),
// as in scripts/golden-compaction.mjs.
const entries = [
	{ type: "compaction", id: "k", parentId: null, timestamp: "2026-10-03T21:12:00.000Z", summary: "Compacted: " + "y".repeat(400), tokensBefore: 5000, firstKeptEntryId: "x" },
	{ type: "branch_summary", id: "bs", parentId: "k", timestamp: "2026-10-03T21:12:00.000Z", fromId: "z", summary: previous, details: { readFiles: ["README.md"], modifiedFiles: ["docs/a.md"] } },
	user("Fix the parser in src/parse.go"),
	assistant([{ type: "thinking", thinking: "Look first." }, { type: "text", text: "Reading it." }, { type: "toolCall", id: "t1", name: "read", arguments: { path: "src/parse.go" } }]),
	toolResult("package parse\n" + "x".repeat(2500)),
	assistant([{ type: "toolCall", id: "t2", name: "edit", arguments: { edits: [{ newText: "a>b", oldText: "a<b" }], path: "src/parse.go" } }]),
	toolResult("ok"),
	assistant([{ type: "text", text: "Done." }]),
	user("Now write tests"),
	assistant([{ type: "toolCall", id: "t3", name: "write", arguments: { content: "package parse", path: "src/parse_test.go" } }]),
];
let prompt = null;
const streamFn = async (model, context, options) => {
	const sys = context.messages.find((m) => m.role === "system");
	const user = context.messages.find((m) => m.role === "user");
	prompt = { system: typeof sys?.content === "string" ? sys.content : context.systemPrompt, prompt: user.content[0].text, maxTokens: options.maxTokens };
	return { result: async () => ({ role: "assistant", content: [{ type: "text", text: "SUMMARY" }], stopReason: "stop", usage: { input: 10, output: 5, cacheRead: 0, cacheWrite: 0, totalTokens: 15, cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: 0 } } }) };
};
const cases = [
	{ name: "all", contextWindow: 200000, maxTokens: 64000, reserveTokens: 16384 },
	{ name: "focus", contextWindow: 200000, maxTokens: 2000, reserveTokens: 16384, customInstructions: "the test plan" },
	{ name: "replace", contextWindow: 200000, maxTokens: 64000, reserveTokens: 16384, customInstructions: "Only list the files.", replaceInstructions: true },
	// Budgets (window - reserve) that keep a summary over budget because the
	// rest is under 90% of it, that drop one, and that leave nothing.
	{ name: "keep-compaction", contextWindow: 200, maxTokens: 64000, reserveTokens: 100 },
	{ name: "keep-branch-summary", contextWindow: 170, maxTokens: 64000, reserveTokens: 100 },
	{ name: "drop-summaries", contextWindow: 160, maxTokens: 64000, reserveTokens: 100 },
	{ name: "nothing-fits", contextWindow: 110, maxTokens: 64000, reserveTokens: 100 },
	{ name: "unknown-window", contextWindow: 0, maxTokens: 0, reserveTokens: 16384 },
];
const out = { entries, cases: [] };
for (const c of cases) {
	prompt = null;
	const model = { provider: "p", id: "m", api: "openai-completions", reasoning: false, contextWindow: c.contextWindow, maxTokens: c.maxTokens };
	const result = await b.generateBranchSummary(entries, { model, apiKey: "k", customInstructions: c.customInstructions, replaceInstructions: c.replaceInstructions, reserveTokens: c.reserveTokens, streamFn });
	if (result.error) throw new Error(result.error);
	out.cases.push({ ...c, request: prompt, summary: result.summary });
}
const empty = await b.generateBranchSummary([toolResult("x")], { model: { provider: "p", id: "m", contextWindow: 1000, maxTokens: 100 }, apiKey: "k", streamFn });
out.empty = empty.summary;
writeFileSync("internal/compaction/testdata/pi-branch-summary.json", JSON.stringify(out, null, 1) + "\n");
console.log(out.cases.map((c) => `${c.name}: ${c.request ? c.request.prompt.length + " chars, maxTokens " + c.request.maxTokens : c.summary}`).join("\n"), "\nempty:", out.empty);
