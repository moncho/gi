// Installed-source oracle for the dialog title only, not Settings behaviour.
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const map=JSON.parse(await fs.readFile(path.join(root,'app/runtime/web/static/classic/dist/app.bundle.js.map'),'utf8'));
const source=name=>{const i=map.sources.findIndex(s=>s.endsWith('/'+name));assert(i>=0,name);return map.sourcesContent[i];};
const dialog=source('components/settings-dialog.ts'),i18n=source('utils/i18n.ts');
assert(dialog.includes('class="settings-dialog-title">${t(\'settings.title\')}'));
assert(i18n.includes("'settings.title': 'Settings'"));
const adapter=await fs.readFile('web/src/gi-settings.ts','utf8');
assert(adapter.includes('id="gi-settings-title">Settings</span>'));
assert(!adapter.includes('>Gi Settings<'));
console.log(JSON.stringify({release:'3.2.4',expectedTitle:'Settings',result:'pass',scope:'Installed source-map title contract; native browser entry/dismissal tests are separate.'}));
