// Installed Piclaw 3.2.4 Classic Settings in a disposable mounted browser fixture.
// Distinguishes source-level lazy component loading from emitted JS requests.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const base=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...base,sessionId:'web:default',theme:'light'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 const scripts=[];page.on('request',r=>{if(r.resourceType()==='script')scripts.push(new URL(r.url()).pathname);});
 try{
  const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'settings-pane-load',revision:0,models:[],sessions:[]}}));
  await page.route('**/agent/client-perf',r=>r.fulfill({json:{ok:true}}));
  await page.route('**/agent/settings-data',r=>r.fulfill({json:{}}));
  await page.goto(host.origin);await host.connected();
  await page.evaluate(()=>window.dispatchEvent(new CustomEvent('piclaw:open-settings')));
  const dialog=page.locator('[data-testid="settings-dialog"]');await dialog.waitFor({state:'visible'});
  const general=dialog.locator('.settings-nav button').filter({hasText:'General'}).first();await general.waitFor();
  assert((await general.getAttribute('class'))?.includes('active'));await dialog.getByText('Instance Configuration').waitFor();
  assert.equal(await dialog.locator('.model-catalogue-settings').count(),0);
  const initialScripts=[...scripts];
  const models=dialog.locator('.settings-nav button').filter({hasText:'Models'}).first();await models.click();
  await dialog.locator('.model-catalogue-settings').waitFor({state:'visible'});
  assert((await models.getAttribute('class'))?.includes('active'));
  const afterModels=[...scripts];
  await general.click();await dialog.getByText('Instance Configuration').waitFor();
  await models.click();await dialog.locator('.model-catalogue-settings').waitFor({state:'visible'});
  assert.deepEqual(scripts,afterModels,'revisiting Models must not fetch JS again');
  assert.deepEqual(afterModels,initialScripts,'installed bundled settings pane must not be misreported as a network chunk');
  host.assert();
  cases.push({browser:browserName,viewport:viewportName,generalFirst:true,modelsOnlyAfterClick:true,revisitNoAdditionalScript:true,scriptRequestsAfterOpen:initialScripts.length});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Mounted installed Piclaw 3.2.4 Classic with disposable settings reads; General then Models rendering and emitted script requests. Source caches section components, but this installed bundle emits no per-pane network chunk; no cold import, full section functionality or Gi module-graph parity.',cases},null,2));
