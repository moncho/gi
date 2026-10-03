// Golden data for gi's port of Piclaw's messages search helpers
// (internal/store/message_timeline_search.go, internal/tools/messages_piclaw.go),
// computed by Piclaw's own code.
//   bun scripts/golden-messages-search.mjs [piclaw runtime/src dir]
// Default source: $PICLAW_SRC, else the installed Piclaw release under /opt/piclaw.
import { readFileSync, writeFileSync } from "node:fs";

const src = process.argv[2] || process.env.PICLAW_SRC || "/opt/piclaw/current/app/runtime/src";
const fts = await import(`${src}/utils/fts-query.ts`);
// escapeRegex, extractSearchTerms and buildContentExcerpt are module-private:
// evaluate their exact source text.
const crud = readFileSync(`${src}/extensions/messages-crud.ts`, "utf8");
const start = crud.indexOf("function escapeRegex(");
const end = crud.indexOf("function parseContentLines(");
if (start < 0 || end < start) throw new Error("messages-crud.ts helper anchors changed");
const body = new Bun.Transpiler({ loader: "ts" }).transformSync(crud.slice(start, end));
const { extractSearchTerms, buildContentExcerpt } = new Function(`${body}; return { extractSearchTerms, buildContentExcerpt };`)();

const queries = ["nonce", "  alpha beta  ", "#topic", "##", "*", "foo AND bar", "\"quoted phrase\" x", "(grouped) y", "col:term", "http://host/x", "-neg --dash a.b/c", "(foo) NOT bar", "naïve café", "x: y", "a_b-c! ?d"];
const fallback = queries.map((q) => ({ query: q, operator: fts.isFtsOperatorQuery(q), terms: fts.extractFtsFallbackTerms(q, { dropFtsKeywords: fts.isFtsOperatorQuery(q) }) }));
const searchTerms = queries.map((q) => ({ query: q, terms: extractSearchTerms(q) }));
const text = "The quick brown fox jumps over the lazy dog while the Nonce-42 value is logged; café naïve résumé end.";
const excerpts = [];
for (const [query, width] of [["nonce", 20], ["fox lazy", 30], ["DOG", 5], ["missing", 40], ["café", 12], ["the", 1000], ["end", 10], ["quick", 0]]) {
	const r = buildContentExcerpt(text, extractSearchTerms(query), width);
	excerpts.push({ query, width, text: r?.text ?? null, truncated: r?.truncated ?? null });
}
writeFileSync("internal/tools/testdata/piclaw-messages-search.json", JSON.stringify({ content: text, fallback, searchTerms, excerpts }, null, 1) + "\n");
