import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';assert.equal((await fs.readFile(root+'/VERSION','utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
assert.equal(createHash('sha256').update(await fs.readFile(root+'/app/runtime/web/static/classic/dist/app.bundle.js.map')).digest('hex'),reference.map.sha256);
const fixture=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const output=path.resolve('test-results/ux-oracle/stop-ui');await fs.mkdir(output,{recursive:true});const cases=[];
for(const [browserName,type]of Object.entries({chromium,webkit}))for(const [viewportName,viewport]of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,theme:'dark',sessionId:'web:default'};const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});let status={status:'idle',data:null};const calls=[];let release;
 try{
  const html=(await fs.readFile(root+'/app/runtime/web/static/classic/index.html','utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'stop',revision:0,models:[],sessions:[]}}));
  await page.route('**/agent/status?*',r=>r.fulfill({json:{status,model:state.model,context:state.model.context_usage,metrics:state.metrics,errors:[]}}));
  const gate=new Promise(r=>release=r);await page.route('**/agent/default/message?*',async r=>{calls.push(r.request().postDataJSON());await gate;await r.fulfill({json:{status:'ok',ui_only:true}})});
  await page.goto(host.origin);await host.connected();const input=page.locator('.compose-box textarea');await input.fill('keep Stop draft');
  status={status:'active',data:{type:'thinking',turn_id:'stop-oracle',chat_jid:state.sessionId,title:'Thinking'}};host.emit('agent_status',status.data);
  const stop=page.getByRole('button',{name:'Stop response',exact:true});await stop.waitFor();const sent=page.waitForRequest(r=>r.method()==='POST'&&r.url().includes('/agent/default/message'));await stop.click();const request=await sent;const body=request.postDataJSON();assert.equal(body.content,'/abort');assert.equal(body.mode,'steer');assert.equal(await input.inputValue(),'keep Stop draft');
  release();status={status:'idle',data:null};host.emit('agent_status',{type:'done',turn_id:'stop-oracle',chat_jid:state.sessionId});await page.waitForFunction(()=>!document.querySelector('[aria-label="Stop response"]'));
  assert.equal(await page.getByRole('button',{name:'Resume queue',exact:true}).count(),0);assert.equal(await input.inputValue(),'keep Stop draft');assert.equal(calls.length,1);host.assert();await page.screenshot({path:path.join(output,`${browserName}-${viewportName}.png`)});cases.push({browserName,viewportName,calls,draft:'keep Stop draft',resumeControl:false,result:'pass'});
 }finally{release?.();await fs.writeFile(path.join(output,'evidence.json'),JSON.stringify({scope:'Installed Piclaw 3.2.4 UI, isolated abort and status responses; no live mutations or physical-device claim.',mapSha256:reference.map.sha256,cases},null,2));await host.dispose();await browser.close()}
}
console.log(JSON.stringify({output,cases}));
