// Golden data for gi's port of Pi's MCP oauth settings and Client ID
// Metadata Documents (internal/mcp/config.go validateOAuth, oauth.go
// callbackID and pickClientMetadataDocument), computed by Pi's own
// validateMcpServerConfig, callbackId and clientMetadataDocument.
//   bun scripts/golden-mcp-cimd.mjs [node_modules dir]
import { existsSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";

const candidates = [process.argv[2], `${homedir()}/.bun/install/global/node_modules`, "/workspace/.cache/pi-ref/node_modules"];
const root = candidates.find((dir) => dir && existsSync(`${dir}/@earendil-works/pi-coding-agent/dist/index.js`));
if (!root) throw new Error("pi-coding-agent not found");
const agent = `${root}/@earendil-works/pi-coding-agent/dist`;
const { validateMcpServerConfig } = await import(`${agent}/core/mcp-servers.js`);
// callbackId and clientMetadataDocument are module-private: load a copy of
// oauth.js that exports them, next to it for its relative imports.
const copy = `${agent}/extensions/mcp/oauth.golden-${process.pid}.js`;
writeFileSync(copy, `${readFileSync(`${agent}/extensions/mcp/oauth.js`, "utf8")}\nexport { callbackId, clientMetadataDocument };\n`);
let oauth;
try {
	oauth = await import(copy);
} finally {
	rmSync(copy);
}

const settings = [
	{},
	{ clientRegistration: "dcr", clientName: "x" },
	{ clientRegistration: "cimd" },
	{ clientRegistration: "cimd", callbackUrl: "http://127.0.0.1:8123/callback" },
	{ clientRegistration: "cimd", callbackUrl: "http://localhost/callback" },
	{ clientRegistration: "cimd", callbackUrl: "http://[::1]:8123/callback" },
	{ clientRegistration: "cimd", callbackUrl: "http://127.0.0.1:8123/cb" },
	{ clientRegistration: "cimd", clientId: "abc" },
	{ clientRegistration: "cimd", clientName: "gi" },
	{ clientRegistration: "auto" },
	{ clientRegistration: 1 },
	{ clientId: 3 },
	{ clientSecret: false },
	{ callbackPort: 0 },
	{ callbackPort: 8123.5 },
	{ callbackPort: 8123, callbackUrl: "http://127.0.0.1:9000/callback" },
	{ callbackPort: 8123, callbackUrl: "http://127.0.0.1/callback" },
	{ callbackUrl: "https://127.0.0.1/callback" },
	{ callbackUrl: "http://example.com/callback" },
	{ callbackUrl: "http://127.0.0.1/callback?x=1" },
	{ callbackUrl: "http://127.0.0.1/callback#f" },
	{ callbackUrl: "http://[::1]/callback" },
	{ scope: ["a"] },
	{ clientName: "  " },
	{ authServerMetadataUrl: "https://auth.example.com/.well-known/oauth-authorization-server" },
	{ authServerMetadataUrl: "http://localhost:9000/meta" },
	{ authServerMetadataUrl: "http://auth.example.com/meta" },
	{ authServerMetadataUrl: "not a url" },
];
const validation = settings.map((oauth) => {
	const result = validateMcpServerConfig("s", { url: "https://mcp.example.com/mcp", oauth });
	return { oauth, error: typeof result === "string" ? result : null };
});
const urls = ["https://mcp.example.com/mcp", "https://MCP.Example.com:443/mcp#frag", "https://mcp.example.com", "http://localhost:8080/a/b?x=1"];
const callbackIds = urls.map((url) => ({ url, id: oauth.callbackId(url) }));
const metadata = [
	undefined,
	{ client_id_metadata_document_supported: true },
	{ client_id_metadata_document_supported: true, token_endpoint_auth_methods_supported: ["client_secret_post"] },
	{ client_id_metadata_document_supported: false, token_endpoint_auth_methods_supported: ["none"] },
	{ client_id_metadata_document_supported: true, token_endpoint_auth_methods_supported: ["none"] },
	{ client_id_metadata_document_supported: true, token_endpoint_auth_methods_supported: ["none"], authorization_response_iss_parameter_supported: true },
];
const documents = metadata.map((m) => {
	try {
		return { metadata: m ?? null, document: oauth.clientMetadataDocument("https://mcp.example.com/mcp", "http://127.0.0.1:8123/callback", m), error: null };
	} catch (e) {
		return { metadata: m ?? null, document: null, error: e.message };
	}
});
writeFileSync("internal/mcp/testdata/pi-mcp-cimd.json", JSON.stringify({ validation, callbackIds, documents }, null, 1) + "\n");
