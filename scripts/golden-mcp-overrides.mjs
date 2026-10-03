// Golden data for gi's port of Pi's MCP project overrides
// (internal/mcp/config.go, cli.go UpdateServerConfig), computed by Pi's own
// loadMcpConfig and updateMcpServerConfig.
//   bun scripts/golden-mcp-overrides.mjs [node_modules dir]
import { existsSync, mkdirSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { homedir, tmpdir } from "node:os";
import { join } from "node:path";

const candidates = [process.argv[2], `${homedir()}/.bun/install/global/node_modules`, "/workspace/.cache/pi-ref/node_modules"];
const root = candidates.find((dir) => dir && existsSync(`${dir}/@earendil-works/pi-coding-agent/dist/index.js`));
if (!root) throw new Error("pi-coding-agent not found");
const { loadMcpConfig, updateMcpServerConfig } = await import(`${root}/@earendil-works/pi-coding-agent/dist/extensions/mcp/config.js`);

const global = {
	mcpServers: {
		docs: { url: "https://example.com/mcp", headers: { Authorization: "Bearer ${TOKEN}" }, exposure: "direct", toolExposure: { search: "deferred" } },
		fs: { command: "srv", args: ["."] },
		off: { command: "srv", enabled: false },
		other: { command: "srv" },
	},
};
const project = {
	mcpServers: {
		// Overrides: one setting each, and toolExposure replacing the global map.
		docs: { exposure: "codemode", toolExposure: { "get_*": "hidden" } },
		fs: { enabled: false },
		off: { enabled: true },
		// Rejected: no global server, a key an override cannot set, a bad value.
		ghost: { enabled: false },
		other: { enabled: false, args: ["x"] },
		bad: { command: "srv", exposure: "public" },
	},
};
const project2 = { mcpServers: { other: { exposure: "loud" } } };

const out = { global, project, cases: [] };
const load = (dir) => {
	const loaded = loadMcpConfig({ agentDir: join(dir, "agent"), cwd: join(dir, "work"), projectTrusted: true });
	const rel = (p) => p?.replace(dir, "<dir>");
	return {
		servers: loaded.servers.map((s) => ({ name: s.name, scope: s.scope, source: rel(s.source), override: rel(s.override), enabled: s.config.enabled !== false, exposure: s.config.exposure ?? "codemode", toolExposure: s.config.toolExposure ?? null })).sort((a, b) => a.name.localeCompare(b.name)),
		errors: loaded.errors.map(rel).sort(),
		projectConfig: rel(loaded.projectConfig),
	};
};
for (const [name, proj] of [["overrides", project], ["bad-exposure", project2]]) {
	const dir = mkdtempSync(join(tmpdir(), "pi-mcp-"));
	mkdirSync(join(dir, "agent"));
	mkdirSync(join(dir, "work", ".pi"), { recursive: true });
	writeFileSync(join(dir, "agent", "mcp.json"), JSON.stringify(global, null, 2));
	writeFileSync(join(dir, "work", ".pi", "mcp.json"), JSON.stringify(proj, null, 2));
	out.cases.push({ name, project: proj, ...load(dir) });
}
// /mcp saves: into a missing project file as a new override, onto an
// override (which keeps default values), and onto a definition.
const updates = [];
const dir = mkdtempSync(join(tmpdir(), "pi-mcp-"));
const file = join(dir, "mcp.json");
const step = (label, path, name, patch, override) => {
	let error = null;
	try {
		updateMcpServerConfig(path, name, patch, { override });
	} catch (e) {
		error = e.message.replace(dir, "<dir>");
	}
	updates.push({ label, name, patch, override, error, text: existsSync(path) ? readFileSync(path, "utf8") : null });
};
step("new override in a missing file", file, "fs", { enabled: false }, true);
step("enable an override keeps the key", file, "fs", { enabled: true }, true);
step("codemode on an override keeps the key", file, "fs", { exposure: "codemode" }, true);
step("missing without override", file, "docs", { enabled: false }, false);
writeFileSync(file, '{\n\t"mcpServers": {\n\t\t"fs": {"enabled": true},\n\t\t"def": {"command": "srv", "enabled": false, "exposure": "direct"}\n\t}\n}\n');
step("enable a definition removes the key", file, "def", { enabled: true }, false);
step("codemode on a definition removes the key", file, "def", { exposure: "codemode" }, false);
step("second override appended", file, "docs", { exposure: "deferred" }, true);
out.updates = updates;
writeFileSync("internal/mcp/testdata/pi-mcp-overrides.json", JSON.stringify(out, null, 1) + "\n");
