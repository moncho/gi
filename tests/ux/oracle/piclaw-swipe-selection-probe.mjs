// Installed Piclaw 3.2.4 Classic bundle, mounted with disposable read-only chat fixtures.
// Synthetic touch and synthetic timeline text selection; no physical touch or real sessions.
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
for(const [name,type] of Object.entries({chromium,webkit}))for(const [size,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180}})){ // iOS-sized fixture only; desktop bootstrap can prewarm another chat
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block',userAgent:'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1'});
 const state={...base,sessionId:'web:default',theme:'dark', model:{...base.model}};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 const chats=[{chat_jid:'web:default',agent_name:'Fixture default',is_active:true},{chat_jid:'web:other',agent_name:'Fixture other',is_active:false}];
 try{
  const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
  await page.route('**/agent/active-chats*',r=>r.fulfill({json:{chats}}));
  await page.route('**/agent/branches*',r=>r.fulfill({json:{chats}}));
  await page.route('**/timeline?*',r=>new URL(r.request().url()).searchParams.get('chat_jid')==='web:other'?r.fulfill({json:{posts:[],has_more:false}}):r.fallback());
  await page.route('**/?chat_jid=web%3Aother*',r=>r.fulfill({contentType:'text/html',body:'<!doctype html><title>Fixture switched</title>'}));
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.goto(host.origin);await host.connected();await page.locator('.timeline').first().waitFor();
  const roster=await page.evaluate(async()=> (await (await fetch('/agent/active-chats')).json()).chats.map(x=>x.chat_jid));assert.deepEqual(roster,['web:default','web:other']);
  const gesture=async selection=>page.locator('.timeline').first().evaluate((timeline,value)=>{
   const span=document.createElement('span');span.style.whiteSpace='pre';span.textContent=value==='   '?'A   B':value;timeline.appendChild(span);
   const range=document.createRange();if(value==='   '){range.setStart(span.firstChild,1);range.setEnd(span.firstChild,4);}else range.selectNodeContents(span);
   const selected=window.getSelection();selected.removeAllRanges();selected.addRange(range);
   const observed=selected.toString();
   for(const [eventName,x] of [['touchstart',190],['touchmove',85],['touchend',85]]){
    const touch={identifier:1,target:timeline,clientX:x,clientY:150};const event=new Event(eventName,{bubbles:true,cancelable:true});
    Object.defineProperty(event,'touches',{value:eventName==='touchend'?[]:[touch]});
    Object.defineProperty(event,'changedTouches',{value:[touch]});timeline.dispatchEvent(event);
   }
   return observed;
  },selection);
  assert.equal(await gesture('Selected words'),'Selected words');
  await page.waitForTimeout(160);assert.equal(new URL(page.url()).searchParams.get('chat_jid'),null,'nonblank selection must block navigation');
  assert.equal(await gesture('   '),'   ');
  await page.waitForFunction(()=>new URL(location.href).searchParams.get('chat_jid')==='web:other',null,{timeout:5000});
  cases.push({browser:name,viewport:size,nonblankSelectionBlocked:true,whitespaceOnlySelectionPermitted:true,selectedChat:'web:other'});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4 mounted Classic bundle, disposable two-chat roster, synthetic DOM selection and touch. No live chat mutation, real device gesture, or whole-clause acceptance.',cases},null,2));
