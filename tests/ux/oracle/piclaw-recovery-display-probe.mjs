// Installed Piclaw 3.2.4 recovery visibility against disposable timeline rows.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const fixture=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const controls=JSON.parse(await fs.readFile(new URL('../fixtures/recovery-controls.json',import.meta.url),'utf8'));
const placeholders=JSON.parse(await fs.readFile(new URL('../fixtures/recovery-placeholders.json',import.meta.url),'utf8'));
const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
const posts=[
 ...controls.map((c,i)=>({id:1001+i,timestamp:fixture.now,chat_jid:'web:default',data:{type:'agent_response',content:`Recovery probe ${c.id}`,agent_id:'default',is_bot_message:true,content_blocks:c.fields===null?[]:[{type:'control_intent',intent:'protected_recovery_continuation',schema_version:1,source_message_id:'source',source_row_id:1,thread_id:1,...c.fields}]}})),
 ...placeholders.map((c,i)=>({id:1101+i,timestamp:fixture.now,chat_jid:'web:default',data:{type:c.role==='user'?'user_message':'agent_response',content:c.content,agent_id:'default',is_bot_message:c.role!=='user',content_blocks:c.blocks}})),
];
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,sessionId:'web:default',theme:'dark'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 try{
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'recovery-display',revision:0,models:[],sessions:[]}}));
  await page.route('**/timeline?*',r=>r.fulfill({json:{posts,has_more:false}}));
  const cardSdk=await fs.readFile(path.join(root,'app/runtime/web/static/common/js/vendor/adaptivecards.min.js'));
  await page.route('**/static/common/js/vendor/adaptivecards.min.js',r=>r.fulfill({contentType:'application/javascript',body:cardSdk}));
  await page.goto(host.origin);await host.connected();await page.locator('#post-1004').waitFor();
  const mismatches=[];
  for(let i=0;i<controls.length;i++){
   const c=controls[i],visible=await page.locator(`#post-${1001+i}`).count();
   if(visible!==(c.hidden?0:1))mismatches.push({kind:'control',id:c.id,expected:c.hidden?0:1,visible});
  }
  for(let i=0;i<placeholders.length;i++){
   const c=placeholders[i],visible=await page.locator(`#post-${1101+i}`).count();
   if(visible!==(c.hidden?0:1))mismatches.push({kind:'placeholder',id:c.id,expected:c.hidden?0:1,visible});
  }
  host.assert();cases.push({browser:browserName,viewport:viewportName,controls:controls.length,placeholders:placeholders.length,mismatches});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed 3.2.4 shipped UI, disposable recovery posts; native Gi fixtures reused for comparison, not parity acceptance. No backend persistence, recovery execution or physical device',cases},null,2));
const expectedDifferences=['control:legacy-partial-failure','control:legacy-bad-failure','placeholder:file','placeholder:file-ref','placeholder:message-ref','placeholder:attachment-ref','placeholder:resource','placeholder:text-annotation'];
for(const c of cases)assert.deepEqual(c.mismatches.map(m=>`${m.kind}:${m.id}`).sort(),[...expectedDifferences].sort(),`${c.browser}/${c.viewport}: unexpected visibility drift`);
