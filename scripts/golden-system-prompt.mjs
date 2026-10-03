// Golden data for gi's port of Pi's system prompt sections
// (internal/prompt), computed by Pi's own code.
//   bun scripts/golden-system-prompt.mjs
import { writeFileSync } from "node:fs";
import { homedir } from "node:os";
import { existsSync as piModulesExist } from "node:fs";
// Pi's packages: PI_NODE_MODULES, the global bun install, or the reference install.
const PI_MODULES = [process.env.PI_NODE_MODULES, `${homedir()}/.bun/install/global/node_modules`, "/workspace/.cache/pi-ref/node_modules"].find((dir) => dir && piModulesExist(`${dir}/@earendil-works/pi-coding-agent`));
const root = `${PI_MODULES}/@earendil-works`;
const sp = await import(`${root}/pi-coding-agent/dist/core/system-prompt.js`);
const { getCurrentSystemMessage } = await import(`${root}/pi-ai/dist/utils/transcript.js`);
const { getSystemMessageText } = await import(`${root}/pi-ai/dist/utils/text.js`);

const full = {
	selectedTools: ["read", "bash", "edit", "write", "codemode"],
	toolSnippets: { read: "Read file contents", bash: "Execute bash commands (ls, grep, find, etc.)", edit: "Make precise file edits", write: "Create or overwrite files" },
	toolGuidelines: { read: ["Use read to examine files instead of cat or sed."], edit: ["Use edit for precise changes", "  Use edit for precise changes  "], write: ["Use write only for new files or complete rewrites."], codemode: ["Prefer codemode"] },
	promptGuidelines: ["Be concise in your responses", "Custom rule"],
	appendSystemPrompt: "Extra instructions.",
	contextFiles: [{ path: "/w/AGENTS.md", content: "# Project\nRules here." }],
	skills: [{ name: "deploy", description: "Deploy <apps> & \"things\"", filePath: "/w/.pi/skills/deploy/SKILL.md", disableModelInvocation: false }],
	cwd: "C:\\work\\proj",
	sections: { mcp_servers: "Server list" },
};
const cases = {
	full,
	custom: { ...full, customPrompt: "Custom preamble." },
	bashOnly: { selectedTools: ["bash"], cwd: "/w", skills: full.skills },
	noTools: { selectedTools: [], cwd: "/w" },
	withGrep: { selectedTools: ["bash", "grep"], toolSnippets: { grep: "Search" }, cwd: "/w" },
};
const out = { cases: {}, diffs: [], replays: [] };
for (const [name, input] of Object.entries(cases)) {
	out.cases[name] = { input, sections: Object.entries(sp.buildSystemPromptSections(input)).map(([n, text]) => ({ name: n, text })) };
}
const a = sp.buildSystemPromptSections(full);
const b = sp.buildSystemPromptSections({ ...full, selectedTools: ["read", "write"], sections: {} , appendSystemPrompt: "" });
const toList = (o) => Object.entries(o).map(([name, text]) => ({ name, text }));
const patch = sp.diffSystemPromptSections(a, b);
out.diffs.push({ previous: toList(a), current: toList(b), patch: Object.entries(patch).map(([name, text]) => ({ name, text })) });
// Replay: initial sections, then the patch, then re-adding a removed section.
const msgs = [
	{ role: "system", content: "", sections: a, timestamp: 0 },
	{ role: "user", content: "hi", timestamp: 1 },
	{ role: "system", content: "", sections: patch, timestamp: 2 },
	{ role: "system", content: "", sections: { mcp_servers: "<mcp_servers>\nback\n</mcp_servers>" }, timestamp: 3 },
];
const current = getCurrentSystemMessage(msgs);
out.replays.push({ initial: toList(a), patches: [Object.entries(patch).map(([name, text]) => ({ name, text })), [{ name: "mcp_servers", text: "<mcp_servers>\nback\n</mcp_servers>" }]], sections: toList(current.sections), text: getSystemMessageText(current) });
writeFileSync(new URL("../internal/prompt/testdata/pi-system-prompt.json", import.meta.url), JSON.stringify(out, null, 1) + "\n");
console.log(Object.keys(out.cases).join(" "), "patch", Object.keys(patch).join(","));
