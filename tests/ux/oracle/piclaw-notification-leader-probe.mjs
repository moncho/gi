// Installed Piclaw 3.2.4 local-notification ownership on the mounted SSE path.
// Synthetic browser Notification, same-device presence and final reply only.
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
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,sessionId:'web:default',theme:'dark'};
 await page.addInitScript(()=>{
  if(window!==window.top)return;
  localStorage.setItem('notificationsEnabled','true');
  localStorage.setItem('piclaw.notifications.deviceId','oracle-device');
  sessionStorage.setItem('piclaw.notifications.clientId','client-b');
  window.__oracleNotices=[];window.__oracleFrames=[];
  const NativeEventSource=window.EventSource;
  window.EventSource=class extends NativeEventSource {
   constructor(...args){super(...args);for(const name of ['agent_response','agent_status'])this.addEventListener(name,e=>window.__oracleFrames.push({name,data:JSON.parse(e.data)}));}
  };
  window.Notification=class {static permission='granted';constructor(title,options){window.__oracleNotices.push({title,body:options?.body});}};
 });
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 const noticeCount=()=>page.evaluate(()=>window.__oracleNotices.length);
 const sibling=(clientId,visibilityState)=>page.evaluate(({clientId,visibilityState,chatJid})=>{
  const key=`piclaw.notifications.presence.oracle-device:${clientId}`;
  localStorage.setItem(key,JSON.stringify({deviceId:'oracle-device',clientId,chatJid,visibilityState,hasFocus:visibilityState==='visible',updatedAtMs:Date.now()}));
 },{clientId,visibilityState,chatJid:state.sessionId});
 const clearSibling=id=>page.evaluate(id=>localStorage.removeItem(`piclaw.notifications.presence.oracle-device:${id}`),id);
 const emitReply=(id,turn)=>{
  host.emit('agent_status',{chat_jid:state.sessionId,type:'thinking',title:'Thinking...',turn_id:turn});
  host.emit('agent_response',{id,chat_jid:state.sessionId,turn_id:turn,timestamp:new Date().toISOString(),data:{type:'agent_response',content:`Oracle reply ${id}`,agent_id:'default',is_bot_message:true}});
  host.emit('agent_status',{chat_jid:state.sessionId,type:'done',turn_id:turn});
 };
 try{
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'notification-leader',revision:0,models:[],sessions:[]}}));
  await page.route('**/agent/status?*',r=>r.fulfill({json:{status:{status:'idle',data:null},model:state.model,context:state.model.context_usage,metrics:state.metrics,errors:[]}}));
  await page.goto(host.origin);await host.connected();
  await page.waitForFunction(()=>Object.keys(localStorage).some(k=>k.startsWith('piclaw.notifications.presence.oracle-device:client-b')));
  // Suppress this tab's visibility using a browser-local descriptor; no OS state is changed.
  await page.evaluate(()=>Object.defineProperty(document,'visibilityState',{configurable:true,get:()=>window.__oracleVisibility||'hidden'}));
  await page.evaluate(()=>{window.__oracleVisibility='hidden';document.dispatchEvent(new Event('visibilitychange'));});
  const settleReply=async id=>{await page.waitForFunction(id=>window.__oracleFrames.some(f=>f.name==='agent_response'&&f.data.id===id)&&window.__oracleFrames.some(f=>f.name==='agent_status'&&f.data.type==='done'&&f.data.turn_id===`oracle-turn-${id-9400}`),id);await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));await page.waitForTimeout(250);};
  await sibling('client-a','visible');emitReply(9401,'oracle-turn-1');await settleReply(9401);assert.equal(await noticeCount(),0,'visible sibling suppresses');
  await sibling('client-a','hidden');emitReply(9402,'oracle-turn-2');await settleReply(9402);assert.equal(await noticeCount(),0,'lexically earlier hidden sibling leads');
  await clearSibling('client-a');emitReply(9403,'oracle-turn-3');await settleReply(9403);await page.waitForFunction(()=>window.__oracleNotices.length===1);
  const notices=await page.evaluate(()=>window.__oracleNotices);assert.equal(notices.length,1,'remaining hidden client owns one local notification');assert.match(notices[0].body,/9403/);
  host.assert();cases.push({browser:browserName,viewport:viewportName,visibleSiblingSuppressed:true,hiddenLeaderSuppressed:true,remainingHiddenNotices:notices.length});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4 mounted SSE with mocked Notification and browser-local same-device presence; no OS delivery, server fanout or physical device.',cases},null,2));
