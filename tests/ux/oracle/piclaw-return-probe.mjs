// Installed shipped UI, isolated queue reads/removal; no native Gi helper involved.
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';assert.equal((await fs.readFile(root+'/VERSION','utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const fixture=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const output=path.resolve('test-results/ux-oracle/queue-return');await fs.mkdir(output,{recursive:true});const cases=[];
for(const [browserName,type]of Object.entries({chromium,webkit}))for(const [viewportName,viewport]of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,theme:'dark',sessionId:'web:default'};const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 const content='returned instruction\n\nFiles:\n- queued-file.txt\n\nReferenced messages:\n- message:41';let items=[{row_id:42,content,timestamp:state.now,thread_id:null}];let atRemoval=null,requests=0;
 try{
  const html=(await fs.readFile(root+'/app/runtime/web/static/classic/index.html','utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'return',revision:0,models:[],sessions:[]}}));
  await page.route('**/agent/queue-state?*',r=>r.fulfill({json:{items,count:items.length}}));
  await page.route('**/agent/queue-remove',async r=>{requests++;assert.equal(r.request().postDataJSON().row_id,42);atRemoval=await page.locator('.compose-box textarea').inputValue();items=[];await r.fulfill({json:{status:'ok',removed:true,count:0}});});
  await page.goto(host.origin);await host.connected();const input=page.locator('.compose-box textarea');await input.fill('existing draft to replace');
  const removed=page.waitForResponse(r=>r.url().endsWith('/agent/queue-remove')&&r.request().method()==='POST');
  const row=page.locator('.compose-queue-stack-item').filter({hasText:'returned instruction'});await row.getByRole('button',{name:'Return queued message to editor'}).click();
  await page.waitForFunction(()=>document.querySelector('.compose-box textarea')?.value==='returned instruction');
  await page.waitForFunction(()=>!document.querySelector('.compose-queue-stack-item'));
  await removed;assert.equal(atRemoval,'returned instruction');assert.equal(requests,1);assert(await input.evaluate(e=>document.activeElement===e));assert.equal(await input.evaluate(e=>e.selectionStart),'returned instruction'.length);
  assert.equal(await page.locator('.compose-box .compose-file-pill').filter({hasText:'queued-file.txt'}).count(),1);assert.equal(await page.locator('.compose-box .compose-file-pill').filter({hasText:'msg:41'}).count(),1);
  host.assert();await page.screenshot({path:path.join(output,`${browserName}-${viewportName}.png`)});cases.push({browser:browserName,viewport:viewportName,result:'pass',atRemoval,requests});
 }finally{await fs.writeFile(path.join(output,'evidence.json'),JSON.stringify({scope:'Installed Piclaw3.2.4 shipped Return UI; isolated queue fixtures, no provider/backend durability claim.',cases},null,2));await host.dispose();await browser.close();}
}
console.log(JSON.stringify({output,cases}));
