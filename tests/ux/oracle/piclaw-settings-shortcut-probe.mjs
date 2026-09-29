// Mounted installed Piclaw 3.2.4 Classic keyboard shortcut with disposable read-only settings state.
// Browser key events; no real settings writes, platform shortcut routing or physical keyboard acceptance.
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
 const state={...base,sessionId:'web:default',theme:'dark'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 try{
  const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'settings-shortcut',revision:0,models:[],sessions:[]}}));
  await page.route('**/agent/client-perf',r=>r.fulfill({json:{ok:true}}));
  await page.route('**/agent/settings-data',r=>r.fulfill({json:{}}));
  await page.goto(host.origin);await host.connected();
  const composer=page.locator('.compose-box textarea').first(),dialog=page.locator('[data-testid="settings-dialog"]');
  await composer.waitFor();await composer.fill('retained shortcut draft');await composer.focus();
  for(let i=0;i<3;i++)await page.keyboard.press('Control+,');
  await page.waitForTimeout(180);assert.equal(await dialog.count(),0,'editable-target shortcut must not open installed Classic Settings');
  assert.equal(await composer.inputValue(),'retained shortcut draft');
  await page.evaluate(()=>{document.activeElement?.blur();document.body.setAttribute('tabindex','-1');document.body.focus();});
  assert.equal(await page.evaluate(()=>document.activeElement?.tagName),'BODY');
  for(let i=0;i<3;i++)await page.keyboard.press('Control+,');
  await dialog.waitFor({state:'visible'});assert.equal(await dialog.count(),1,'rapid shortcut must mount one installed dialog');
  assert.equal(await composer.inputValue(),'retained shortcut draft');host.assert();
  cases.push({browser:browserName,viewport:viewportName,editableBlocked:true,noneditableTripleOpenedOne:true,draftRetained:true});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Mounted installed Piclaw 3.2.4 Classic bundle; browser Control-comma from focused textarea versus noneditable body. Disposable read-only fixture; no settings write, OS shortcut arbitration or physical keyboard acceptance.',cases},null,2));
