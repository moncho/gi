// The oracle has numeric message IDs; Gi adapts display only, not storage identity.
import fs from 'node:fs/promises';
import assert from 'node:assert/strict';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(root+'/VERSION','utf8')).trim(),'3.2.4');
const map=JSON.parse(await fs.readFile(root+'/app/runtime/web/static/classic/dist/app.bundle.js.map','utf8'));
const i=map.sources.findIndex(s=>s.endsWith('/components/compose-box.ts'));assert(i>=0);
const source=map.sourcesContent[i];
assert.equal(source.split("label=${'msg:' + id}").length-1,2);
assert(source.includes("title=${'Message reference: ' + id}"));
assert(source.includes('capturedMessageRefs.map((id) => `- message:${id}`)'));
console.log(JSON.stringify({result:'pass',release:'3.2.4',scope:'Installed source contract for composer/queue pill prefix, tooltip and submitted reference; Gi canonical-string versus numeric-row adapter is native-tested, not whole UX parity.'}));
