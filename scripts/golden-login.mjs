// Golden data for gi's port of Pi's /login and /logout UIs
// (internal/tui/login.go): Pi's own OAuthSelectorComponent and
// LoginDialogComponent, rendered as plain text.
//   bun scripts/golden-login.mjs [node_modules dir]
import { existsSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";

const candidates = [process.argv[2], `${homedir()}/.bun/install/global/node_modules`, "/workspace/.cache/pi-ref/node_modules"];
const root = candidates.find((dir) => dir && existsSync(`${dir}/@earendil-works/pi-coding-agent/dist/index.js`));
if (!root) throw new Error("pi-coding-agent not found");
const agent = `${root}/@earendil-works/pi-coding-agent/dist`;
const { initTheme } = await import(`${agent}/modes/interactive/theme/theme.js`);
const { OAuthSelectorComponent } = await import(`${agent}/modes/interactive/components/oauth-selector.js`);
const { LoginDialogComponent } = await import(`${agent}/modes/interactive/components/login-dialog.js`);
const { KeybindingsManager } = await import(`${agent}/core/keybindings.js`);
const { setKeybindings } = await import(`${root}/@earendil-works/pi-tui/dist/index.js`);
initTheme("dark");
setKeybindings(new KeybindingsManager());
process.env.PATH = ""; // showAuth opens a browser; leave none to find

const strip = (s) => s.replace(/\x1b\[[0-9;]*m/g, "").replace(/\x1b\]8;;[^\x07\x1b]*(?:\x07|\x1b\\)/g, "").replace(/\x1b_[^\x07]*\x07/g, "");
const rows = (component, width) => component.render(width).map((line) => strip(line).trimEnd());
const login = [
	{ id: "anthropic", name: "Anthropic", authType: "oauth", subscription: true, status: { type: "oauth", source: "stored credential" } },
	{ id: "anthropic", name: "Anthropic", authType: "api_key", subscription: true, status: { type: "oauth", source: "stored credential" } },
	{ id: "groq", name: "Groq", authType: "api_key", subscription: false, status: { type: "api_key", source: "environment" } },
	{ id: "openrouter", name: "OpenRouter", authType: "oauth", subscription: false },
	{ id: "mistral", name: "Mistral", authType: "api_key", subscription: false, status: { type: "api_key", source: "GROQ_API_KEY" } },
	...["a", "b", "c", "d", "e", "f"].map((x) => ({ id: `p-${x}`, name: `Provider ${x.toUpperCase()}`, authType: "api_key", subscription: false })),
];
const logout = [
	{ id: "anthropic", name: "Anthropic", authType: "oauth", subscription: true, status: { type: "oauth", source: "stored credential" } },
	{ id: "groq", name: "Groq", authType: "api_key", subscription: false, status: { type: "api_key", source: "stored credential" } },
];
const keys = { up: "\x1b[A", down: "\x1b[B", backspace: "\x7f" };
const selectors = {
	login: { mode: "login", providers: login, steps: ["down", "down", "down", "down", "down", "down", "up", ["type", "an"], "backspace", ["type", "zz"]] },
	logout: { mode: "logout", providers: logout, steps: ["down", "down"] },
	login_one_type: { mode: "login", providers: login.filter((p) => p.authType === "api_key").slice(0, 3), initial: "gr", steps: [] },
	logout_empty: { mode: "logout", providers: [], steps: [] },
	login_empty: { mode: "login", providers: [], steps: [] },
};
const out = { selectors: {}, dialogs: {} };
for (const width of [80, 40]) {
	for (const [name, s] of Object.entries(selectors)) {
		const selector = new OAuthSelectorComponent(s.mode, s.providers, () => {}, () => {}, s.initial);
		const states = [{ step: "open", rows: rows(selector, width) }];
		for (const step of s.steps) {
			if (Array.isArray(step)) for (const ch of step[1]) selector.handleInput(ch);
			else selector.handleInput(keys[step]);
			states.push({ step: Array.isArray(step) ? step.join(":") : step, rows: rows(selector, width) });
		}
		(out.selectors[name] ??= { ...s, widths: {} }).widths[width] = states;
	}
	const tui = { requestRender() {} };
	const dialogs = {};
	let d = new LoginDialogComponent(tui, "anthropic", () => {}, "Anthropic");
	d.showAuth("https://claude.ai/oauth/authorize?code=true", "Paste the code shown after signing in.");
	void d.showPrompt("Paste the authorization code:", "abc#123").catch(() => {});
	dialogs.auth_prompt = rows(d, width);
	for (const ch of "xyz") d.handleInput(ch);
	dialogs.auth_prompt_typed = rows(d, width);
	d.handleInput("\r");
	d.showProgress("Exchanging code for tokens…");
	dialogs.auth_submitted = rows(d, width);
	d = new LoginDialogComponent(tui, "github-copilot", () => {}, "GitHub Copilot");
	d.showDeviceCode({ verificationUri: "https://github.com/login/device", userCode: "ABCD-1234" });
	d.showWaiting("Waiting for authentication...");
	dialogs.device = rows(d, width);
	d = new LoginDialogComponent(tui, "groq", () => {}, "Groq");
	void d.showPrompt("Enter API key:").catch(() => {});
	dialogs.api_key = rows(d, width);
	d = new LoginDialogComponent(tui, "x", () => {}, "X");
	void d.showManualInput("Paste the redirect URL:").catch(() => {});
	dialogs.manual = rows(d, width);
	out.dialogs[width] = dialogs;
}
writeFileSync("internal/tui/testdata/pi-login.json", JSON.stringify(out, null, 1) + "\n");
