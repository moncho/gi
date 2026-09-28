// Installed Piclaw 3.2.4 shipped UI: verify which widget SSE events actually
// reach it. A known agent status is the positive control. No live writes.
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
 const pane=page.locator('.floating-widget-pane');
 const payload={chat_jid:state.sessionId,turn_id:'widget-turn',tool_call_id:'widget-call',widget_id:'widget-id',title:'Oracle widget',artifact:{kind:'html'}};
 try{
  // Observe raw browser delivery independently of the shipped client's
  // event bindings; an absent pane cannot be explained by a lost SSE frame.
  await page.addInitScript(() => {
   window.__widgetOracleEvents=[];
   const NativeEventSource=window.EventSource;
   window.EventSource=class extends NativeEventSource {
    constructor(...args){
     super(...args);
     for(const name of ['agent_status','generated_widget_open','generated_widget_delta','generated_widget_final'])
      this.addEventListener(name,event=>window.__widgetOracleEvents.push({name,data:JSON.parse(event.data)}));
    }
   };
  });
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'widget-oracle',revision:0,models:[],sessions:[]}}));
  // An idle authoritative GET makes the visible status title an SSE-only control.
  await page.route('**/agent/status?*',r=>r.fulfill({json:{status:{status:'idle',data:null},model:state.model,context:state.model.context_usage,metrics:state.metrics,errors:[]}}));
  await page.goto(host.origin);await host.connected();
  const input=page.locator('.compose-box textarea');await input.fill('unsent widget draft');
  // Known registered event establishes the same SSE stream reaches the app.
  host.emit('agent_status',{chat_jid:state.sessionId,turn_id:'widget-turn',type:'thinking',title:'Oracle status ready'});
  await page.getByText('Oracle status ready',{exact:false}).first().waitFor();
  await page.waitForFunction(()=>window.__widgetOracleEvents?.[0]?.name==='agent_status');
  for(const [event,patch] of [
   ['generated_widget_open',{status:'loading'}],
   ['generated_widget_delta',{status:'streaming',artifact:{kind:'html',html:'<p>widget streaming</p>'}}],
   ['generated_widget_final',{status:'final',artifact:{kind:'html',html:'<p>widget final</p>'}}],
  ])host.emit(event,{...payload,...patch});
  await page.waitForFunction(()=>window.__widgetOracleEvents?.length===4);
  assert.deepEqual(await page.evaluate(()=>window.__widgetOracleEvents.map(e=>e.name)),['agent_status','generated_widget_open','generated_widget_delta','generated_widget_final']);
  await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
  assert.equal(await pane.count(),0,'delivered but unbound widget SSE must not be counted as a rendered live widget');
  assert.equal(await input.inputValue(),'unsent widget draft');
  assert.equal(host.calls.filter(c=>c.method==='POST'&&c.path.includes('/queue-')).length,0);
  host.assert();cases.push({browser:browserName,viewport:viewportName,registeredStatusVisible:true,rawWidgetEventsDelivered:3,widgetEventPaneCount:0,result:'unbound'});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed shipped UI, synthetic same-chat SSE; registered status positive control and three raw widget frames delivered, no mounted pane. No live backend/provider/queue mutation',cases},null,2));
