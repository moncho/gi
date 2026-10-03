// Golden data for gi's port of Pi's HTML export
// (internal/sessionexport/html.go, internal/tui/export_theme.go): Pi's
// exportFromFile on testdata/pi-session.jsonl, Pi's resolved theme colours
// and export colours for built-in and custom themes, and JavaScript's
// String.replace substitutions that Pi's page assembly applies.
//   COLORTERM=truecolor bun scripts/golden-export-html.mjs [node_modules dir]
// It also refreshes gi's copy of Pi's template (internal/sessionexport/template).
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { homedir, tmpdir } from "node:os";
import { join } from "node:path";

const candidates = [process.argv[2], process.env.PI_NODE_MODULES, `${homedir()}/.bun/install/global/node_modules`, "/workspace/.cache/pi-ref/node_modules"];
const root = candidates.find((dir) => dir && existsSync(`${dir}/@earendil-works/pi-coding-agent/dist/index.js`));
if (!root) throw new Error("pi-coding-agent not found");
const dist = `${root}/@earendil-works/pi-coding-agent/dist`;
const themeDir = `${dist}/modes/interactive/theme`;
const templateDir = `${dist}/core/export-html`;
process.env.COLORTERM = "truecolor";
const agentDir = mkdtempSync(join(tmpdir(), "pi-export-"));
process.env.PI_CODING_AGENT_DIR = agentDir;
const theme = await import(`${themeDir}/theme.js`);
const { validateThemeJson } = await import(`${themeDir}/theme-json.js`);
const { exportFromFile } = await import(`${templateDir}/index.js`);
theme.setThemeJsonValidator(validateThemeJson);

// gi's copy of the template.
for (const file of ["template.html", "template.css", "template.js", "vendor/marked.min.js", "vendor/highlight.min.js"]) {
	copyFileSync(`${templateDir}/${file}`, `internal/sessionexport/template/${file}`);
}

const dark = JSON.parse(readFileSync(`${themeDir}/dark.json`, "utf8"));
const light = JSON.parse(readFileSync(`${themeDir}/light.json`, "utf8"));
// ocean: every colour form, and export colours from vars, okhsl and an index.
const ocean = structuredClone(dark);
ocean.name = "ocean";
ocean.vars = { ...ocean.vars, base: "#112233", alias: "base", deep: 24 };
Object.assign(ocean.colors, {
	accent: "#0af",
	border: 75,
	text: "",
	mdLink: "oklch(70% 0.1 200)",
	success: "okhsl(140deg 60% 60%)",
	toolTitle: "alias",
	borderMuted: "deep",
	selectedBg: "",
	userMessageBg: 236,
});
delete ocean.colors.thinkingMax;
delete ocean.colors.searchMatchBg;
ocean.export = { pageBg: "alias", cardBg: "okhsl(200 50% 20%)", infoBg: 24 };
// plain: no appearance (detected), no export section, a light userMessageBg.
const plain = structuredClone(light);
plain.name = "plain";
delete plain.appearance;
delete plain.export;
Object.assign(plain.colors, { text: "", toolPendingBg: "", userMessageBg: "#f0f0f0" });
// partial: export colours partly set, oklch kept as written.
const partial = structuredClone(dark);
partial.name = "partial";
partial.export = { pageBg: "oklch(30% 0.02 250)", cardBg: "" };
const files = { ocean, plain, partial };
mkdirSync(join(agentDir, "themes"));
for (const [file, json] of Object.entries(files)) writeFileSync(join(agentDir, "themes", `${file}.json`), JSON.stringify(json, null, 2));

// Each theme's colours, export colours and the CSS variables block Pi writes.
const themes = {};
for (const name of ["dark", "light", "ocean", "plain", "partial"]) {
	const file = join(agentDir, `${name}.html`);
	await exportFromFile("internal/sessionexport/testdata/pi-session.jsonl", { outputPath: file, themeName: name });
	const css = readFileSync(file, "utf8");
	const start = css.indexOf(":root {");
	const root = css.slice(start, css.indexOf("}", start) + 1);
	themes[name] = { colors: Object.entries(theme.getResolvedThemeColors(name)), export: theme.getThemeExportColors(name), root };
}

// The system theme, generated from the terminal's reported colours.
theme.setTerminalColors({ background: { r: 30, g: 30, b: 46 }, foreground: { r: 205, g: 214, b: 244 } });
themes.system = { colors: Object.entries(theme.getResolvedThemeColors("system")), export: theme.getThemeExportColors("system"), terminal: { background: [30, 30, 46], foreground: [205, 214, 244] } };
theme.setTerminalColors({});

// Pi's page for the fixture session in the ocean theme. The inlined
// libraries are replaced by markers (as String.replace inserted them) and the
// session data is decoded.
const output = join(agentDir, "out.html");
await exportFromFile("internal/sessionexport/testdata/pi-session.jsonl", { outputPath: output, themeName: "ocean" });
let page = readFileSync(output, "utf8");
const inserted = (pattern, file) => pattern.replace(pattern, readFileSync(`${templateDir}/${file}`, "utf8"));
for (const [marker, file] of [["{{JS}}", "template.js"], ["{{MARKED_JS}}", "vendor/marked.min.js"], ["{{HIGHLIGHT_JS}}", "vendor/highlight.min.js"]]) {
	const body = inserted(marker, file);
	if (page.split(body).length !== 2) throw new Error(`${file} not found once`);
	page = page.replace(body, () => marker);
}
const match = page.match(/id="session-data"[^>]*>([A-Za-z0-9+/=]+)</);
if (!match) throw new Error("session data not found");
const sessionData = JSON.parse(Buffer.from(match[1], "base64").toString("utf8"));
page = page.replace(match[1], () => "{{SESSION_DATA}}");

const replacements = ["plain", "1$$2", "a$&b", "x$`y", "p$'q", "n$1$0$<g>", "end$"].map((replacement) => [replacement, "L{{X}}R".replace("{{X}}", replacement)]);

const out = { themes, session: { theme: "ocean", page, sessionData }, replacements, themeFiles: files };
writeFileSync("internal/sessionexport/testdata/pi-export-html.json", JSON.stringify(out, null, 1) + "\n");
console.log(Object.keys(themes), page.length, sessionData.entries.length);

// gi's built-in theme colours for exports, as Pi resolves them.
const q = (s) => JSON.stringify(s);
let go = `// Code generated by scripts/golden-export-html.mjs from Pi ${JSON.parse(readFileSync(`${dist}/../package.json`, "utf8")).version}; DO NOT EDIT.

package tui

import "github.com/rcarmo/gi/internal/sessionexport"

// piBuiltinExportThemes are Pi's built-in themes as its HTML export writes
// them: resolved colours in the theme's order and the explicit export colours.
var piBuiltinExportThemes = map[string]sessionexport.Theme{
`;
for (const name of ["dark", "light"]) {
	const t = themes[name];
	go += `\t${q(name)}: {\n\t\tColors: [][2]string{\n`;
	for (const [key, value] of t.colors) go += `\t\t\t{${q(key)}, ${q(value)}},\n`;
	go += `\t\t},\n\t\tPageBg: ${q(t.export.pageBg ?? "")}, CardBg: ${q(t.export.cardBg ?? "")}, InfoBg: ${q(t.export.infoBg ?? "")},\n\t},\n`;
}
writeFileSync("internal/tui/pi_export_themes_gen.go", go + "}\n");
