// Golden data for gi's port of Pi's /settings (internal/tui/settings_menu.go):
// pi-tui's SettingsList inside Pi's selector borders, with Pi's items for
// the settings gi implements, rendered as plain text after each key.
//   bun scripts/golden-settings.mjs [node_modules dir]
import { existsSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";

const candidates = [process.argv[2], `${homedir()}/.bun/install/global/node_modules`, "/workspace/.cache/pi-ref/node_modules"];
const root = candidates.find((dir) => dir && existsSync(`${dir}/@earendil-works/pi-coding-agent/dist/index.js`));
if (!root) throw new Error("pi-coding-agent not found");
const agent = `${root}/@earendil-works/pi-coding-agent/dist`;
// ThemeSubmenu is module-private: load a copy of settings-selector.js that
// exports it, next to it for its relative imports. Bun caches directory
// listings, so the copy is written before anything loads from its directory.
const selectorPath = `${agent}/modes/interactive/components/settings-selector.js`;
const copy = selectorPath.replace(/\.js$/, `.golden-${process.pid}.js`);
writeFileSync(copy, `${readFileSync(selectorPath, "utf8")}\nexport { ThemeSubmenu };\n`);
process.on("exit", () => rmSync(copy, { force: true }));
const { ThemeSubmenu } = await import(copy);
const { initTheme, getSettingsListTheme } = await import(`${agent}/modes/interactive/theme/theme.js`);
const { DynamicBorder } = await import(`${agent}/modes/interactive/components/dynamic-border.js`);
const { KeybindingsManager } = await import(`${agent}/core/keybindings.js`);
const tui = await import(`${root}/@earendil-works/pi-tui/dist/index.js`);
initTheme("dark");
tui.setKeybindings(new KeybindingsManager());
const availableThemes = ["system", "dark", "light", "ocean"];

const strip = (s) => s.replace(/\x1b\[[0-9;]*m/g, "").replace(/\x1b_[^\x07]*\x07/g, "");
// Pi's items (settings-selector.js) for the settings gi has, in Pi's order.
// The theme submenu records its previews.
let previews = [];
const items = (theme = "system") => [
	{ id: "autocompact", label: "Auto-compact", description: "Automatically compact context when it gets too large", currentValue: "true", values: ["true", "false"] },
	{ id: "hide-thinking", label: "Hide thinking", description: "Hide thinking blocks in assistant responses", currentValue: "false", values: ["true", "false"] },
	{ id: "quiet-startup", label: "Quiet startup", description: "Disable verbose printing at startup (header: keep only the startup header)", currentValue: "false", values: ["true", "header", "false"] },
	{ id: "tree-filter-mode", label: "Tree filter mode", description: "Default filter when opening /tree", currentValue: "default", values: ["default", "no-tools", "user-only", "labeled-only", "all"] },
	{ id: "tui-mode", label: "TUI mode", description: "Interface layout; regular mode uses the terminal's normal scrollback", currentValue: "fullscreen", values: ["regular", "fullscreen"] },
	{ id: "fullscreen-wheel-scroll-lines", label: "Fullscreen wheel scrolling", description: "Lines per mouse-wheel event in fullscreen mode; 'auto' speeds up fast wheel spins where the terminal does not", currentValue: "auto", values: ["auto", "1", "2", "3", "5", "10"] },
	{ id: "theme", label: "Theme", description: "Color theme for the interface", currentValue: theme, submenu: (currentValue, done) => new ThemeSubmenu(currentValue, "dark", availableThemes, { onThemePreview: (name) => previews.push(name) }, done) },
];
const keys = { up: "\x1b[A", down: "\x1b[B", enter: "\r", space: " ", backspace: "\x7f", esc: "\x1b" };
const scenarios = {
	cycle: ["enter", "down", "space", "down", "enter", "enter", "up", "up", "up", "enter", "down", "down", "down", "down", "enter"],
	search: [["type", "wheel"], "space", "enter", "backspace", "backspace", "backspace", "backspace", "backspace", ["type", "zzz"], "backspace", "backspace", "backspace", "down"],
	// The theme submenu: previews while moving, Enter saves, Escape restores.
	"theme-single": ["up", "enter", "down", "down", "down", "up", "enter"],
	"theme-cancel": ["up", "enter", "down", "down", "esc", "esc"],
	"theme-automatic": ["up", "enter", "down", "enter", "enter", "down", "down", "enter", "down", "enter", "up", "esc", "down", "down", "space"],
	"theme-pair": ["up", "enter", "down", "down", "down", "enter", "up", "enter", "esc"],
};
const initialTheme = { "theme-pair": "light/ocean" };
const out = {};
for (const width of [80, 50]) {
	for (const [name, steps] of Object.entries(scenarios)) {
		const changes = [];
		previews = [];
		const list = new tui.SettingsList(items(initialTheme[name]), 10, getSettingsListTheme(), (id, value) => changes.push([id, value]), () => {}, { enableSearch: true });
		const render = () => [new DynamicBorder(), list, new DynamicBorder()].flatMap((c) => c.render(width)).map((l) => strip(l).trimEnd());
		const states = [{ step: "open", rows: render(), changes: [], previews: [] }];
		for (const step of steps) {
			const before = changes.length;
			const previewsBefore = previews.length;
			if (Array.isArray(step)) for (const ch of step[1]) list.handleInput(ch);
			else list.handleInput(keys[step]);
			states.push({ step: Array.isArray(step) ? step.join(":") : step, rows: render(), changes: changes.slice(before), previews: previews.slice(previewsBefore) });
		}
		(out[name] ??= { steps, widths: {} }).widths[width] = states;
	}
}
writeFileSync("internal/tui/testdata/pi-settings.json", JSON.stringify(out, null, 1) + "\n");
