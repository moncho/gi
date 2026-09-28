import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';assert.equal((await fs.readFile(root+'/VERSION','utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const mapBytes=await fs.readFile(root+'/app/runtime/web/static/classic/dist/app.bundle.js.map');assert.equal(createHash('sha256').update(mapBytes).digest('hex'),reference.map.sha256);
const fixture=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const output=path.resolve('test-results/ux-oracle/compose-escape');await fs.mkdir(output,{recursive:true});const cases=[];
for(const [browserName,type]of Object.entries({chromium,webkit}))for(const [viewportName,viewport]of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const host=await installPixelHost({page,host:'piclaw',root,state:{...fixture,theme:'dark',sessionId:'web:default'},reference,allowPresenceBeacon:true});let sends=0;
 try{
  const html=(await fs.readFile(root+'/app/runtime/web/static/classic/index.html','utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'escape',revision:0,models:[],sessions:[]}}));
  await page.route('**/agent/default/message?*',r=>{sends++;return r.fulfill({status:500,json:{error:'unexpected send'}})});
  await page.goto(host.origin);await host.connected();const input=page.locator('.compose-box textarea');await input.fill('keep Escape draft Ω');await input.press('Escape');
  assert.equal(await input.evaluate(e=>document.activeElement===e),false);assert.equal(await input.inputValue(),'keep Escape draft Ω');
  await input.focus();await input.fill('/mo');await page.locator('.slash-autocomplete').waitFor();await input.press('Escape');assert.equal(await page.locator('.slash-autocomplete').count(),0);assert.equal(await input.evaluate(e=>document.activeElement===e),true);assert.equal(await input.inputValue(),'/mo');
  await input.press('Escape');assert.equal(await input.evaluate(e=>document.activeElement===e),false);assert.equal(await input.inputValue(),'/mo');assert.equal(sends,0);host.assert();
  await page.screenshot({path:path.join(output,`${browserName}-${viewportName}.png`)});cases.push({browser:browserName,viewport:viewportName,result:'pass',plainEscapeBlurs:true,autocompleteFirstEscapeRetainsFocus:true,secondEscapeBlurs:true,sends});
 }finally{await fs.writeFile(path.join(output,'evidence.json'),JSON.stringify({scope:'Installed Piclaw3.2.4 bundle with isolated APIs. Focus and draft behaviour, not physical keyboard dismissal.',mapSha256:reference.map.sha256,cases},null,2));await host.dispose();await browser.close()}
}
console.log(JSON.stringify({output,cases}));
