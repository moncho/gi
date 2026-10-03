// Golden data for gi's port of pi-tui's slash-command autocomplete with
// command argument completions (internal/tui/slash_menu.go), recorded from
// pi-tui's own Editor and CombinedAutocompleteProvider.
//   bun scripts/golden-slash-autocomplete.mjs [node_modules dir]
// Default: ~/.bun/install/global/node_modules, else /workspace/.cache/pi-ref/node_modules.
import { existsSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";

const candidates = [process.argv[2], `${homedir()}/.bun/install/global/node_modules`, "/workspace/.cache/pi-ref/node_modules"];
const root = candidates.find((dir) => dir && existsSync(`${dir}/@earendil-works/pi-tui/dist/index.js`));
if (!root) throw new Error("pi-tui not found");
const tui = await import(`${root}/@earendil-works/pi-tui/dist/index.js`);

// Pi's /mcp getArgumentCompletions (pi-coding-agent extensions/mcp/index.js),
// over two servers: one OAuth server needing sign-in, one connected stdio server.
const servers = [
	{ name: "github", oauth: true, connected: true, state: "needs sign-in" },
	{ name: "local", oauth: false, connected: true, state: "connected · 2 tools" },
];
const mcpCompletions = (prefix) => {
	const [action, server, ...rest] = prefix.trimStart().split(/\s+/);
	if (rest.length > 0) return null;
	if (server === undefined) {
		return ["login", "logout", "reconnect"].filter((item) => item.startsWith(action ?? "")).map((item) => ({ value: `${item} `, label: item }));
	}
	if (action !== "login" && action !== "logout" && action !== "reconnect") return null;
	const items = servers
		.filter((candidate) => (action === "reconnect" ? candidate.connected : candidate.oauth))
		.filter((candidate) => candidate.name.startsWith(server))
		.map((candidate) => ({ value: `${action} ${candidate.name}`, label: candidate.name, description: candidate.state }));
	return items.length > 0 ? items : null;
};
// In Pi's order: built-ins (model, copy, compact), then extension commands.
const commands = [
	{ name: "model", description: "Select model" },
	{ name: "copy", description: "Copy the last reply" },
	{ name: "compact", description: "Compact the session" },
	{ name: "mcp", description: "Manage MCP servers", getArgumentCompletions: mcpCompletions },
];

const identity = (s) => s;
const theme = new Proxy({}, { get: (_, key) => (key === "selectList" ? new Proxy({}, { get: () => identity }) : identity) });
const keys = { tab: "\t", enter: "\r", esc: "\x1b", backspace: "\x7f" };
const settle = () => new Promise((resolve) => setTimeout(resolve, 5));

// Each step is a literal string typed one character at a time, or a key name.
const scenarios = {
	actions: ["/", "mc", "p", " ", "l", "tab", "g", "enter"],
	reconnect: ["/mcp r", "tab", "l", "enter"],
	reopen_after_escape: ["/co", "esc", "m"],
	command_enter_submits: ["/cop", "enter"],
	argument_enter_completes: ["/mcp lo", "enter"],
	no_matches: ["/mcp x"],
	extra_argument: ["/mcp login github", " ", "x"],
	no_completions_command: ["/copy", " ", "a"],
	backspace_reopens: ["/mcp lo", "esc", "backspace"],
	tab_completes_command: ["/mc", "tab", "l"],
	one_letter_completion: ["/mcp login githu", "tab", "enter"],
};
const out = {};
for (const [name, steps] of Object.entries(scenarios)) {
	const editor = new tui.Editor({ requestRender() {} }, theme);
	editor.setAutocompleteProvider(new tui.CombinedAutocompleteProvider(commands, "/tmp"));
	let submitted = null;
	editor.onSubmit = (text) => { submitted = text; };
	const states = [];
	for (const step of steps) {
		for (const data of keys[step] !== undefined ? [keys[step]] : [...step]) {
			editor.handleInput(data);
			await settle();
		}
		const list = editor.autocompleteList;
		states.push({
			step,
			text: submitted ?? editor.getText(),
			submitted: submitted !== null,
			open: editor.isShowingAutocomplete(),
			items: editor.isShowingAutocomplete() && list ? list.filteredItems?.map((item) => item.label) ?? list.items.map((item) => item.label) : [],
			descriptions: editor.isShowingAutocomplete() && list ? (list.filteredItems ?? list.items).map((item) => item.description ?? "") : [],
		});
		submitted = null;
	}
	out[name] = states;
}
writeFileSync("internal/tui/testdata/pi-slash-autocomplete.json", JSON.stringify(out, null, 1) + "\n");
