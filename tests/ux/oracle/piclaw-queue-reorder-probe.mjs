// Installed Piclaw 3.2.4 shipped queue controls with isolated API fixtures.
// No backend persistence, provider execution or whole-queue parity claim.
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(root+'/VERSION','utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const fixture=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const output=path.resolve('test-results/ux-oracle/queue-reorder');await fs.mkdir(output,{recursive:true});const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,theme:'dark',sessionId:'web:default'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 const rows=[{row_id:31,content:'FIRST FOLLOWUP',timestamp:state.now},{row_id:32,content:'SECOND FOLLOWUP',timestamp:state.now},{row_id:33,content:'THIRD FOLLOWUP',timestamp:state.now}];
 let items=[...rows],moveRelease,failedMoveRelease,failedRemoveRelease,failMove=false,failRemove=false;const requests=[],queueReads=[],consoleErrors=[];
 page.on('console',message=>{if(message.type()==='error'||message.type()==='warning')consoleErrors.push(message.text());});
 const gate=new Promise(resolve=>moveRelease=resolve);
 const failedGate=new Promise(resolve=>failedMoveRelease=resolve);
 const failedRemoveGate=new Promise(resolve=>failedRemoveRelease=resolve);
 try{
  const html=(await fs.readFile(root+'/app/runtime/web/static/classic/index.html','utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'queue',revision:0,models:[],sessions:[]}}));
  await page.route('**/agent/queue-state?*',r=>{queueReads.push(items.map(item=>item.row_id));return r.fulfill({json:{items,count:items.length}});});
  await page.route('**/agent/queue-reorder',async r=>{const data=r.request().postDataJSON();requests.push({operation:'move',data});if(failMove){await failedGate;return r.fulfill({status:409,json:{error:'stale queue order'}});}await gate;const [moved]=items.splice(data.from_index,1);items.splice(data.to_index,0,moved);await r.fulfill({json:{ok:true}});});
  await page.route('**/agent/queue-remove',async r=>{const data=r.request().postDataJSON();requests.push({operation:'remove',data});if(failRemove){await failedRemoveGate;return r.fulfill({status:409,json:{error:'stale queued row'}});}items=items.filter(item=>item.row_id!==data.row_id);await r.fulfill({json:{ok:true,removed:true,count:items.length}});});
  await page.goto(host.origin);await host.connected();const input=page.locator('.compose-box textarea');await input.fill('DRAFT MUST SURVIVE');
  const queue=page.locator('.compose-queue-stack-item');await page.waitForFunction(()=>document.querySelectorAll('.compose-queue-stack-item').length===3);
  const order=async()=>queue.locator('.compose-queue-stack-text').allTextContents();
  const row=text=>queue.filter({hasText:text});
  const initial=await order();assert.deepEqual(initial,['FIRST FOLLOWUP','SECOND FOLLOWUP','THIRD FOLLOWUP']);
  await row('SECOND FOLLOWUP').getByRole('button',{name:'Move up in queue'}).click();
  await page.waitForFunction(()=>[...document.querySelectorAll('.compose-queue-stack-text')].map(e=>e.textContent).join('|')==='SECOND FOLLOWUP|FIRST FOLLOWUP|THIRD FOLLOWUP');
  const optimistic=await order();assert.deepEqual(optimistic,['SECOND FOLLOWUP','FIRST FOLLOWUP','THIRD FOLLOWUP']);
  assert.deepEqual(requests,[{operation:'move',data:{from_index:1,to_index:0,chat_jid:'web:default'}}]);
  moveRelease();await page.waitForFunction(()=>[...document.querySelectorAll('.compose-queue-stack-text')].map(e=>e.textContent).join('|')==='SECOND FOLLOWUP|FIRST FOLLOWUP|THIRD FOLLOWUP');
  await row('FIRST FOLLOWUP').getByRole('button',{name:'Cancel queued message'}).click();
  await page.waitForFunction(()=>document.querySelectorAll('.compose-queue-stack-item').length===2);
  assert.deepEqual(requests[1],{operation:'remove',data:{row_id:31,chat_jid:'web:default'}});
  // Wait past the installed 250 ms foreground refresh cooldown before the failure.
  await page.waitForTimeout(400);
  failMove=true;
  const rejected=page.waitForResponse(r=>new URL(r.url()).pathname==='/agent/queue-reorder'&&r.status()===409);
  await row('SECOND FOLLOWUP').getByRole('button',{name:'Move down in queue'}).click();
  await page.waitForFunction(()=>document.querySelectorAll('.compose-queue-stack-text').length===2);
  const optimisticFailure=await order();
  assert.deepEqual(requests[2],{operation:'move',data:{from_index:0,to_index:1,chat_jid:'web:default'}});
  assert.deepEqual(optimisticFailure,['THIRD FOLLOWUP','SECOND FOLLOWUP']);
  failedMoveRelease();await rejected;
  await page.waitForFunction(()=>[...document.querySelectorAll('.compose-queue-stack-text')].map(e=>e.textContent).join('|')==='SECOND FOLLOWUP|THIRD FOLLOWUP');
  const afterRejected=await order();
  assert.deepEqual(afterRejected,['SECOND FOLLOWUP','THIRD FOLLOWUP']);
  assert.deepEqual(items.map(item=>item.content),['SECOND FOLLOWUP','THIRD FOLLOWUP']);
  assert.equal(queueReads.length,2);
  assert(consoleErrors.some(text=>text.includes('Failed to persist queue reorder')));
  await page.waitForTimeout(400);
  failRemove=true;
  const rejectedRemoval=page.waitForResponse(r=>new URL(r.url()).pathname==='/agent/queue-remove'&&r.status()===409);
  await row('SECOND FOLLOWUP').getByRole('button',{name:'Cancel queued message'}).click();
  await page.waitForFunction(()=>document.querySelectorAll('.compose-queue-stack-item').length===1);
  assert.deepEqual(requests[3],{operation:'remove',data:{row_id:32,chat_jid:'web:default'}});
  failedRemoveRelease();await rejectedRemoval;
  await page.waitForFunction(()=>[...document.querySelectorAll('.compose-queue-stack-text')].map(e=>e.textContent).join('|')==='SECOND FOLLOWUP|THIRD FOLLOWUP');
  assert.equal(queueReads.length,3);
  assert(consoleErrors.some(text=>text.includes('Failed to remove queued item')));
  assert.equal(await input.inputValue(),'DRAFT MUST SURVIVE');host.assert();
  cases.push({browser:browserName,viewport:viewportName,result:'pass',initial,optimistic,optimisticFailure,afterRejected,requests,queueReads,consoleErrors});
 }finally{moveRelease();failedMoveRelease();failedRemoveRelease();await fs.writeFile(path.join(output,'evidence.json'),JSON.stringify({scope:'Installed Piclaw3.2.4 shipped queue controls with isolated mocked API. No backend persistence, provider execution or full queue parity claim.',cases},null,2));await host.dispose();await browser.close();}
}
console.log(JSON.stringify({output,cases}));
