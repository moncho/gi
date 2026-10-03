// Golden data for gi's port of Pi's custom themes
// (internal/tui/custom_theme.go): themes in <agent dir>/themes resolved by
// Pi's own theme code, with Pi's validation (setThemeJsonValidator).
//   COLORTERM=truecolor bun scripts/golden-custom-theme.mjs [node_modules dir]
import { existsSync, mkdirSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { homedir, tmpdir } from "node:os";
import { join } from "node:path";

const candidates = [process.argv[2], process.env.PI_NODE_MODULES, `${homedir()}/.bun/install/global/node_modules`, "/workspace/.cache/pi-ref/node_modules"];
const root = candidates.find((dir) => dir && existsSync(`${dir}/@earendil-works/pi-coding-agent/dist/index.js`));
if (!root) throw new Error("pi-coding-agent not found");
const dir = `${root}/@earendil-works/pi-coding-agent/dist/modes/interactive/theme`;
process.env.COLORTERM = "truecolor";
const agentDir = mkdtempSync(join(tmpdir(), "pi-themes-"));
process.env.PI_CODING_AGENT_DIR = agentDir;
const theme = await import(`${dir}/theme.js`);
const { validateThemeJson } = await import(`${dir}/theme-json.js`);
theme.setThemeJsonValidator(validateThemeJson);

const dark = JSON.parse(readFileSync(`${dir}/dark.json`, "utf8"));
const ocean = structuredClone(dark);
ocean.name = "ocean";
ocean.vars = { ...ocean.vars, base: "#112233", alias: "base", deep: 24 };
Object.assign(ocean.colors, {
	accent: "#0af", // three digits
	border: 75, // 256-colour index
	text: "", // the terminal's default
	mdLink: "oklch(70% 0.1 200)",
	success: "okhsl(140deg 60% 60%)",
	toolTitle: "alias", // a var naming a var
	borderMuted: "deep", // a var holding an index
	selectedBg: "",
});
delete ocean.colors.thinkingMax; // fallback: thinkingXhigh
delete ocean.colors.searchMatchBg; // fallback: selectedBg
const missing = { name: "missing", colors: { accent: "#fff", text: "#000" } };
const circular = { ...structuredClone(dark), name: "circular", vars: { a: "b", b: "a" } };
circular.colors.accent = "a";
const unknownVar = { ...structuredClone(dark), name: "unknownvar" };
unknownVar.colors.accent = "nosuchvar";
const badColor = { ...structuredClone(dark), name: "badcolor" };
badColor.colors.accent = "#12345";
const wrongType = { ...structuredClone(dark), name: "wrongtype" };
wrongType.colors.accent = true;
const slash = { ...structuredClone(dark), name: "a/b" };
const files = { ocean, missing, circular, unknownvar: unknownVar, badcolor: badColor, wrongtype: wrongType, slash };
mkdirSync(join(agentDir, "themes"));
for (const [file, json] of Object.entries(files)) writeFileSync(join(agentDir, "themes", `${file}.json`), JSON.stringify(json, null, 2));
writeFileSync(join(agentDir, "themes", "broken.json"), "{ not json");
writeFileSync(join(agentDir, "themes", "notes.txt"), "ignored");

// Every colour key gi uses, as Pi renders it: [r, g, b], an index, or "default".
const keys = Object.keys(dark.colors).sort();
const sample = (key) => {
	const s = key.endsWith("Bg") ? theme.theme.bg(key, "x") : theme.theme.fg(key, "x");
	let m = s.match(/\x1b\[[34]8;2;(\d+);(\d+);(\d+)m/);
	if (m) return m.slice(1).map(Number);
	m = s.match(/\x1b\[[34]8;5;(\d+)m/);
	if (m) return Number(m[1]);
	if (/\x1b\[[34]9m/.test(s)) return "default";
	throw new Error(`${key}: unexpected ${JSON.stringify(s)}`);
};
theme.initTheme("dark");
const out = { files: { ...files, broken: "{ not json" }, available: theme.getAvailableThemes(), themes: {} };
for (const name of ["ocean", "missing", "circular", "unknownvar", "badcolor", "wrongtype", "slash", "broken", "nosuch"]) {
	const result = theme.setTheme(name);
	out.themes[name] = result.success ? { colors: Object.fromEntries(keys.map((key) => [key, sample(key)])) } : { error: result.error.replaceAll(agentDir, "<agent>") };
}
writeFileSync("internal/tui/testdata/pi-custom-theme.json", JSON.stringify(out, null, 1) + "\n");
console.log(out.available, Object.fromEntries(Object.entries(out.themes).map(([k, v]) => [k, v.error ?? "ok"])));
