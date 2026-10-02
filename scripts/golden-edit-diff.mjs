// Golden data for gi's ports of jsdiff (internal/textdiff) and Pi's edit
// diff (internal/tools/edit_tool.go), computed by the code Pi runs.
//   bun scripts/golden-edit-diff.mjs
import { writeFileSync } from "node:fs";
import { homedir } from "node:os";
const root = `${homedir()}/.bun/install/global/node_modules`;
const Diff = await import(`${root}/diff/libesm/index.js`);
const edit = await import(`${root}/@earendil-works/pi-coding-agent/dist/core/tools/edit-diff.js`);

const pairs = [
	["", ""],
	["a\n", "a\n"],
	["a\nb\nc\n", "a\nB\nc\n"],
	["a\nb\nc", "a\nb\nc\nd"],
	["one\ntwo\nthree\n", "zero\none\nthree\nfour\n"],
	["x\r\ny\r\n", "x\r\nz\r\n"],
	[Array.from({ length: 40 }, (_, i) => `line ${i}`).join("\n"), Array.from({ length: 40 }, (_, i) => (i % 13 === 5 ? `changed ${i}` : `line ${i}`)).join("\n")],
	["a\nb\na\nb\na\n", "b\na\nb\na\nb\n"],
	["\n\n\n", "\n\nx\n"],
	["abc", ""],
	["", "abc\n"],
];
const words = [
	["foo bar baz", "foo baz"],
	["foo bar baz", "foo qux baz"],
	["foo\nbar baz", "foo baz"],
	["foo baz", "foo\nbar baz"],
	["foo   bar baz", "foo  baz"],
	["  const x = 1;", "  const y = 2;"],
	["return a + b", "return a - b;"],
	["héllo wörld", "hello world"],
	["", "new"],
	["old", ""],
	["call(a, b)", "call(a,b, c)"],
	["\tindented\tline", "\tindented  line"],
	["a b c d e", "e d c b a"],
	["日本語 テキスト", "日本語 文字"],
	["trailing ", "trailing"],
	[" leading", "leading"],
];
const file = Array.from({ length: 30 }, (_, i) => `function f${i}() {\n\treturn ${i};\n}`).join("\n") + "\n";
const edits = [
	["single", file, [{ oldText: "\treturn 3;", newText: "\treturn 33;" }]],
	["multi", file, [{ oldText: "f1()", newText: "g1()" }, { oldText: "\treturn 20;\n}", newText: "\treturn 20;\n}\n// end" }]],
	["fuzzy", "say \u201chi\u201d   \nnext\n", [{ oldText: 'say "hi"\nnext', newText: 'say "bye"\nnext' }]],
	["insertLines", "a\nb\nc\n", [{ oldText: "b\n", newText: "b\nb2\nb3\n" }]],
	["notFound", "abc\n", [{ oldText: "xyz", newText: "q" }]],
	["notFoundMulti", "abc\n", [{ oldText: "abc", newText: "q" }, { oldText: "xyz", newText: "q" }]],
	["duplicate", "x\nx\n", [{ oldText: "x", newText: "y" }]],
	["empty", "abc", [{ oldText: "", newText: "y" }]],
	["overlap", "abcdef", [{ oldText: "abc", newText: "1" }, { oldText: "cde", newText: "2" }]],
	["noChange", "abc", [{ oldText: "abc", newText: "abc" }]],
	["crlfEdit", "one\ntwo\n", [{ oldText: "one\r\ntwo", newText: "1\r\n2" }]],
];
const result = {
	lines: pairs.map(([a, b]) => ({ old: a, new: b, changes: Diff.diffLines(a, b).map(({ value, count, added, removed }) => ({ value, count, added, removed })) })),
	words: words.map(([a, b]) => ({ old: a, new: b, changes: Diff.diffWords(a, b).map(({ value, count, added, removed }) => ({ value, count, added, removed })) })),
	diffStrings: pairs.map(([a, b]) => ({ old: a, new: b, ...edit.generateDiffString(a, b) })),
	edits: edits.map(([name, content, list]) => {
		try {
			const { baseContent, newContent } = edit.applyEditsToNormalizedContent(edit.normalizeToLF(content), list, "f.txt");
			return { name, content, edits: list, newContent, ...edit.generateDiffString(baseContent, newContent) };
		} catch (e) {
			return { name, content, edits: list, error: e.message };
		}
	}),
};
writeFileSync(new URL("../internal/textdiff/testdata/jsdiff.json", import.meta.url), JSON.stringify(result, null, 1) + "\n");
console.log("lines", result.lines.length, "words", result.words.length, "edits", result.edits.length);
