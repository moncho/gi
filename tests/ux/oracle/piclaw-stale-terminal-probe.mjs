// Bounded shipped Piclaw 3.2.4 stale-terminal replay; no reconnect or backend writes.
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const fixture=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit})) for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,sessionId:'web:default',theme:'dark'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 let status={status:'idle',data:null};
 try{
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'stale-terminal',revision:0,models:[],sessions:[]}}));
  await page.route('**/agent/status?*',r=>r.fulfill({json:{status,model:state.model,context:state.model.context_usage,metrics:state.metrics,errors:[]}}));
  await page.goto(host.origin);await host.connected();
  const stop=page.getByRole('button',{name:'Stop response',exact:true});
  const active=id=>({type:'thinking',title:'Thinking...',phase:'thinking',turn_id:id,chat_jid:state.sessionId});
  const old=active('old-turn');status={status:'active',data:old};host.emit('agent_status',old);await stop.waitFor();
  const fresh=active('new-turn');status={status:'active',data:fresh};host.emit('agent_status',fresh);await stop.waitFor();
  await page.waitForTimeout(100); // settle the new turn before replaying the old frame
  host.emit('agent_status',{type:'done',turn_id:'old-turn',chat_jid:state.sessionId});
  await page.waitForTimeout(250);
  const staleKeptStop=await stop.count()===1;
  assert.equal(status.data.turn_id,'new-turn'); // fixture's authoritative read never changed
  assert(staleKeptStop,'an older terminal frame must not leave the newer turn without Stop');
  status={status:'idle',data:null};host.emit('agent_status',{type:'done',turn_id:'new-turn',chat_jid:state.sessionId});
  await page.waitForFunction(()=>!document.querySelector('[aria-label="Stop response"]'));
  host.assert();cases.push({browser:browserName,viewport:viewportName,staleKeptStop,freshDoneClearsStop:true});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed Piclaw shipped UI, synthetic same-chat status sequence; no reconnect, live run, or HTTP acceptance',cases},null,2));
