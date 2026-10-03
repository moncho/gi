// Golden data for gi's port of Pi's /mcp manager view
// (internal/tui/mcp_manager.go), rendered by Pi's own McpManagerView
// (pi-coding-agent extensions/mcp/ui.js): rows as plain text.
//   bun scripts/golden-mcp-manager.mjs [node_modules dir]
import { existsSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";

const candidates = [process.argv[2], `${homedir()}/.bun/install/global/node_modules`, "/workspace/.cache/pi-ref/node_modules"];
const root = candidates.find((dir) => dir && existsSync(`${dir}/@earendil-works/pi-coding-agent/dist/index.js`));
if (!root) throw new Error("pi-coding-agent not found");
const agent = `${root}/@earendil-works/pi-coding-agent/dist`;
const { initTheme, theme } = await import(`${agent}/modes/interactive/theme/theme.js`);
const { McpManagerView } = await import(`${agent}/extensions/mcp/ui.js`);
const { getKeybindings, setKeybindings } = await import(`${root}/@earendil-works/pi-tui/dist/index.js`);
const { KeybindingsManager } = await import(`${agent}/core/keybindings.js`);
initTheme("dark");
setKeybindings(new KeybindingsManager()); // the app's keys too, as in Pi (app.message.copy)

const strip = (s) => s.replace(/\x1b\[[0-9;]*m/g, "").replace(/\x1b\]8;;[^\x07\x1b]*(?:\x07|\x1b\\)/g, "").replace(/\x1b_[^\x07]*\x07/g, "");
const many = Array.from({ length: 14 }, (_, i) => ({ value: `tool_${i}`, label: `tool_${i}`, description: `Tool number ${i}` }));
const menus = {
	servers: {
		title: "MCP servers",
		error: "config: /home/u/.pi/agent/mcp.json: server \"x\": url or command required",
		items: [
			{ value: "github", label: "github", description: "needs sign-in · codemode · global" },
			{ value: "local", label: "local", description: "connected · 2 tools · direct · global" },
			{ value: "off", label: "off", description: "disabled · codemode · global" },
		],
		confirmLabel: "manage",
		cancelLabel: "close",
	},
	server: {
		title: "MCP server github",
		details: "https://api.example.com/mcp\nglobal: /home/u/.pi/agent/mcp.json\nState: failed",
		error: "initialize: connection refused",
		items: [
			{ value: "reconnect", label: "Reconnect" },
			{ value: "exposure", label: "Exposure", description: "codemode" },
			{ value: "disable", label: "Disable", description: "saved to the global mcp.json" },
		],
		selected: "exposure",
		confirmLabel: "select",
		cancelLabel: "back",
	},
	gone: { title: "gone", items: [], empty: "This server is no longer configured.", confirmLabel: "", cancelLabel: "back" },
	scrolled: { title: "Tools of big", details: "Exposure direct: declared to the model like built-in tools", items: many, selected: "tool_13", confirmLabel: "back", cancelLabel: "back" },
};
const out = { menus: {}, status: {}, signin: {} };
for (const width of [70, 40]) {
	for (const [name, menu] of Object.entries(menus)) {
		const view = new McpManagerView({ requestRender() {} }, theme, getKeybindings());
		void view.menu(() => menu);
		(out.menus[name] ??= {})[width] = { menu, rows: view.render(width).map((line) => strip(line).trimEnd()) };
	}
	const view = new McpManagerView({ requestRender() {} }, theme, getKeybindings());
	view.status("MCP server github", "Reconnecting…");
	out.status[width] = view.render(width).map((line) => strip(line).trimEnd());
	// The sign-in screen (Pi 1.0.1): the URL with click and copy hints, and
	// the redirect URL input, before and after typing.
	const signin = new McpManagerView({ requestRender() {} }, theme, getKeybindings());
	signin.focused = true;
	const url = "https://auth.example.com/authorize?client_id=gi&state=abc123&code_challenge=xyz";
	void signin.redirectUrl("Sign in to github", url, new AbortController().signal);
	const rows = () => signin.render(width).map((line) => strip(line).trimEnd());
	out.signin[width] = { url, empty: rows() };
	for (const ch of "http://x") signin.handleInput(ch);
	out.signin[width].typed = rows();
}
writeFileSync("internal/tui/testdata/pi-mcp-manager.json", JSON.stringify(out, null, 1) + "\n");
