// Shipped Piclaw 3.2.4 UI with a disposable timeline and browser clipboard capture.
// Covers copy controls only; deletion has separate oracle evidence.
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(root+'/VERSION','utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const fixture=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const output=path.resolve('test-results/ux-oracle/copy-controls');await fs.mkdir(output,{recursive:true});const cases=[];
const source='# Source Markdown\n\n**Bold** and _italic_\n\n```js\nconst value = "<世界>";\n```\n';
const code='const value = "<世界>";\n';
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,sessionId:'web:default',theme:'light'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 try{
  const html=(await fs.readFile(root+'/app/runtime/web/static/classic/index.html','utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'copy',revision:0,models:[],sessions:[]}}));
  await page.route('**/timeline?*',r=>r.fulfill({json:{posts:[{id:301,timestamp:state.now,chat_jid:state.sessionId,data:{type:'agent_response',content:source,agent_id:'default',is_bot_message:true}}],has_more:false}}));
  await page.addInitScript(()=>{window.__oracleCopies=[];document.addEventListener('copy',e=>window.__oracleCopies.push({trusted:e.isTrusted,text:e.clipboardData?.getData('text/plain'),html:e.clipboardData?.getData('text/html')}));});
  await page.goto(host.origin);await host.connected();const post=page.locator('#post-301');await post.waitFor();
  const input=page.locator('.compose-box textarea');await input.fill('keep unsent draft');
  await post.getByRole('button',{name:'Copy message',exact:true}).click();
  await page.waitForFunction(()=>window.__oracleCopies.length>0);
  const message=await page.evaluate(()=>window.__oracleCopies.at(-1));
  assert.equal(message.trusted,true);assert.equal(message.text,source.trimEnd());
  assert(message.html.includes('<strong>Bold</strong>'));
  await post.getByRole('button',{name:'Copy code',exact:true}).click();
  await page.waitForFunction(()=>window.__oracleCopies.length>1);
  const snippet=await page.evaluate(()=>window.__oracleCopies.at(-1));
  assert.equal(snippet.trusted,true);assert.equal(snippet.text,code);
  assert.equal(await input.inputValue(),'keep unsent draft');host.assert();
  cases.push({browser:browserName,viewport:viewportName,result:'pass',message,snippet,draft:await input.inputValue()});
 }finally{await fs.writeFile(path.join(output,'evidence.json'),JSON.stringify({scope:'Shipped Piclaw3.2.4 copy controls with disposable timeline; no backend deletion, clipboard OS or physical-device acceptance.',cases},null,2));await host.dispose();await browser.close();}
}
console.log(JSON.stringify({output,cases}));
