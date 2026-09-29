// Installed Piclaw 3.2.4 Classic recovered chip in a disposable assistant post.
// Compare the metadata order Gi already renders; no real retry/recovery execution or draft persistence.
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
 try{
  const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
  const posts=[{id:781,chat_jid:state.sessionId,timestamp:state.now,data:{type:'agent_response',content:'Recovered outcome fixture',is_bot_message:true,agent_id:'default',content_blocks:[{type:'recovery_marker',recovered:true,recovery_kind:'native_interrupted_turn',attempts_used:2}]}},{id:782,chat_jid:state.sessionId,timestamp:state.now,data:{type:'agent_response',content:'Ordinary outcome fixture',is_bot_message:true,agent_id:'default'}}];
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'outcome-chip',revision:0,models:[],sessions:[]}}));
  await page.route('**/timeline?*',r=>r.fulfill({json:{posts,has_more:false}}));
  await page.goto(host.origin);await host.connected();
  const post=page.locator('#post-781'),chip=post.locator('.post-recovery-chip'),time=post.locator('.post-time');await chip.waitFor({state:'visible'});
  assert.equal(await chip.textContent(),'recovered');assert.equal(await chip.getAttribute('title'),'Recovered after 2 attempts');
  assert.equal(await chip.evaluate(el=>el.previousElementSibling?.classList.contains('post-time')),true);
  const placement=await page.evaluate(()=>{const t=document.querySelector('#post-781 .post-time').getBoundingClientRect(),c=document.querySelector('#post-781 .post-recovery-chip').getBoundingClientRect();return {rowCenterDifference:Math.abs(t.y+t.height/2-c.y-c.height/2),afterTimestamp:c.x>=t.x+t.width};});
  assert(placement.rowCenterDifference<=2,`metadata row misaligned: ${JSON.stringify(placement)}`);assert.equal(placement.afterTimestamp,true);
  assert.equal(await page.locator('#post-782 .post-recovery-chip').count(),0);
  const draft=page.locator('.compose-box textarea');await draft.fill('retained outcome draft');assert.equal(await draft.inputValue(),'retained outcome draft');
  await page.reload();await chip.waitFor({state:'visible'});
  assert.equal(await chip.evaluate(el=>el.previousElementSibling?.classList.contains('post-time')),true);
  cases.push({browser:browserName,viewport:viewportName,recoveredAfterTimestamp:true,sameRow:true,tooltip:true,ordinaryNoChip:true,chipAfterReload:true});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4 mounted Classic disposable recovered/ordinary posts; metadata placement, tooltip and fixture reload only. No actual retry, Gi database recovery, live provider or whole rendering parity.',cases},null,2));
