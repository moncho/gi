// Installed Piclaw 3.2.4 Classic code-block copy control with disposable Markdown timeline.
// Browser-native copy event; no OS clipboard persistence or physical permission claim.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const base=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const code='const answer = "<tag> & 中文🙂";\nconsole.log(answer);\n';
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'}),state={...base,sessionId:'web:default',theme:'light'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 try{
  const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
  const posts=[{id:761,chat_jid:state.sessionId,timestamp:state.now,data:{type:'agent_response',content:'```javascript\n'+code+'```',is_bot_message:true,agent_id:'default'}}];
  await page.addInitScript(()=>{window.__nativeCopies=[];document.addEventListener('copy',event=>{const data=event.clipboardData;window.__nativeCopies.push({trusted:event.isTrusted,text:data?.getData('text/plain'),html:data?.getData('text/html')});});});
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'code-copy',revision:0,models:[],sessions:[]}}));
  await page.route('**/timeline?*',r=>r.fulfill({json:{posts,has_more:false}}));
  await page.goto(host.origin);await host.connected();
  const post=page.locator('#post-761'),block=post.locator('.post-code-block'),button=block.getByRole('button',{name:'Copy code',exact:true});
  await button.waitFor({state:'visible'});assert.equal(await block.locator('pre code').textContent(),code);
  const position=await button.evaluate(el=>{const a=el.getBoundingClientRect(),b=el.closest('.post-code-block').getBoundingClientRect();return {top:a.top-b.top,right:b.right-a.right};});
  assert(position.top>=0&&position.top<=12&&position.right>=0&&position.right<=12,`Copy button placement ${JSON.stringify(position)}`);
  const draft=page.locator('.compose-box textarea');await draft.fill('retained code draft');
  await button.click();await page.waitForFunction(()=>window.__nativeCopies.length>0);
  const event=await page.evaluate(()=>window.__nativeCopies.at(-1));assert.equal(event.trusted,true);assert.equal(event.text,code);assert(!event.text.includes('<span'));
  assert.equal(await draft.inputValue(),'retained code draft');host.assert();
  cases.push({browser:browserName,viewport:viewportName,topRight:true,trustedCopy:true,exactText:true,draftRetained:true});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4 mounted Classic, disposable assistant fenced-code row, native browser copy event and placement. No persisted provider response, OS clipboard permission/paste, physical input or whole-feature parity.',cases},null,2));
