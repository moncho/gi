// Golden data for gi's port of Pi's system theme generator
// (internal/tui/system_theme.go), computed by Pi's own code.
//   bun scripts/golden-system-theme.mjs
import { writeFileSync } from "node:fs";
import { homedir } from "node:os";
const root = `${homedir()}/.bun/install/global/node_modules/@earendil-works`;
const { generateSystemThemeColors } = await import(`${root}/pi-coding-agent/dist/modes/interactive/theme/system-theme.js`);
const hex = (h) => ({ r: parseInt(h.slice(1, 3), 16), g: parseInt(h.slice(3, 5), 16), b: parseInt(h.slice(5, 7), 16) });
const frappe = ["#51576d", "#e78284", "#a6d189", "#e5c890", "#8caaee", "#f4b8e4", "#81c8be", "#b5bfe2", "#626880", "#e78284", "#a6d189", "#e5c890", "#8caaee", "#f4b8e4", "#81c8be", "#a5adce"];
const latte = ["#5c5f77", "#d20f39", "#40a02b", "#df8e1d", "#1e66f5", "#ea76cb", "#179299", "#acb0be", "#6c6f85", "#d20f39", "#40a02b", "#df8e1d", "#1e66f5", "#ea76cb", "#179299", "#bcc0cc"];
const xterm = ["#000000", "#cd0000", "#00cd00", "#cdcd00", "#0000ee", "#cd00cd", "#00cdcd", "#e5e5e5", "#7f7f7f", "#ff0000", "#00ff00", "#ffff00", "#5c5cff", "#ff00ff", "#00ffff", "#ffffff"];
const cases = [
	{ name: "frappe", background: "#303446", foreground: "#c6d0f5", palette: frappe },
	{ name: "latte", background: "#eff1f5", foreground: "#4c4f69", palette: latte },
	{ name: "xterm", background: "#000000", foreground: "#ffffff", palette: xterm },
	{ name: "solarizedBgOnly", background: "#002b36" },
	{ name: "lightBgFg", background: "#fdf6e3", foreground: "#657b83" },
	{ name: "midGray", background: "#808080", foreground: "#ffffff" },
	{ name: "midGrayPalette", background: "#777777", foreground: "#000000", palette: xterm },
	{ name: "dimForeground", background: "#1e1e1e", foreground: "#5a5a5a", palette: frappe },
];
const out = cases.map((c) => {
	const input = { background: hex(c.background), saturation: 1 };
	if (c.foreground) input.foreground = hex(c.foreground);
	if (c.palette) input.palette = c.palette.map(hex);
	const { colors, appearance } = generateSystemThemeColors(input);
	return { ...c, appearance, colors };
});
writeFileSync(new URL("../internal/tui/testdata/pi-system-theme.json", import.meta.url), JSON.stringify(out, null, 1) + "\n");
console.log(out.map((c) => `${c.name}:${c.appearance} accent=${c.colors.accent} text=${c.colors.text}`).join("\n"));
