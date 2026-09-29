// Installed Piclaw 3.2.4 Classic direct deletion with a disposable timeline/DELETE fixture.
// Checks the single-post path Gi already implements; no live backend or reply graph.
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
 let release;const ack=new Promise(resolve=>release=resolve),requests=[];let posts=[{id:741,chat_jid:state.sessionId,timestamp:state.now,data:{type:'user_message',content:'Disposable direct-delete row'}}];
 try{
  const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'direct-delete',revision:0,models:[],sessions:[]}}));
  await page.route('**/timeline?*',r=>r.fulfill({json:{posts,has_more:false}}));
  await page.route('**/post/741?*',async r=>{const u=new URL(r.request().url());requests.push({method:r.request().method(),cascade:u.searchParams.get('cascade'),chatJid:u.searchParams.get('chat_jid')});await ack;posts=[];await r.fulfill({json:{ids:[741],deleted:1}});});
  await page.goto(host.origin);await host.connected();
  const post=page.locator('#post-741');await post.waitFor({state:'visible'});
  const draft=page.locator('.compose-box textarea');await draft.fill('retained direct-delete draft');
  const clicks=[];page.on('dialog',dialog=>{clicks.push(dialog.message());void dialog.dismiss();});
  await post.locator('.post-delete-btn').click();
  for(let i=0;i<60&&requests.length===0;i++)await page.waitForTimeout(25);
  assert.deepEqual(requests,[{method:'DELETE',cascade:'false',chatJid:state.sessionId}]);
  assert.equal(await post.isVisible(),true);assert.equal((await post.getAttribute('class')).includes('removing'),false);
  assert.deepEqual(clicks,[],'single post should not prompt for cascade');
  release();await page.waitForFunction(()=>document.querySelector('#post-741')?.classList.contains('removing')===true,{timeout:2500});
  await post.waitFor({state:'detached'});assert.equal(await draft.inputValue(),'retained direct-delete draft');
  await page.reload();await page.waitForFunction(()=>document.querySelector('.timeline')!==null);assert.equal(await post.count(),0);
  cases.push({browser:browserName,viewport:viewportName,directRequest:true,noPrompt:true,heldAckDoesNotRemove:true,removingAfterAck:true,absentAfterReload:true,draftRetained:true});
 }finally{release?.();await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4 mounted Classic, disposable timeline and held DELETE ack. No production HTTP/auth, real database, search index, reply graph, or live deletion.',cases},null,2));
