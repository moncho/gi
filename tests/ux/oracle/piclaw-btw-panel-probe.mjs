// Installed Piclaw 3.2.4 mounted BTW panel with disposable side-prompt stream.
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
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 const pane=page.locator('.btw-panel');let calls=0,release=()=>{},injects=0;
 try{
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'btw-oracle',revision:0,models:[],sessions:[]}}));
  await page.route('**/agent/default/message?*',route=>{
   injects++;const request=route.request().postDataJSON();
   assert.match(request.content,/fixture side question/);assert.match(request.content,/Side answer/);
   assert.equal(new URL(route.request().url()).searchParams.get('chat_jid'),state.sessionId);
   return route.fulfill({json:{ok:true}});
  });
  await page.route('**/agent/side-prompt/stream',async route=>{
   calls++;const body=route.request().postDataJSON();assert.equal(body.chat_jid,state.sessionId);
   assert.equal(body.prompt,'fixture side question');
   const thisCall=calls;await new Promise(resolve=>{release=resolve;});
   const event=(name,data)=>`event: ${name}\ndata: ${JSON.stringify(data)}\n\n`;
   const frames=thisCall===1
    ? event('side_prompt_error',{error:'Fixture side error'})
    : event('side_prompt_thinking_delta',{delta:'Side reasoning'})+event('side_prompt_text_delta',{delta:'Side answer'})+event('side_prompt_done',{text:'Side answer',thinking:'Side reasoning'});
   await route.fulfill({contentType:'text/event-stream',body:frames});
  });
  await page.goto(host.origin);await host.connected();const input=page.locator('.compose-box textarea');
  const ask=async()=>{await input.fill('/btw fixture side question');await input.press('Enter');};
  await ask();await pane.waitFor();assert.equal(await pane.locator('.btw-question').textContent(),'fixture side question');
  assert.equal(await pane.locator('.btw-panel-footer').count(),0);assert.equal(await pane.locator('.btw-answer').count(),0);
  release();await pane.locator('.btw-error').waitFor();await pane.getByRole('button',{name:'Retry'}).waitFor();
  assert.equal(await pane.getByRole('button',{name:'Inject into chat'}).isDisabled(),true);
  await pane.getByRole('button',{name:'Retry'}).click();await page.waitForFunction(()=>document.querySelector('.btw-panel-status-running'));
  assert.equal(await pane.locator('.btw-panel-footer').count(),0);release();
  await pane.locator('.btw-answer').waitFor();assert.match(await pane.locator('.btw-answer').textContent(),/Side answer/);
  assert.match(await pane.locator('.btw-thinking').textContent(),/Side reasoning/);
  assert.equal(await pane.getByRole('button',{name:'Inject into chat'}).isEnabled(),true);
  await pane.getByRole('button',{name:'Inject into chat'}).click();await page.waitForFunction(()=>document.querySelector('.btw-panel')===null||document.querySelector('.btw-panel-status-success'));
  assert.equal(injects,1);assert.equal(calls,2);host.assert();cases.push({browser:browserName,viewport:viewportName,errorRetry:true,completedAnswer:true,injectRequested:true});
 }finally{release();await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed 3.2.4 shipped BTW UI with synthetic side-prompt error/success and intercepted Inject request; no live provider, main-chat persistence or physical device.',cases},null,2));
