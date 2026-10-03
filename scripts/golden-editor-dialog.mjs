// Golden data for gi's port of Pi's ctx.ui.editor dialog
// (internal/tui/editor_dialog.go), rendered by Pi's own
// ExtensionEditorComponent: rows as plain text after each key.
//   bun scripts/golden-editor-dialog.mjs [node_modules dir]
import { existsSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";

const candidates = [process.argv[2], `${homedir()}/.bun/install/global/node_modules`, "/workspace/.cache/pi-ref/node_modules"];
const root = candidates.find((dir) => dir && existsSync(`${dir}/@earendil-works/pi-coding-agent/dist/index.js`));
if (!root) throw new Error("pi-coding-agent not found");
const agent = `${root}/@earendil-works/pi-coding-agent/dist`;
const { initTheme } = await import(`${agent}/modes/interactive/theme/theme.js`);
const { ExtensionEditorComponent } = await import(`${agent}/modes/interactive/components/extension-editor.js`);
const { KeybindingsManager } = await import(`${agent}/core/keybindings.js`);
const { setKeybindings } = await import(`${root}/@earendil-works/pi-tui/dist/index.js`);
initTheme("dark");
const keybindings = new KeybindingsManager();
setKeybindings(keybindings);

const strip = (s) => s.replace(/\x1b\[[0-9;]*m/g, "").replace(/\x1b_[^\x07]*\x07/g, "").replace(/\x1b\]8;;[^\x07\x1b]*(?:\x07|\x1b\\)/g, "");
const keys = { enter: "\r", esc: "\x1b", backspace: "\x7f", "shift+enter": "\x1b[13;2u" };
const tui = { requestRender() {}, terminal: { rows: 24, columns: 80 } };
const scenarios = {
	submit: [["type", "focus on the lexer"], "backspace", "backspace", ["type", "er"], "shift+enter", ["type", "and tests"], "enter"],
	cancel: [["type", "x"], "esc"],
	empty: ["enter"],
};
const out = {};
for (const width of [80, 40]) {
	for (const [name, steps] of Object.entries(scenarios)) {
		let result = null;
		const dialog = new ExtensionEditorComponent(tui, keybindings, "Custom summarization instructions", undefined, (text) => { result = { submit: text }; }, () => { result = { cancel: true }; });
		dialog.focused = true;
		const render = () => dialog.render(width).map((line) => strip(line).trimEnd());
		const states = [{ step: "open", rows: render(), result }];
		for (const step of steps) {
			if (Array.isArray(step)) for (const ch of step[1]) dialog.handleInput(ch);
			else dialog.handleInput(keys[step]);
			states.push({ step: Array.isArray(step) ? step.join(":") : step, rows: render(), result });
		}
		(out[name] ??= { steps, widths: {} }).widths[width] = states;
	}
}
writeFileSync("internal/tui/testdata/pi-editor-dialog.json", JSON.stringify(out, null, 1) + "\n");
