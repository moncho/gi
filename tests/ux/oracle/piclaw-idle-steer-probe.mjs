// Installed shipped browser + real queue action method, disposable collaborators.
import fs from 'node:fs/promises';
import path from 'node:path';
import {pathToFileURL} from 'node:url';
import assert from 'node:assert/strict';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';assert.equal((await fs.readFile(root+'/VERSION','utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const fixture=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const {WebAgentControlPlaneService}=await import(pathToFileURL(root+'/app/runtime/src/channels/web/agent/agent-control-plane-service.ts'));
const output=path.resolve('test-results/ux-oracle/idle-steer');await fs.mkdir(output,{recursive:true});const report={scope:'Installed Piclaw3.2.4 shipped UI and real queue method with in-memory storage/dispatch. No live provider or backend durability acceptance.',cases:[]};
for(const [browserName,type]of Object.entries({chromium,webkit}))for(const [viewportName,viewport]of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,theme:'dark',sessionId:'web:default'};const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 let item={rowId:41,queuedContent:'idle selected instruction',queuedAt:state.now,mediaIds:[9],threadId:7},failStore=true;const posts=[],calls=[];
 const lifecycle={async removeQueuedFollowupForAction(){const removed=item;item=null;return{removed,source:'deferred'}},getQueuedFollowupCount(){return item?1:0},prependQueuedFollowupItem(chat,row){item=row}};
 const service=new WebAgentControlPlaneService({defaultChatJid:'web:default',defaultAgentId:'default',json:(data,status=200)=>Response.json(data,{status}),queuedFollowupLifecycle:lifecycle,agentPool:{isStreaming:()=>false},getInflightMessageId:()=>null,storeMessage:(chat,text,bot,media,options)=>{calls.push({kind:'store',text,media,options});if(failStore)return null;const post={id:301,chat_jid:chat,timestamp:state.now,data:{type:'user_message',content:text,is_bot_message:false,thread_id:7}};posts.push(post);return post;},broadcastEvent:(event,payload)=>host.emit(event,payload),queue:{enqueue(_fn,key,scope){calls.push({kind:'enqueue',key,scope})}},processChat:async()=>{throw Error('No inference allowed')}});
 try{
  const html=(await fs.readFile(root+'/app/runtime/web/static/classic/index.html','utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'idle-steer',revision:0,models:[],sessions:[]}}));
  await page.route('**/timeline?*',r=>r.fulfill({json:{posts,has_more:false}}));
  await page.route('**/agent/queue-state?*',r=>r.fulfill({json:{items:item?[{row_id:41,content:item.queuedContent,timestamp:state.now,thread_id:7}]:[],count:item?1:0}}));
  await page.route('**/agent/queue-steer',async r=>{const req=r.request();calls.push({kind:'request',body:req.postDataJSON()});const res=await service.handleAgentQueueSteer(new Request(req.url(),{method:'POST',headers:{'Content-Type':'application/json'},body:req.postData()}));await r.fulfill({status:res.status,json:await res.json()});});
  await page.goto(host.origin);await host.connected();const input=page.locator('.compose-box textarea');await input.fill('oracle unsent draft');await input.blur();
  const row=page.locator('.compose-queue-stack-item').filter({hasText:'idle selected instruction'}),steer=row.locator('.compose-queue-stack-steer-btn');await steer.waitFor();assert(await steer.isEnabled(),'idle Steer enabled');
  await steer.click();await page.waitForFunction(()=>document.body.innerText.includes('idle selected instruction'));await steer.waitFor();assert(item,'failed storage restores item');assert(!calls.some(c=>c.kind==='enqueue'));
  failStore=false;await steer.click();await page.waitForFunction(()=>!document.querySelector('.compose-queue-stack-item'));await page.locator('#post-301').waitFor();
  assert.equal(item,null);assert.equal(calls.filter(c=>c.kind==='enqueue').length,1);assert.equal(posts.length,1);assert.deepEqual(calls.filter(c=>c.kind==='store').at(-1).media,[9]);assert.equal(await input.inputValue(),'oracle unsent draft');host.assert();
  await page.screenshot({path:path.join(output,`${browserName}-${viewportName}.png`)});report.cases.push({browser:browserName,viewport:viewportName,result:'pass',calls});
 }finally{await fs.writeFile(path.join(output,'evidence.json'),JSON.stringify(report,null,2));await host.dispose();await browser.close();}
}
console.log(JSON.stringify({output,cases:report.cases.map(({browser,viewport,result})=>({browser,viewport,result}))}));
