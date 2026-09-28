// Installed Piclaw 3.2.4 event translator and shipped Classic UI with two
// synthetic overlapping calls. No provider, live HTTP handler or store writes.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {pathToFileURL} from 'node:url';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';

const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const stateBase=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
const {createStreamingEventHandler}=await import(pathToFileURL(path.join(root,'app/runtime/src/channels/web/sse/agent-events.ts')).href);
const results=[];
for(const [browserName,type] of Object.entries({chromium,webkit})) for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...stateBase,theme:'dark',sessionId:'web:default'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 let status={status:'idle',data:null};const frames=[];
 const emitter={status:payload=>{frames.push(payload);status={status:'active',data:{...payload,chat_jid:state.sessionId}};host.emit('agent_status',status.data);}};
 const handler=createStreamingEventHandler({emitter,agentId:'default',threadId:'1',turnId:'two-calls',displayUpdateIntervalMs:0});
 try{
  await page.route(host.origin+'/',route=>route.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',route=>route.fulfill({json:{scope:'two-calls',revision:0,models:[],sessions:[]}}));
  await page.route('**/agent/status?*',route=>route.fulfill({json:{status,model:state.model,context:state.model.context_usage,metrics:state.metrics,errors:[]}}));
  await page.goto(host.origin);await host.connected();
  const pane=page.locator('[data-panel-key="tool-output"]');
  const start=(id,command)=>handler({type:'tool_execution_start',toolCallId:id,toolName:'shell',args:{command}});
  const update=(id,text)=>handler({type:'tool_execution_update',toolCallId:id,toolName:'shell',partialResult:{content:[{type:'text',text}]}});
  const end=id=>handler({type:'tool_execution_end',toolCallId:id,toolName:'shell',result:{content:[{type:'text',text:'done'}]},isError:false});
  start('call-a','printf A');update('call-a','FIRST CALL output');
  await pane.getByText('FIRST CALL output',{exact:false}).waitFor();
  start('call-b','printf B');update('call-b','SECOND CALL output');
  await pane.getByText('SECOND CALL output',{exact:false}).waitFor();
  assert.deepEqual(frames.at(-1).active_tools.map(tool=>tool.tool_call_id),['call-a','call-b']);
  assert.equal(frames.at(-1).active_tool_count,2);
  end('call-a');
  const surviving=frames.at(-1);
  assert.equal(surviving.type,'tool_status');assert.equal(surviving.tool_call_id,'call-b');
  assert.equal(surviving.last_completed_tool.tool_call_id,'call-a');
  assert.deepEqual(surviving.active_tools.map(tool=>tool.tool_call_id),['call-b']);
  await pane.getByText('SECOND CALL output',{exact:false}).waitFor();
  assert.equal(await page.getByText('Waiting for model...', {exact:false}).count(),0);
  assert.equal(await page.getByText('Completed: shell',{exact:true}).count(),0);
  assert.equal(await page.locator('.post').count(),0);
  end('call-b');
  assert.equal(frames.at(-1).type,'waiting');assert.equal(frames.at(-1).active_tool_count,0);
  await page.getByText('Waiting for model...', {exact:false}).first().waitFor();
  assert.equal(await pane.count(),0);assert.equal(await page.locator('.post').count(),0);
  host.assert();results.push({browser:browserName,viewport:viewportName,survivingCall:surviving.tool_call_id,completedCall:surviving.last_completed_tool.tool_call_id,result:'pass'});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed translator + shipped UI, two synthetic overlapping tool calls; no provider, live routing, reload or store acceptance',cases:results},null,2));
