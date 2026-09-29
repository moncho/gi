// Installed Piclaw 3.2.4 Classic Markdown table rendering with a disposable timeline.
// Checks the table geometry Gi already implements; no live provider or pixel-equivalence claim.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const base=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const markdown='| Column | Description |\n| --- | --- |\n| Alpha | Native stored Markdown |\n| Beta | 第二行 |';
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'}),state={...base,sessionId:'web:default',theme:'light'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 try{
  const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
  const posts=[{id:751,chat_jid:state.sessionId,timestamp:state.now,data:{type:'agent_response',content:markdown,is_bot_message:true,agent_id:'default'}}];
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'markdown-table',revision:0,models:[],sessions:[]}}));
  await page.route('**/timeline?*',r=>r.fulfill({json:{posts,has_more:false}}));
  await page.goto(host.origin);await host.connected();
  const post=page.locator('#post-751'),table=post.locator('.post-content table');await table.waitFor({state:'visible'});
  const measured=await table.evaluate(el=>{const css=getComputedStyle(el),box=el.getBoundingClientRect(),parent=el.parentElement.getBoundingClientRect();return {display:css.display,layout:css.tableLayout,width:box.width,parent:parent.width,rows:el.querySelectorAll('tbody tr').length,text:el.textContent||''};});
  assert.equal(measured.display,'table');assert.equal(measured.layout,'auto');assert(Math.abs(measured.width-measured.parent)<=1);
  assert.equal(measured.rows,2);assert(measured.text.includes('第二行'));
  const draft=page.locator('.compose-box textarea');await draft.fill('retained table draft');assert.equal(await draft.inputValue(),'retained table draft');host.assert();
  cases.push({browser:browserName,viewport:viewportName,display:measured.display,layout:measured.layout,fullParentWidth:true,rows:measured.rows,unicode:true,draftRetained:true});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4 mounted Classic with disposable assistant Markdown row; computed table display/layout/width and draft only. No live provider, stored Gi database, visual pixel tolerance or whole rendering feature acceptance.',cases},null,2));
