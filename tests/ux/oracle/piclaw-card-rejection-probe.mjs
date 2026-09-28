// Installed Piclaw 3.2.4 rejected Adaptive Card action; disposable timeline/API.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const fixture=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const card=JSON.parse(await fs.readFile(new URL('../fixtures/card-rejection.json',import.meta.url),'utf8'));
const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
const posts=[{id:931,timestamp:fixture.now,chat_jid:'web:default',data:{type:'agent_response',content:'Card submission capability check',agent_id:'default',is_bot_message:true,content_blocks:[card]}}];
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,sessionId:'web:default',theme:'dark'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});let actions=0;
 try{
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'card-rejection',revision:0,models:[],sessions:[]}}));
  const cardSdk=await fs.readFile(path.join(root,'app/runtime/web/static/common/js/vendor/adaptivecards.min.js'));
  await page.route('**/static/common/js/vendor/adaptivecards.min.js',r=>r.fulfill({contentType:'application/javascript',body:cardSdk}));
  await page.route('**/timeline?*',r=>r.fulfill({json:{posts,has_more:false}}));
  await page.route('**/agent/card-action',r=>{actions++;const body=r.request().postDataJSON();assert.equal(body.card_id,card.card_id);assert.equal(body.post_id,931);assert.equal(body.action.type,'Action.Submit');return r.fulfill({status:503,json:{error:'Fixture rejected card action'}});});
  await page.goto(host.origin);await host.connected();const input=page.locator('.compose-box textarea');await input.fill('unsent card draft');

  const post=page.locator('#post-931'),container=post.locator('.adaptive-card-container');
  const answer=container.getByRole('textbox',{name:'Card answer',exact:true}),submit=container.getByRole('button',{name:'Submit answer',exact:true});
  await submit.waitFor();await answer.fill('kept card answer');await submit.click();
  await container.locator('.adaptive-card-notice-error').waitFor();
  assert.match(await container.locator('.adaptive-card-notice-error').textContent(),/Fixture rejected card action/);
  assert.equal(await container.locator('.adaptive-card-status,.adaptive-card-submission-receipt').count(),0);
  assert.equal(await answer.inputValue(),'kept card answer');assert.equal(await input.inputValue(),'unsent card draft');
  assert.equal(actions,1);assert.equal(host.calls.filter(c=>c.path==='/agent/default/message'&&c.method==='POST').length,0);
  host.assert();cases.push({browser:browserName,viewport:viewportName,rejectedNotice:true,noSuccess:true,cardInputRetained:true,actions});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed 3.2.4 shipped card UI, synthetic rejected Action.Submit; no successful admission, stored receipt, provider/auth or physical-device acceptance',cases},null,2));
