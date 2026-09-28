// Installed Piclaw 3.2.4 Classic UI + real theme handler, backed by an
// in-memory DB and temporary config. No live HTTP server, agent or chat writes.
import assert from 'node:assert/strict';
import {readFile,mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import path from 'node:path';
import {pathToFileURL} from 'node:url';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';

assert.equal(process.env.PICLAW_DB_IN_MEMORY,'1','set PICLAW_DB_IN_MEMORY=1');
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const reference=JSON.parse(await readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const fixture=JSON.parse(await readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const workspace=await mkdtemp(path.join(tmpdir(),'piclaw-theme-combined-'));
process.env.PICLAW_WORKSPACE=workspace;
const source=file=>pathToFileURL(path.join(root,'app/runtime/src',file)).href;
const {initDatabase,getDb,closeDatabase}=await import(source('db/connection.ts'));
const {extensionKvGet}=await import(source('db/extension-kv.ts'));
const {getServerUiThemeConfig}=await import(source('channels/web/ui-state.ts'));
const {handleAgentMessage}=await import(source('channels/web/handlers/agent.ts'));
const html=(await readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
const cases=[];
try{
 initDatabase();assert.equal(getDb().query('PRAGMA database_list').all().find(row=>row.name==='main').file,'');
 for(const [browserName,type] of Object.entries({chromium,webkit})){
  const browser=await type.launch({headless:true}),page=await browser.newPage({viewport:{width:1440,height:900},serviceWorkers:'block'});
  const state={...fixture,sessionId:'web:default',theme:'light'};
  const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
  let timeline=[],nextId=300,agentRuns=0;const events=[],requests=[];
  const channel={
   agentPool:{isStreaming:()=>false,isActive:()=>false},getQueuedFollowupCount:()=>0,
   broadcastEvent:(type,payload)=>{events.push({type,payload});host.emit(type,payload);},
   sendMessage:async(chat,content,options)=>{
    const post={id:++nextId,chat_jid:chat,timestamp:state.now,data:{type:'agent_response',content,agent_id:'default',is_bot_message:true}};
    timeline.push(post);events.push({type:'new_post',payload:post,options});host.emit('new_post',post);
   },
   json:(body,status=200)=>Response.json(body,{status}),
   queue:{enqueue:()=>{agentRuns++;throw Error('theme command reached agent queue');}},
   processChat:async()=>{agentRuns++;throw Error('theme command reached agent');},
  };
  try{
   await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
   await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'theme-combined',revision:0,models:[],sessions:[]}}));
   await page.route('**/timeline?*',r=>r.fulfill({json:{posts:timeline,has_more:false}}));
   await page.route('**/agent/default/message?*',async route=>{
    const request=route.request(),body=request.postDataJSON();
    const response=await handleAgentMessage(channel,new Request(request.url(),{method:'POST',headers:{'Content-Type':'application/json'},body:request.postData()}),'/agent/default/message',state.sessionId,'default');
    requests.push({content:body.content,status:response.status,result:await response.json()});
    await route.fulfill({status:response.status,json:requests.at(-1).result});
   });
   await page.goto(host.origin);await host.connected();
   const input=page.locator('.compose-box textarea');await input.fill('unsent theme draft');
   const view=()=>page.evaluate(()=>{const root=document.documentElement,css=getComputedStyle(root);return{theme:root.dataset.colorTheme||'',tint:root.dataset.tint||'',storedTheme:localStorage.getItem('piclaw_theme'),storedTint:localStorage.getItem('piclaw_tint'),background:css.getPropertyValue('--bg-primary').trim(),accent:css.getPropertyValue('--accent-color').trim()};});
   const snapshots=[];
   const submit=async(command,expected)=>{
    const before=requests.length;await input.fill(command);await input.press('Enter');
    for(let n=0;n<80&&requests.length===before;n++)await page.waitForTimeout(25);
    assert.equal(requests.length,before+1);assert.equal(requests.at(-1).content,command);
    assert.equal(requests.at(-1).status,200);assert.equal(requests.at(-1).result.ui_only,true);
    await page.locator(`#post-${301+before}`).waitFor();
    await page.waitForFunction(expected);
    snapshots.push({command,appearance:await view(),status:requests.at(-1).result.command.status});
   };
   await submit('/theme ristretto',()=>document.documentElement.dataset.colorTheme==='ristretto');
   const ristretto=snapshots.at(-1).appearance;
   assert.equal(ristretto.theme,'ristretto');assert.equal(ristretto.tint,'');
   await submit('/tint orange',()=>document.documentElement.dataset.tint==='orange');
   const orange=snapshots.at(-1).appearance;
   assert.equal(orange.theme,'default');assert.equal(orange.storedTint,'orange');
   await submit('/tint off',()=>document.documentElement.dataset.tint==='');
   const untinted=snapshots.at(-1).appearance;
   assert.equal(untinted.theme,'default');assert.equal(untinted.tint,'');
   await submit('/tint $$notacolor',()=>document.querySelector('#post-304')?.textContent?.includes('Invalid tint'));
   assert.equal(snapshots.at(-1).status,'error');assert.deepEqual(snapshots.at(-1).appearance,untinted);
   assert.equal(events.filter(e=>e.type==='ui_theme').length,3);
   assert.equal(events.filter(e=>e.type==='new_post').length,4);
   assert(events.filter(e=>e.type==='new_post').every(e=>e.options?.forceRoot===true));
   assert.equal(agentRuns,0);
   assert.deepEqual(extensionKvGet('piclaw-ui','theme','global'),{theme:'default',tint:null});
   assert.equal(JSON.parse(await readFile(path.join(workspace,'.piclaw','config.json'),'utf8')).ui.tint,null);
   host.assert();cases.push({browser:browserName,snapshots,requests:requests.map(r=>({content:r.content,status:r.status,ui_only:r.result.ui_only})),result:'pass'});
  }finally{await host.dispose();await browser.close();}
 }
 console.log(JSON.stringify({scope:'Installed Piclaw handler + shipped UI with disposable browser fixture, in-memory DB and temporary config; no production HTTP, reload, agent or live chat',cases},null,2));
}finally{closeDatabase();await rm(workspace,{recursive:true,force:true});}
