// Golden data for gi's port of Piclaw's keychain (internal/keychain):
// entries sealed by Piclaw's own keychain code (gi must open them), its
// shell variable names, the variables a command's text names and its
// keychain: placeholder substitutions.
//   bun scripts/golden-keychain.mjs [piclaw runtime dir]
import { writeFileSync } from "node:fs";

const runtime = process.argv[2] || "/opt/piclaw/current/app/runtime";
process.env.PICLAW_DB_IN_MEMORY = "1";
process.env.PICLAW_KEYCHAIN_KEY = "golden-master";
const { initDatabase, getDb } = await import(`${runtime}/src/db/connection.ts`);
const kc = await import(`${runtime}/src/secure/keychain.ts`);
initDatabase();

const entries = [
	{ name: "fixtures/kc-x.v1", type: "basic", secret: "s3cret", username: "octo" },
	{ name: "STRIPE_KEY", type: "token", secret: "sk_live" },
	{ name: "lower_case", type: "secret", secret: "lc-value" },
	{ name: "a-b", type: "secret", secret: "first" },
	{ name: "a.b", type: "secret", secret: "second" },
	{ name: "9lives", type: "password", secret: "cat" },
	{ name: "ns:inner", type: "secret", secret: "colon-secret", username: "colon-user" },
	{ name: "weird name!", type: "secret", secret: "w" },
	{ name: "quote's", type: "secret", secret: "it's $HOME `x` \"q\"" },
];
for (const e of entries) await kc.setKeychainEntry(e);
const rows = getDb().prepare("SELECT name, type, ciphertext, nonce, salt, kdf, kdf_iterations FROM keychain_entries ORDER BY name").all()
	.map((r) => ({ name: r.name, type: r.type, ciphertext: Buffer.from(r.ciphertext).toString("base64"), nonce: Buffer.from(r.nonce).toString("base64"), salt: Buffer.from(r.salt).toString("base64"), kdf: r.kdf, kdf_iterations: r.kdf_iterations }));

const names = ["fixtures/kc-x.v1", "a//b..c", "-lead", "9abc", "x9", "", "///", "é-accent", "with space", "under_score", "MiXeD.case", "a:b"];
const envNames = Object.fromEntries(names.map((n) => [n, kc.toShellEnvName(n)]));

const commands = [
	'echo "$FIXTURES_KC_X_V1"',
	"echo ${FIXTURES_KC_X_V1}",
	"echo $env:STRIPE_KEY",
	"echo %lower_case%",
	"echo $A_B $WEIRDNAME",
	"v=STRIPE_KEY; echo ${!v}",
	"printenv",
	"echo $FIXTURES_KC_X_V1_SUFFIX",
	"echo keychain:fixtures/kc-x.v1",
	"echo keychain:fixtures/kc-x.v1:username keychain:fixtures/kc-x.v1:user",
	"echo keychain:fixtures/kc-x.v1:secret keychain:fixtures/kc-x.v1:password keychain:fixtures/kc-x.v1:token",
	"curl -H 'Authorization: Bearer keychain:STRIPE_KEY' https://x",
	"echo keychain:ns:inner keychain:ns:inner:user",
	"echo keychain: keychain:",
	"echo keychain:missing",
	"echo keychain:STRIPE_KEY:username",
	"echo keychain:fixtures/kc-x.v1:bogus",
	"echo keychain:STRIPE_KEY:keychain:lower_case",
	"echo 'keychain:quote's'",
];
const shell = [];
for (const command of commands) {
	const env = await kc.buildInjectedShellEnv({ includeProcessEnv: false, referencedTexts: [command] });
	let resolved = null, error = null;
	try { resolved = await kc.resolveKeychainPlaceholders(command); } catch (e) { error = e.message; }
	shell.push({ command, env, resolved, error });
}

const out = { key: "golden-master", entries, rows, injectable: kc.listInjectableKeychainEntries(), envNames, shell };
writeFileSync("internal/keychain/testdata/piclaw-keychain.json", JSON.stringify(out, null, 1) + "\n");
console.log(rows.length, shell.length);
