// Installed Piclaw 3.2.4 Classic NumberStepper on a mounted Compaction pane.
// Distinguishes uncommitted display from blur normalization; no settings save.
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
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'}),state={...base,sessionId:'web:default',theme:'light'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 const writes=[];let reads=0;
 page.on('request',request=>{if(!['GET','HEAD'].includes(request.method()))writes.push(new URL(request.url()).pathname);});
 try{
  const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'settings-stepper',revision:0,models:[],sessions:[]}}));
  await page.route('**/agent/client-perf',r=>r.fulfill({json:{ok:true}}));
  await page.route('**/agent/settings-data',r=>{reads++;return r.fulfill({json:{toolResultSemanticSummaryEnabled:true,toolResultSemanticSummaryMaxInputChars:12000}});});
  await page.goto(host.origin);await host.connected();
  const input=page.locator('.compose-box textarea');await input.fill('stepper draft');
  await page.evaluate(()=>window.dispatchEvent(new CustomEvent('piclaw:open-settings')));
  const dialog=page.locator('[data-testid="settings-dialog"]');await dialog.waitFor({state:'visible'});
  await dialog.locator('.settings-nav button').filter({hasText:'Compaction'}).first().click();
  const target=dialog.locator('input[aria-label="semantic summary input limit"]');
  await target.waitFor({state:'visible'});assert.equal(await target.isEnabled(),true);
  await target.fill('128000');assert.equal(await target.inputValue(),'128000');assert.equal(await target.getAttribute('aria-invalid'),null);
  assert.equal(writes.filter(x=>x==='/agent/settings-data').length,0);
  await target.fill('300000');assert.equal(await target.inputValue(),'300000');assert.equal(await target.getAttribute('aria-invalid'),'true');
  await target.blur();await page.waitForFunction(()=>document.querySelector('input[aria-label="semantic summary input limit"]')?.value==='200000');
  assert.equal(await input.inputValue(),'stepper draft');host.assert();
  cases.push({browser:browserName,viewport:viewportName,typed128000:true,invalid300000:true,blurClamped200000:true,settingsWrites:0,reads});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Mounted installed Piclaw 3.2.4 Compaction numeric text stepper and disposable settings-data; 128000 typed within the 500–200000 input-character range, 300000 normalized on blur. No save, live config persistence, policy equivalence to Gi Saved context window, or physical input.',cases},null,2));
