// Golden data for gi's port of Pi's /settings (internal/tui/settings_menu.go):
// pi-tui's SettingsList inside Pi's selector borders, with Pi's items for
// the settings gi implements, rendered as plain text after each key.
//   bun scripts/golden-settings.mjs [node_modules dir]
import { existsSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";

const candidates = [process.argv[2], `${homedir()}/.bun/install/global/node_modules`, "/workspace/.cache/pi-ref/node_modules"];
const root = candidates.find((dir) => dir && existsSync(`${dir}/@earendil-works/pi-coding-agent/dist/index.js`));
if (!root) throw new Error("pi-coding-agent not found");
const agent = `${root}/@earendil-works/pi-coding-agent/dist`;
const { initTheme, getSettingsListTheme } = await import(`${agent}/modes/interactive/theme/theme.js`);
const { DynamicBorder } = await import(`${agent}/modes/interactive/components/dynamic-border.js`);
const { KeybindingsManager } = await import(`${agent}/core/keybindings.js`);
const tui = await import(`${root}/@earendil-works/pi-tui/dist/index.js`);
initTheme("dark");
tui.setKeybindings(new KeybindingsManager());

const strip = (s) => s.replace(/\x1b\[[0-9;]*m/g, "").replace(/\x1b_[^\x07]*\x07/g, "");
// Pi's items (settings-selector.js) for the settings gi has, in Pi's order.
const items = () => [
	{ id: "autocompact", label: "Auto-compact", description: "Automatically compact context when it gets too large", currentValue: "true", values: ["true", "false"] },
	{ id: "hide-thinking", label: "Hide thinking", description: "Hide thinking blocks in assistant responses", currentValue: "false", values: ["true", "false"] },
	{ id: "quiet-startup", label: "Quiet startup", description: "Disable verbose printing at startup (header: keep only the startup header)", currentValue: "false", values: ["true", "header", "false"] },
	{ id: "tui-mode", label: "TUI mode", description: "Interface layout; regular mode uses the terminal's normal scrollback", currentValue: "fullscreen", values: ["regular", "fullscreen"] },
	{ id: "fullscreen-wheel-scroll-lines", label: "Fullscreen wheel scrolling", description: "Lines per mouse-wheel event in fullscreen mode; 'auto' speeds up fast wheel spins where the terminal does not", currentValue: "auto", values: ["auto", "1", "2", "3", "5", "10"] },
];
const keys = { up: "\x1b[A", down: "\x1b[B", enter: "\r", space: " ", backspace: "\x7f" };
const scenarios = {
	cycle: ["enter", "down", "space", "down", "enter", "enter", "up", "up", "up", "enter", "down", "down", "down", "down", "enter"],
	search: [["type", "wheel"], "space", "enter", "backspace", "backspace", "backspace", "backspace", "backspace", ["type", "zzz"], "backspace", "backspace", "backspace", "down"],
};
const out = {};
for (const width of [80, 50]) {
	for (const [name, steps] of Object.entries(scenarios)) {
		const changes = [];
		const list = new tui.SettingsList(items(), 10, getSettingsListTheme(), (id, value) => changes.push([id, value]), () => {}, { enableSearch: true });
		const render = () => [new DynamicBorder(), list, new DynamicBorder()].flatMap((c) => c.render(width)).map((l) => strip(l).trimEnd());
		const states = [{ step: "open", rows: render(), changes: [] }];
		for (const step of steps) {
			const before = changes.length;
			if (Array.isArray(step)) for (const ch of step[1]) list.handleInput(ch);
			else list.handleInput(keys[step]);
			states.push({ step: Array.isArray(step) ? step.join(":") : step, rows: render(), changes: changes.slice(before) });
		}
		(out[name] ??= { steps, widths: {} }).widths[width] = states;
	}
}
writeFileSync("internal/tui/testdata/pi-settings.json", JSON.stringify(out, null, 1) + "\n");
