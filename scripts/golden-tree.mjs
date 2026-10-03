// Golden data for gi's port of Pi's /tree (internal/tui/tree_selector.go):
// Pi's own TreeSelectorComponent over a branched session, rendered as plain
// text after each key, with what Enter selects and what Ctrl+X copies.
//   bun scripts/golden-tree.mjs [node_modules dir]
import { existsSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";

const candidates = [process.argv[2], `${homedir()}/.bun/install/global/node_modules`, "/workspace/.cache/pi-ref/node_modules"];
const root = candidates.find((dir) => dir && existsSync(`${dir}/@earendil-works/pi-coding-agent/dist/index.js`));
if (!root) throw new Error("pi-coding-agent not found");
const agent = `${root}/@earendil-works/pi-coding-agent/dist`;
const { initTheme } = await import(`${agent}/modes/interactive/theme/theme.js`);
const { TreeSelectorComponent } = await import(`${agent}/modes/interactive/components/tree-selector.js`);
const { KeybindingsManager } = await import(`${agent}/core/keybindings.js`);
const { setKeybindings } = await import(`${root}/@earendil-works/pi-tui/dist/index.js`);
initTheme("dark");
setKeybindings(new KeybindingsManager());
process.env.HOME = "/home/u";
// A fixed clock in UTC, so label timestamps render the same everywhere.
process.env.TZ = "UTC";
const NOW = Date.parse("2026-10-03T21:12:00Z");
const RealDate = Date;
globalThis.Date = class extends RealDate {
	constructor(...args) { super(...(args.length ? args : [NOW])); }
	static now() { return NOW; }
};

const strip = (s) => s.replace(/\x1b\[[0-9;]*m/g, "").replace(/\x1b_[^\x07]*\x07/g, "");
const user = (id, parentId, text) => ({ type: "message", id, parentId, message: { role: "user", content: text } });
const assistant = (id, parentId, text, calls = []) => ({
	type: "message", id, parentId,
	message: { role: "assistant", content: [...(text ? [{ type: "text", text }] : []), ...calls.map(([cid, name, args]) => ({ type: "toolCall", id: cid, name, arguments: args }))], stopReason: calls.length ? "toolUse" : "stop" },
});
const result = (id, parentId, callId, name, text) => ({ type: "message", id, parentId, message: { role: "toolResult", toolCallId: callId, toolName: name, content: [{ type: "text", text }] } });
// Main line: u1 a1 u2 a2(tool) r2 a3 u3 a4; a branch from a1 (u5 a5) and one from a3 (u6 a6 u7 a7 ...).
const entries = [
	user("u1", null, "Plan the refactor of the parser"),
	assistant("a1", "u1", "Here is a plan:\n1. split lexer"),
	user("u2", "a1", "Start with the lexer"),
	assistant("a2", "u2", "", [["c1", "read", { path: "/home/u/src/lexer.go", offset: 10, limit: 20 }], ["c2", "bash", { command: "go test ./... && echo done with a very long tail of words" }]]),
	result("r1", "a2", "c1", "read", "package lexer"),
	result("r2", "r1", "c2", "bash", "ok"),
	assistant("a3", "r2", "Lexer split; tests pass."),
	user("u3", "a3", "Now the parser"),
	assistant("a4", "u3", "Parser done."),
	user("u5", "a1", "Actually, start with the parser"),
	assistant("a5", "u5", "Parser first then."),
	user("u6", "a3", "Rename the package instead"),
	assistant("a6", "u6", "Renamed."),
	{ type: "compaction", id: "k1", parentId: "a6", tokensBefore: 45210, summary: "Compacted summary" },
	user("u7", "k1", "And update the docs"),
	assistant("a7", "u7", "Docs updated."),
	{ type: "model_change", id: "m1", parentId: "a7", modelId: "gpt-5" },
	user("u8", "m1", "Thanks"),
];
const labels = { a3: "split", u6: "rename" };
const buildTree = () => {
	const nodes = new Map(entries.map((e) => [e.id, { entry: e, children: [], label: labels[e.id] }]));
	const roots = [];
	for (const e of entries) (e.parentId ? nodes.get(e.parentId).children : roots).push(nodes.get(e.id));
	return roots;
};
const keys = {
	up: "\x1b[A", down: "\x1b[B", left: "\x1b[D", right: "\x1b[C", pgup: "\x1b[5~", pgdn: "\x1b[6~", enter: "\r", esc: "\x1b", backspace: "\x7f",
	"ctrl+left": "\x1b[1;5D", "ctrl+right": "\x1b[1;5C", "alt+left": "\x1b[1;3D",
	"ctrl+d": "\x04", "ctrl+t": "\x14", "ctrl+u": "\x15", "ctrl+l": "\x0c", "ctrl+a": "\x01", "ctrl+o": "\x0f", "ctrl+x": "\x18",
	L: "L", T: "T",
};
const scenarios = {
	navigate: { leaf: "a4", steps: ["up", "up", "up", "up", "up", "up", "pgup", "down", "pgdn", "left", "right", "enter"] },
	branches: { leaf: "u8", steps: ["ctrl+left", "ctrl+left", "ctrl+left", "ctrl+right", "ctrl+right", "ctrl+left", "up", "ctrl+left", "ctrl+right", "enter"] },
	filters: { leaf: "a4", steps: ["ctrl+t", "ctrl+u", "ctrl+u", "ctrl+l", "ctrl+a", "ctrl+o", "ctrl+o", "ctrl+o", "ctrl+d"] },
	search: { leaf: "a4", steps: [["type", "parser"], "down", "backspace", "esc", ["type", "zzz"], "esc", "esc"] },
	labels: { leaf: "a4", steps: ["up", "L", ["type", "mark"], "enter", "T", "L", "esc", "up", "ctrl+x"] },
};
const out = { entries, labels, scenarios: {} };
for (const [width, height] of [[80, 24], [44, 16]]) {
	for (const [name, scenario] of Object.entries(scenarios)) {
		let selected = null, cancelled = false, labelled = null, copied = null;
		const selector = new TreeSelectorComponent(buildTree(), scenario.leaf, height, (id) => { selected = id; }, () => { cancelled = true; }, (id, label) => { labelled = [id, label ?? null]; });
		selector.onCopy = (text) => { copied = text ?? null; };
		const render = () => selector.render(width).map((line) => strip(line).trimEnd());
		const states = [{ step: "open", rows: render() }];
		for (const step of scenario.steps) {
			selected = null; labelled = null; copied = null;
			if (Array.isArray(step)) for (const ch of step[1]) selector.handleInput(ch);
			else selector.handleInput(keys[step]);
			states.push({ step: Array.isArray(step) ? step.join(":") : step, rows: render(), selected, cancelled, labelled, copied });
		}
		(out.scenarios[name] ??= { leaf: scenario.leaf, steps: scenario.steps, sizes: {} }).sizes[`${width}x${height}`] = states;
	}
}
writeFileSync("internal/tui/testdata/pi-tree.json", JSON.stringify(out, null, 1) + "\n");
