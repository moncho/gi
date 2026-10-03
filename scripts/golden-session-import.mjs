// Golden data for gi's /import (internal/sessionimport): Pi session files
// built with Pi's own SessionManager, reopened as Pi's /import does, with
// the active branch, model context entries, name, model, thinking level and
// labels Pi derives from them.
//   bun scripts/golden-session-import.mjs [node_modules dir]
import { existsSync, mkdtempSync, writeFileSync } from "node:fs";
import { homedir, tmpdir } from "node:os";
import { join } from "node:path";

const candidates = [process.argv[2], process.env.PI_NODE_MODULES, `${homedir()}/.bun/install/global/node_modules`, "/workspace/.cache/pi-ref/node_modules"];
const root = candidates.find((dir) => dir && existsSync(`${dir}/@earendil-works/pi-coding-agent/dist/index.js`));
if (!root) throw new Error("pi-coding-agent not found");
const sm = await import(`${root}/@earendil-works/pi-coding-agent/dist/core/session-manager.js`);

let clock = Date.parse("2026-10-03T10:00:00.000Z");
const RealDate = Date;
globalThis.Date = class extends RealDate {
	constructor(...args) { super(...(args.length ? args : [(clock += 1000)])); }
	static now() { return (clock += 1000); }
};
const usage = { input: 10, output: 5, cacheRead: 0, cacheWrite: 0, totalTokens: 15, cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: 0 } };
const user = (text) => ({ role: "user", content: [{ type: "text", text }], timestamp: Date.now() });
const assistant = (blocks, extra = {}) => ({ role: "assistant", content: blocks, api: "anthropic-messages", provider: "anthropic", model: "claude-sonnet-4-5", usage, stopReason: blocks.some((b) => b.type === "toolCall") ? "toolUse" : "stop", timestamp: Date.now(), ...extra });
const toolResult = (id, name, text, isError = false) => ({ role: "toolResult", toolCallId: id, toolName: name, content: [{ type: "text", text }], isError, timestamp: Date.now() });

const sessions = {};
const dump = (name, s) => {
	const lines = [s.getHeader(), ...s.getEntries()].map((e) => JSON.stringify(e));
	const dir = mkdtempSync(join(tmpdir(), "pi-import-"));
	const file = join(dir, `${name}.jsonl`);
	writeFileSync(file, lines.join("\n") + "\n");
	// Reopened as /import does: the leaf is the last entry.
	const reopened = sm.SessionManager.open(file, dir);
	const leaf = reopened.getLeafId();
	const entries = reopened.getEntries();
	const byId = new Map(entries.map((e) => [e.id, e]));
	const path = [];
	for (let e = byId.get(leaf); e; e = e.parentId ? byId.get(e.parentId) : undefined) path.unshift(e.id);
	const context = sm.buildContextEntries(entries, leaf, byId);
	const settings = sm.buildSessionContext(entries, leaf, byId);
	const labels = {};
	for (const id of path) if (reopened.getLabel(id)) labels[id] = reopened.getLabel(id);
	sessions[name] = {
		jsonl: lines.join("\n") + "\n",
		path,
		context: context.map((e) => e.id),
		name: reopened.getSessionName() ?? null,
		model: settings.model,
		thinkingLevel: settings.thinkingLevel,
		hasThinkingEntry: path.some((id) => byId.get(id).type === "thinking_level_change"),
		labels,
	};
};

// A branched session: compaction, a branch summary, labels, a context edit,
// custom messages, ! commands, images, model and thinking changes.
{
	const s = sm.SessionManager.inMemory("/home/u/project");
	s.appendSessionInfo("first name");
	const u1 = s.appendMessage(user("Read the parser"));
	s.appendMessage(assistant([{ type: "thinking", thinking: "Look." }, { type: "text", text: "Reading." }, { type: "toolCall", id: "t1", name: "read", arguments: { path: "src/parse.go" } }]));
	s.appendMessage(toolResult("t1", "read", "package parse"));
	const a1 = s.appendMessage(assistant([{ type: "text", text: "It parses." }]));
	s.appendLabelChange(u1, "start");
	s.appendModelChange("openai", "gpt-5");
	const u2 = s.appendMessage(user("Fix the lexer"));
	s.appendMessage(assistant([{ type: "text", text: "Fixed." }], { provider: "openai", model: "gpt-5", api: "openai-responses" }));
	s.appendCompaction("Summary of the parser work.", u2, 4000);
	s.appendThinkingLevelChange("high");
	s.appendMessage({ role: "bashExecution", command: "go test ./...", output: "ok", exitCode: 0, cancelled: false, truncated: false, timestamp: Date.now() });
	s.appendMessage({ role: "bashExecution", command: "ls", output: "a\nb", exitCode: 0, cancelled: false, truncated: false, excludeFromContext: true, timestamp: Date.now() });
	s.appendCustomMessageEntry("note", "Remember the tests.", true);
	s.appendCustomEntry("ext-state", { count: 1 });
	s.appendCustomEntry("gi.system", { text: "Queued prompt" });
	const u3 = s.appendMessage({ role: "user", content: [{ type: "text", text: "Look at this" }, { type: "image", data: "iVBORw0KGgo=", mimeType: "image/png" }], timestamp: Date.now() });
	const a3 = s.appendMessage(assistant([{ type: "text", text: "A secret: hunter2" }], { provider: "openai", model: "gpt-5" }));
	const edit = s.appendContextEdit(a3, { content: "[redacted]" });
	// An abandoned branch from a1, then back with a summary.
	s.branch(a1);
	s.appendMessage(user("Try another way"));
	s.appendMessage(assistant([{ type: "text", text: "Other way." }]));
	s.branchWithSummary(edit, "Tried another way; it failed.");
	s.appendMessage(user("Continue"));
	s.appendMessage(assistant([{ type: "text", text: "Continuing." }], { provider: "openai", model: "gpt-5" }));
	s.appendLabelChange(u3, "image");
	s.appendSessionInfo("Parser work");
	dump("branched", s);
}
// A plain linear session without names or changes.
{
	const s = sm.SessionManager.inMemory("/home/u/project");
	s.appendMessage(user("Hello"));
	s.appendMessage(assistant([{ type: "text", text: "Hi." }]));
	dump("linear", s);
}
writeFileSync("internal/sessionimport/testdata/pi-sessions.json", JSON.stringify(sessions, null, 1) + "\n");
console.log(Object.fromEntries(Object.entries(sessions).map(([k, v]) => [k, { path: v.path.length, context: v.context.length, name: v.name, model: v.model, thinking: v.thinkingLevel, labels: v.labels }])));
