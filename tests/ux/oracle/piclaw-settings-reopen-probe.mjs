// Mounted installed Piclaw 3.2.4 Classic Settings with disposable settings-data responses.
// Measures browser-fixture cached reopen only; no live config persistence or global latency SLO.
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
 let release;const gate=new Promise(resolve=>release=resolve);let reads=0;
 try{
  const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'settings-reopen',revision:0,models:[],sessions:[]}}));
  await page.route('**/agent/client-perf',r=>r.fulfill({json:{ok:true}}));
  await page.route('**/agent/settings-data',async r=>{reads++;if(reads===2)await gate;await r.fulfill({json:{composeUploadLimitMb:reads===1?64:96,workspaceUploadLimitMb:512}});});
  await page.goto(host.origin);await host.connected();
  const input=page.locator('.compose-box textarea'),dialog=page.locator('[data-testid="settings-dialog"]');await input.fill('cached settings draft');
  const open=async()=>page.evaluate(()=>window.dispatchEvent(new CustomEvent('piclaw:open-settings')));
  await open();await dialog.waitFor({state:'visible'});
  const compose=dialog.locator('input[aria-label="compose upload limit"]');await page.waitForFunction(()=>document.querySelector('[data-testid="settings-dialog"] input[aria-label="compose upload limit"]')?.value==='64');
  assert.equal(await compose.inputValue(),'64');assert.equal(reads,1);
  await page.locator('.settings-dialog-backdrop').click({position:{x:2,y:2}});await dialog.waitFor({state:'hidden'});
  const start=Date.now();await open();await dialog.waitFor({state:'visible',timeout:1000});await page.waitForFunction(()=>document.querySelector('[data-testid="settings-dialog"] input[aria-label="compose upload limit"]')?.value==='64',null,{timeout:1000});
  const visibleMs=Date.now()-start;assert(visibleMs<1000);await page.waitForFunction(()=>document.querySelector('[data-testid="settings-dialog"] .settings-nav button')?.textContent?.includes('General'));
  await page.waitForFunction(()=>document.querySelectorAll('[data-testid="settings-dialog"]').length===1);assert.equal(await compose.inputValue(),'64');
  for(let i=0;i<60&&reads<2;i++)await page.waitForTimeout(25);
  assert.equal(reads,2,'installed Classic refreshes settings-data on reopen while showing cached values');
  release();await page.waitForFunction(()=>document.querySelector('[data-testid="settings-dialog"] input[aria-label="compose upload limit"]')?.value==='96');
  assert.equal(await input.inputValue(),'cached settings draft');host.assert();
  cases.push({browser:browserName,viewport:viewportName,cachedBeforeHeldRead:true,secondRead:true,refreshApplied:true,visibleUnderOneSecond:visibleMs<1000});
 }finally{release?.();await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4 mounted Classic with disposable first/held-second settings-data responses. Fixture timing only; no live config write, network latency SLO, physical device or whole-clause acceptance.',cases},null,2));
