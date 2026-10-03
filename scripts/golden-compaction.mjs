// Golden data for gi's port of Pi's compaction summaries
// (internal/compaction/summary.go), computed by Pi's own code with a stub
// model that records each prompt.
//   bun scripts/golden-compaction.mjs
import { writeFileSync } from "node:fs";
import { homedir } from "node:os";
import { existsSync as piModulesExist } from "node:fs";
// Pi's packages: PI_NODE_MODULES, the global bun install, or the reference install.
const PI_MODULES = [process.env.PI_NODE_MODULES, `${homedir()}/.bun/install/global/node_modules`, "/workspace/.cache/pi-ref/node_modules"].find((dir) => dir && piModulesExist(`${dir}/@earendil-works/pi-coding-agent`));
const root = `${PI_MODULES}/@earendil-works`;
const c = await import(`${root}/pi-coding-agent/dist/core/compaction/compaction.js`);
const u = await import(`${root}/pi-coding-agent/dist/core/compaction/utils.js`);

const user = (text) => ({ role: "user", content: [{ type: "text", text }], timestamp: 1 });
const assistant = (blocks) => ({ role: "assistant", content: blocks, timestamp: 2, stopReason: "stop" });
const toolResult = (text) => ({ role: "toolResult", toolCallId: "t", toolName: "read", content: [{ type: "text", text }], isError: false, timestamp: 3 });
const history = [
	user("Fix the parser in src/parse.go"),
	assistant([{ type: "thinking", thinking: "Look at the file first." }, { type: "text", text: "Reading it." }, { type: "toolCall", id: "t1", name: "read", arguments: { path: "src/parse.go" } }]),
	toolResult("package parse\n" + "x".repeat(2500)),
	assistant([{ type: "toolCall", id: "t2", name: "edit", arguments: { edits: [{ newText: "a>b", oldText: "a<b" }], path: "src/parse.go" } }]),
	toolResult("Successfully replaced 1 block(s) in src/parse.go."),
	assistant([{ type: "text", text: "Done." }]),
];
const prefix = [user("Now add tests"), assistant([{ type: "toolCall", id: "t3", name: "write", arguments: { content: "package parse", path: "src/parse_test.go" } }])];

const prompts = [];
const streamFn = async (model, context, options) => {
	const msg = context.messages.find((m) => m.role === "user");
	const sys = context.messages.find((m) => m.role === "system");
	const system = typeof sys?.content === "string" ? sys.content : sys?.content?.map((b) => b.text).join("") ?? context.systemPrompt;
	prompts.push({ system, prompt: msg.content[0].text, maxTokens: options.maxTokens });
	return { result: async () => ({ role: "assistant", content: [{ type: "text", text: `SUMMARY ${prompts.length}` }], stopReason: "stop", usage: { input: 10, output: 5, cacheRead: 0, cacheWrite: 0, totalTokens: 15, cost: { input: 0.1, output: 0.2, cacheRead: 0, cacheWrite: 0, total: 0.3 } } }) };
};
const model = { provider: "p", id: "m", reasoning: false, maxTokens: 4000 };
const fileOps = (read, written) => ({ read: new Set(read), written: new Set(written), edited: new Set() });

const out = { serialized: u.serializeConversation([...history, ...prefix]), cases: [] };
for (const [name, prep, instructions] of [
	["initial", { messagesToSummarize: history, turnPrefixMessages: [], isSplitTurn: false, previousSummary: undefined, fileOps: fileOps(["src/parse.go"], ["src/parse.go"]), settings: { reserveTokens: 16384 } }, undefined],
	["update", { messagesToSummarize: history, turnPrefixMessages: [], isSplitTurn: false, previousSummary: "OLD SUMMARY", fileOps: fileOps(["README.md"], []), settings: { reserveTokens: 16384 } }, "focus on tests"],
	["split", { messagesToSummarize: history, turnPrefixMessages: prefix, isSplitTurn: true, previousSummary: "OLD", fileOps: fileOps([], ["src/parse_test.go"]), settings: { reserveTokens: 3000 } }, undefined],
]) {
	prompts.length = 0;
	const result = await c.compact({ firstKeptEntryId: "x", tokensBefore: 0, ...prep }, model, "key", {}, instructions, undefined, undefined, streamFn);
	out.cases.push({ name, instructions: instructions ?? "", previousSummary: prep.previousSummary ?? "", split: prep.isSplitTurn, reserveTokens: prep.settings.reserveTokens, prompts: [...prompts], summary: result.summary, usage: result.usage });
}
out.estimates = [...history, ...prefix].map((m) => c.estimateTokens(m));
writeFileSync(new URL("../internal/compaction/testdata/pi-compaction.json", import.meta.url), JSON.stringify({ messages: [...history, ...prefix], ...out }, null, 1) + "\n");
console.log(out.cases.map((x) => `${x.name}: ${x.prompts.length} prompts, maxTokens ${x.prompts.map((p) => p.maxTokens)}`).join("\n"));
