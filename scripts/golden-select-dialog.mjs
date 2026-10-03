// Golden data for gi's port of Pi's ctx.ui.select selector
// (internal/tui/select_dialog.go), rendered by Pi's own
// ExtensionSelectorComponent: rows as plain text after each key.
//   bun scripts/golden-select-dialog.mjs [node_modules dir]
import { existsSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";

const candidates = [process.argv[2], `${homedir()}/.bun/install/global/node_modules`, "/workspace/.cache/pi-ref/node_modules"];
const root = candidates.find((dir) => dir && existsSync(`${dir}/@earendil-works/pi-coding-agent/dist/index.js`));
if (!root) throw new Error("pi-coding-agent not found");
const base = `${root}/@earendil-works/pi-coding-agent/dist/modes/interactive`;
const { initTheme } = await import(`${base}/theme/theme.js`);
const { ExtensionSelectorComponent } = await import(`${base}/components/extension-selector.js`);
initTheme("dark");

const strip = (s) => s.replace(/\x1b\[[0-9;]*m/g, "").replace(/\x1b\]8;;[^\x07\x1b]*(?:\x07|\x1b\\)/g, "");
const keys = { up: "\x1b[A", down: "\x1b[B", j: "j", k: "k", enter: "\r", escape: "\x1b" };
const width = 40;
const steps = ["start", "down", "down", "down", "k", "up", "up", "j", "enter"];
let result = null;
const dialog = new ExtensionSelectorComponent("MCP server", ["github", "local", "remote-with-a-rather-long-name-here"], (v) => { result = { select: v }; }, () => { result = { cancel: true }; });
const states = [];
for (const step of steps) {
	if (step !== "start") dialog.handleInput(keys[step]);
	states.push({ step, rows: dialog.render(width).map((line) => strip(line).trimEnd()), result });
}
const cancelled = new ExtensionSelectorComponent("MCP server", ["github", "local"], () => { result = { select: true }; }, () => { result = { cancel: true }; });
result = null;
cancelled.handleInput(keys.escape);
// Pi's showExtensionConfirm (/import): the title and message on two lines,
// Yes and No.
const confirm = new ExtensionSelectorComponent("Import session\nReplace current session with ~/sessions/a-rather-long-session-file-name.jsonl?", ["Yes", "No"], () => {}, () => {});
const confirmRows = confirm.render(width).map((line) => strip(line).trimEnd());
writeFileSync("internal/tui/testdata/pi-select-dialog.json", JSON.stringify({ width, states, escape: result, confirm: confirmRows }, null, 1) + "\n");
