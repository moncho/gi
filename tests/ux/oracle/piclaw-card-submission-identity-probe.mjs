// Installed Piclaw 3.2.4 submission-identity display filter; disposable posts.
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
const valid={type:'adaptive_card_submission',card_id:'card-1',source_post_id:42,submitted_at:'2026-09-28T12:00:00.000Z',action_type:'Action.Submit'};
const blocks=[valid,{...valid,card_id:'   '},{...valid,card_id:'x'.repeat(257)},{...valid,source_post_id:0},{...valid,source_post_id:Number.MAX_SAFE_INTEGER+1},{...valid,submitted_at:'invalid'},{...valid,action_type:'Action.OpenUrl'},{...valid,action_type:undefined,card_id:'legacy-card'}];
const posts=blocks.map((block,index)=>({id:901+index,timestamp:fixture.now,chat_jid:'web:default',data:{type:'agent_response',content:`Submission identity fixture ${index}`,agent_id:'default',is_bot_message:true,content_blocks:[block]}}));
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,sessionId:'web:default',theme:'dark'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 try{
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'submission-identity',revision:0,models:[],sessions:[]}}));
  await page.route('**/timeline?*',r=>r.fulfill({json:{posts,has_more:false}}));
  await page.goto(host.origin);await host.connected();await page.locator('#post-908').waitFor();
  for(let i=0;i<blocks.length;i++){
   const count=await page.locator(`#post-${901+i} .adaptive-card-submission-receipt`).count();
   assert.equal(count,i===0||i===7?1:0,`block ${i}: receipt count`);
  }
  assert.equal(host.calls.filter(c=>c.method==='POST'&&!['/workspace/visibility','/agent/push/presence'].includes(c.path)).length,0);
  host.assert();cases.push({browser:browserName,viewport:viewportName,accepted:[0,7],rejected:[1,2,3,4,5,6]});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed shipped UI, synthetic persisted submission blocks only; no real card action, stored receipt, auth or provider execution',cases},null,2));
