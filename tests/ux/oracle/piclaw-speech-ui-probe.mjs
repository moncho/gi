// Shipped Piclaw 3.2.4 speech controls with a disposable timeline.
// Only the OS speech and clipboard boundaries are stubbed; no live writes.
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
const source='# First post\n\nHello **world**.\n\n```js\nconst value = "<b>literal</b>";\n```';
const second='Second post with [a link](https://example.invalid).';
const code='const value = "<b>literal</b>";\n';
const results=[];
for(const [browserName,type] of Object.entries({chromium,webkit})) for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,sessionId:'web:default',theme:'light'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 try{
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'speech',revision:0,models:[],sessions:[]}}));
  await page.route('**/timeline?*',r=>r.fulfill({json:{posts:[source,second].map((content,index)=>({id:301+index,timestamp:state.now,chat_jid:state.sessionId,data:{type:'agent_response',content,agent_id:'default',is_bot_message:true}})),has_more:false}}));
  await page.addInitScript(()=>{
   const log={calls:[],utterances:[]};window.__speechOracle=log;window.__speechCopies=[];
   Object.defineProperty(window,'SpeechSynthesisUtterance',{configurable:true,value:class{constructor(text){this.text=text;}}});
   Object.defineProperty(window,'speechSynthesis',{configurable:true,value:{
    speak(utterance){log.calls.push('speak');log.utterances.push(utterance);},
    cancel(){log.calls.push('cancel');log.utterances.at(-1)?.onend?.();},
   }});
   document.addEventListener('copy',event=>window.__speechCopies.push({trusted:event.isTrusted,text:event.clipboardData?.getData('text/plain'),html:event.clipboardData?.getData('text/html')}));
  });
  await page.goto(host.origin);await host.connected();
  const first=page.locator('#post-301'),other=page.locator('#post-302');await first.waitFor();await other.waitFor();
  const input=page.locator('.compose-box textarea');await input.fill('keep unsent speech draft');
  await first.getByRole('button',{name:'Read aloud',exact:true}).click();
  await first.getByRole('button',{name:'Stop reading aloud',exact:true}).waitFor();
  const firstUtterance=await page.evaluate(()=>window.__speechOracle.utterances[0]?.text);
  assert(firstUtterance.includes('Code block omitted.'));
  assert(!firstUtterance.includes('const value'));
  await other.getByRole('button',{name:'Read aloud',exact:true}).click();
  await other.getByRole('button',{name:'Stop reading aloud',exact:true}).waitFor();
  assert.equal(await first.getByRole('button',{name:'Read aloud',exact:true}).count(),1);
  assert.deepEqual(await page.evaluate(()=>window.__speechOracle.calls),['cancel','speak','cancel','speak']);
  await page.evaluate(()=>{window.__speechOracle.utterances[0].onend?.();window.__speechOracle.utterances[0].onerror?.();});
  assert.equal(await other.getByRole('button',{name:'Stop reading aloud',exact:true}).count(),1);
  await first.getByRole('button',{name:'Copy code',exact:true}).click();
  await page.waitForFunction(()=>window.__speechCopies.length>0);
  assert.deepEqual(await page.evaluate(()=>window.__speechCopies.at(-1)),{trusted:true,text:code,html:''});
  assert.equal(await other.getByRole('button',{name:'Stop reading aloud',exact:true}).count(),1);
  await other.getByRole('button',{name:'Stop reading aloud',exact:true}).click();
  assert.equal(await page.locator('.post-speak-btn.is-active').count(),0);
  assert.equal(await input.inputValue(),'keep unsent speech draft');
  host.assert();
  results.push({browser:browserName,viewport:viewportName,firstUtterance,calls:await page.evaluate(()=>window.__speechOracle.calls),result:'pass'});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({oracle:'Installed Piclaw 3.2.4 shipped UI, disposable timeline and stubbed OS speech/clipboard',cases:results},null,2));
