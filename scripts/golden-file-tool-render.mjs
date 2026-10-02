// Golden data for gi's ports of pi-tui's wrapTextWithAnsi and Pi's
// renderDiff (internal/tui/file_tool_render.go).
//   bun scripts/golden-file-tool-render.mjs
import { writeFileSync } from "node:fs";
import { homedir } from "node:os";
const root = `${homedir()}/.bun/install/global/node_modules/@earendil-works`;
const { wrapTextWithAnsi } = await import(`${root}/pi-tui/dist/utils.js`);
const { initTheme } = await import(`${root}/pi-coding-agent/dist/modes/interactive/theme/theme.js`);
const { renderDiff } = await import(`${root}/pi-coding-agent/dist/modes/interactive/components/diff.js`);
const edit = await import(`${root}/pi-coding-agent/dist/core/tools/edit-diff.js`);
initTheme("dark");

const wraps = [
	["short line", 20],
	["trailing spaces   ", 10],
	["the quick brown fox jumps over the lazy dog", 10],
	["averyveryverylongwordthatdoesnotfit and more", 8],
	["    indented code that wraps around", 12],
	["日本語のテキストを折り返す", 7],
	["a  b   c    d", 3],
	["func main() { fmt.Println(\"hello, world\") }", 16],
	["x".repeat(25), 10],
	["emoji 👩🏽‍💻 here and there", 9],
	["word\u00a0nbsp spaced", 6],
];
const strip = (s) => s.replace(/\x1b\[[0-9;]*m/g, "");
const diffs = [
	edit.generateDiffString("alpha\nbeta\ngamma\n", "alpha\nbeta two\ngamma\n").diff,
	edit.generateDiffString("  const x = 1;\n", "  const y = 2;\n").diff,
	edit.generateDiffString("a\nb\nc\nd\n", "a\nB\nC\nd\ne\n").diff,
	edit.generateDiffString("\tif (a) {\n\t\treturn b;\n\t}\n", "\tif (a) {\n\t\treturn c;\n\t}\n").diff,
	edit.generateDiffString(Array.from({ length: 30 }, (_, i) => `l${i}`).join("\n"), Array.from({ length: 30 }, (_, i) => (i === 3 || i === 25 ? `L${i}` : `l${i}`)).join("\n")).diff,
	"not a diff line\n+1 added",
];
// Each rendered line as runs of {text, inverse}.
function runs(line) {
	const out = [];
	let inverse = false;
	for (const part of line.split(/(\x1b\[[0-9;]*m)/)) {
		if (!part) continue;
		const m = part.match(/^\x1b\[([0-9;]*)m$/);
		if (m) {
			for (const code of m[1].split(";")) {
				if (code === "7") inverse = true;
				if (code === "27" || code === "0") inverse = false;
			}
			continue;
		}
		const last = out[out.length - 1];
		if (last && last.inverse === inverse) last.text += part;
		else out.push({ text: part, inverse });
	}
	return out;
}
const result = {
	wraps: wraps.map(([text, width]) => ({ text, width, lines: wrapTextWithAnsi(text, width).map(strip) })),
	diffs: diffs.map((diff) => ({ diff, lines: renderDiff(diff).split("\n").map(runs) })),
};
writeFileSync(new URL("../internal/tui/testdata/pi-file-tool-render.json", import.meta.url), JSON.stringify(result, null, 1) + "\n");
console.log("wraps", result.wraps.length, "diffs", result.diffs.length);
