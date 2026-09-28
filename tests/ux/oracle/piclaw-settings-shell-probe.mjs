// Installed Piclaw 3.2.4 Classic Settings shell with disposable API responses.
// Tests open/dismiss and section loading only, not settings writes or live auth.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';

const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const fixture=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
const results=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,sessionId:'web:default',theme:'light'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 let resolveSettings;const settingsGate=new Promise(resolve=>resolveSettings=resolve);
 const settingsRequests=[];
 try{
  await page.route(host.origin+'/',route=>route.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',route=>route.fulfill({json:{scope:'settings-shell',revision:0,models:[],sessions:[]}}));
  await page.route('**/agent/client-perf',route=>route.fulfill({json:{ok:true}}));
  await page.route('**/agent/settings-data',async route=>{settingsRequests.push(route.request().url());await settingsGate;await route.fulfill({json:{}});});
  await page.goto(host.origin);await host.connected();
  const input=page.locator('.compose-box textarea');await input.fill('unsent settings draft');
  await page.evaluate(()=>window.dispatchEvent(new CustomEvent('piclaw:open-settings')));
  const dialog=page.locator('[data-testid="settings-dialog"]');await dialog.waitFor();
  assert.equal(await dialog.count(),1);
  await page.waitForFunction(()=>document.querySelectorAll('[data-testid="settings-dialog"]').length===1);
  // The dialog module is already bundled by the installed release. A held
  // settings-data response leaves General interactive; it does not expose
  // the loader's import-time shell in this ordinary open sequence.
  assert.equal(await dialog.locator('.settings-dialog-loading-shell').count(),0);
  const general=dialog.locator('.settings-nav button').filter({hasText:'General'}).first();await general.waitFor();
  assert((await general.getAttribute('class'))?.includes('active'));
  const nav=await dialog.locator('.settings-nav button').allTextContents();
  assert(nav.includes('Appearance')&&nav.includes('Compaction')&&nav.includes('Providers'));
  assert.equal(settingsRequests.length,1);
  assert((await dialog.innerText()).includes('Identity'),'General renders while settings-data is held');
  resolveSettings();
  await page.keyboard.press('Escape');await dialog.waitFor({state:'hidden'});
  assert.equal(await input.inputValue(),'unsent settings draft');
  // Reopen, then dismiss by the actual backdrop without dispatching into
  // the workspace. This is a browser-fixture observation, not a server write.
  await page.evaluate(()=>window.dispatchEvent(new CustomEvent('piclaw:open-settings')));
  await dialog.waitFor();await page.locator('.settings-dialog-backdrop').click({position:{x:2,y:2}});await dialog.waitFor({state:'hidden'});
  assert.equal(await input.inputValue(),'unsent settings draft');
  host.assert();results.push({browser:browserName,viewport:viewportName,nav:nav.slice(0,8),settingsRequests:settingsRequests.length,result:'pass'});
 }finally{resolveSettings();await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4 shipped Settings shell, disposable reads; no settings mutation/live auth/physical device',cases:results},null,2));
