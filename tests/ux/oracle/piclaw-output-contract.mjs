// Independent browser oracle: installed shipped Piclaw assets, not the Gi vendor copy.
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {pathToFileURL} from 'node:url';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const fixture=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const {createStreamingEventHandler}=await import(pathToFileURL(path.join(root,'app/runtime/src/channels/web/sse/agent-events.ts')).href);
const output=path.resolve('test-results/ux-oracle/output-contract');await fs.mkdir(output,{recursive:true});
const text=Array.from({length:12},(_,i)=>`line-${String(i).padStart(2,'0')} **literal** <script>not executed</script> ${i===11?'x'.repeat(170):''}`).join('\n');
const statuses=[];
const emitter={status:p=>statuses.push(p)};
const handler=createStreamingEventHandler({emitter,agentId:'default',threadId:'1',turnId:'output-turn',displayUpdateIntervalMs:0});
handler({type:'tool_execution_start',toolCallId:'call',toolName:'shell',args:{command:'printf output'}});
handler({type:'tool_execution_update',toolCallId:'call',toolName:'shell',partialResult:{content:[{type:'text',text}]}});
const running=statuses.at(-1);
handler({type:'tool_execution_end',toolCallId:'call',toolName:'shell',result:{content:[{type:'text',text:'done'}]},isError:false});
const waiting=statuses.at(-1);assert.equal(waiting.type,'waiting');
const report={scope:'Installed event translator and shipped browser assets versus Gi adapter. Bounded synthetic lifecycle; no provider/device/reload acceptance.',cases:[]};
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true});const observations={};
 try{for(const hostName of ['piclaw','gi']){
  const page=await browser.newPage({viewport,serviceWorkers:'block'});
  const state={...fixture,theme:'dark',sessionId:hostName==='piclaw'?'web:default':'gi:main'};
  const host=await installPixelHost({page,host:hostName,root:hostName==='piclaw'?root:path.resolve('internal/web/static'),state,reference,allowPresenceBeacon:true});
  const runningActivity={status:'running',phase:'waiting_on_tools',turn_id:'output-turn',tool:{state:'running',name:'shell',preview:'printf output',started_at:running.started_at,output_preview:running.output_preview,output_total_lines:running.output_total_lines}};
  let activity={status:'idle'},piclawStatus={status:'idle',data:null};
  if(hostName==='piclaw'){
   const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
   await page.route('**/agent/status?*',r=>r.fulfill({json:{status:piclawStatus,model:state.model,context:state.model.context_usage,metrics:state.metrics,errors:[]}}));
   await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'output',revision:0,models:[],sessions:[]}}));
  }else {
   await page.route('**/api/sessions/main/activity',r=>r.fulfill({json:activity}));
   // Native Go serves this generated route; it is not a static-file asset.
   await page.route('**/manifest.json',r=>r.fulfill({json:{name:'Gi',short_name:'Gi',start_url:'/',display:'standalone'}}));
  }
  try{
   const scoped=hostName==='gi'?page.waitForRequest(r=>r.url().includes('/sse/stream?chat_jid=gi%3Amain')):null;
   await page.goto(host.origin);await host.connected();await page.locator('.compose-box textarea').fill('unsent draft');
   if(scoped){await scoped;await page.waitForTimeout(100);await host.connected();}
   await page.locator('.compose-box textarea').blur();
   if(hostName==='piclaw'){piclawStatus={status:'active',data:{...running,chat_jid:state.sessionId,turn_id:'output-turn'}};host.emit('agent_status',piclawStatus.data);}
   else{activity=runningActivity;host.emit('agent_status',{...activity,chat_jid:'gi:main'});}
   host.emit('agent_thought_delta',{turn_id:'output-turn',delta:'Inspect tool arguments before response.'});
   host.emit('agent_draft_delta',{turn_id:'output-turn',delta:'Draft response stays in progress.'});
   const thought=page.locator('[data-panel-key="thought"]'),draft=page.locator('[data-panel-key="draft"]');
   await thought.getByText('Inspect tool arguments before response.',{exact:false}).waitFor();
   await draft.getByText('Draft response stays in progress.',{exact:false}).waitFor();
   const pane=page.locator('[data-panel-key="tool-output"]');await pane.waitFor().catch(async error=>{await fs.writeFile(path.join(output,'failure.json'),JSON.stringify({hostName,browserName,body:await page.locator('body').innerText(),panes:await page.locator('[data-panel-key]').evaluateAll(es=>es.map(e=>({key:e.dataset.panelKey,rect:e.getBoundingClientRect().toJSON(),display:getComputedStyle(e).display,html:e.outerHTML.slice(0,800)}))),failures:host.failures,calls:host.calls},null,2));throw error;});
   const snap=()=>pane.evaluate(e=>({html:e.innerHTML,text:e.textContent,styles:[e,e.querySelector('.agent-thinking-body')].map(n=>{const s=getComputedStyle(n);return {fontFamily:s.fontFamily,fontSize:s.fontSize,whiteSpace:s.whiteSpace,overflowX:s.overflowX}})}));
   await page.waitForTimeout(100);const collapsed=await snap();
   assert(collapsed.text.includes('line-06')&&!collapsed.text.includes('line-00'),'collapsed tail is six lines');
   assert.equal(await pane.locator('script').count(),0,'output HTML must remain sanitized');
   await pane.locator('button.agent-thinking-truncation').click();await page.waitForTimeout(50);const expanded=await snap();
   assert(expanded.text.includes('line-00')&&expanded.text.includes('x'.repeat(170)),'expanded preview preserves long lines');
   await page.screenshot({path:path.join(output,`${browserName}-${viewportName}-${hostName}.png`)});
   if(hostName==='piclaw'){piclawStatus={status:'active',data:{...waiting,chat_jid:state.sessionId,turn_id:'output-turn'}};host.emit('agent_status',piclawStatus.data);}
   else{activity={...activity,tool:{...activity.tool,state:'completed'}};host.emit('agent_status',{...activity,chat_jid:'gi:main'});}
   await page.getByText('Waiting for model...', {exact:false}).first().waitFor();assert.equal(await pane.count(),0,'tool completion removes Output without completing turn');
   assert((await thought.textContent()).includes('Inspect tool arguments before response.'),'thought survives the intra-turn wait');
   assert((await draft.textContent()).includes('Draft response stays in progress.'),'draft survives the intra-turn wait');
   assert.equal(await page.locator('.post').count(),0,'tool result must not become a conversation post');
   if(hostName==='piclaw'){piclawStatus={status:'idle',data:null};host.emit('agent_status',{type:'done',chat_jid:state.sessionId,turn_id:'output-turn'});}
   else{activity={status:'idle',turn_id:'output-turn'};host.emit('agent_status',{...activity,chat_jid:'gi:main'});}
   await page.waitForFunction(()=>document.querySelectorAll('.agent-status-panel').length===0);
   assert.equal(await pane.count(),0);assert.equal(await thought.count(),0);assert.equal(await draft.count(),0);
   assert.equal(await page.locator('.compose-box textarea').inputValue(),'unsent draft');host.assert();
   observations[hostName]={collapsed,expanded};
  }finally{await host.dispose();await page.close();}
 }
 // DOM/text/style comparison, not generated asset/screenshot byte equality.
 assert.deepEqual(observations.gi,observations.piclaw,`${browserName}/${viewportName}: Output contract differs`);
 report.cases.push({browser:browserName,viewport:viewportName,result:'pass',observations});
 }finally{await browser.close();await fs.writeFile(path.join(output,'evidence.json'),JSON.stringify(report,null,2));}
}
console.log(JSON.stringify({cases:report.cases.map(({browser,viewport,result})=>({browser,viewport,result})),output}));
