// Installed Piclaw 3.2.4 Classic resource and link-preview navigation with disposable remote targets.
// Tests the new-tab isolation Gi already implements; no live remote-site trust or backend persistence.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const base=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const links=[['.resource-link','https://resource.example.invalid/report?q=beta'],['.link-preview','http://preview.example.invalid/article']];
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'}),state={...base,sessionId:'web:default',theme:'light'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 const remote=[];
 try{
  const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
  const posts=[{id:771,chat_jid:state.sessionId,timestamp:state.now,data:{type:'agent_response',content:'Remote-link fixture',is_bot_message:true,agent_id:'default',content_blocks:[{type:'resource_link',uri:links[0][1],title:'Report'}],link_previews:[{url:links[1][1],title:'Article'}]}}];
  await page.context().route(/^https?:\/\/[^/]+\.example\.invalid\//,r=>{remote.push({url:r.request().url(),referer:r.request().headers().referer||null});return r.fulfill({status:200,contentType:'text/html',body:'<!doctype html><title>Disposable remote target</title>'});});
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'remote-links',revision:0,models:[],sessions:[]}}));
  await page.route('**/timeline?*',r=>r.fulfill({json:{posts,has_more:false}}));
  await page.goto(host.origin);await host.connected();
  const post=page.locator('#post-771');await post.waitFor({state:'visible'});
  const draft=page.locator('.compose-box textarea');await draft.fill('retained link draft β');
  for(const [selector,url] of links){
   const link=post.locator(selector);await link.waitFor({state:'visible'});
   assert.equal(await link.getAttribute('href'),url);assert.equal(await link.getAttribute('target'),'_blank');assert.equal(await link.getAttribute('rel'),'noopener noreferrer');
   const popupPromise=page.context().waitForEvent('page');if(selector==='.resource-link')await link.click();else{await link.focus();await link.press('Enter');}
   const popup=await popupPromise;try{await popup.waitForLoadState();assert.equal(popup.url(),url);assert.equal(await popup.evaluate(()=>window.opener===null),true);}finally{await popup.close();}
   assert.deepEqual(remote.at(-1),{url,referer:null});assert.equal(await draft.inputValue(),'retained link draft β');
  }
  assert.equal(remote.length,2);host.assert();
  cases.push({browser:browserName,viewport:viewportName,resourceAndPreview:true,newTabs:true,nullOpener:true,noReferrer:true,draftRetained:true});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4 mounted Classic with disposable resource/preview post and intercepted remote pages. Browser new-tab/opener/referrer only; no live remote target, Gi database persistence or whole timeline feature acceptance.',cases},null,2));
