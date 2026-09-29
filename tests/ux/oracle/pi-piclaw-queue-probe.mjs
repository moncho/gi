// Invoke installed production methods with disposable in-memory collaborators.
// This is method-level evidence, not PTY/browser or live backend acceptance.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {pathToFileURL} from 'node:url';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
const pi=path.join(root,'app/node_modules/@earendil-works/pi-coding-agent');
const pkg=JSON.parse(await fs.readFile(path.join(pi,'package.json'),'utf8'));
assert.equal(pkg.version,'0.87.1');assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const {InteractiveMode}=await import(pathToFileURL(path.join(pi,'dist/modes/interactive/interactive-mode.js')));
const {TruncatedText}=await import(pathToFileURL(path.join(root,'app/node_modules/@earendil-works/pi-tui/dist/components/truncated-text.js')));
const {initTheme}=await import(pathToFileURL(path.join(pi,'dist/modes/interactive/theme/theme.js')));initTheme('dark');
const {AgentSession}=await import(pathToFileURL(path.join(pi,'dist/core/agent-session.js')));
const {WebAgentControlPlaneService}=await import(pathToFileURL(path.join(root,'app/runtime/src/channels/web/agent/agent-control-plane-service.ts')));
const out=path.resolve(process.env.ORACLE_OUTPUT||`test-results/ux-oracle/queue-semantics/run-${Date.now()}`);await fs.mkdir(out,{recursive:true});
const results=[];
const record=async(name,f)=>{const evidence=await f();results.push({name,result:'pass',evidence});};
await record('Pi Alt+Enter selects followUp while streaming and ordinary submit while idle',async()=>{
 const cases=[];
 for(const streaming of [true,false]){
  let text='  next task  ';const calls=[];
  const shell={editor:{getText:()=>text,setText:t=>text=t,addToHistory:t=>calls.push(['history',t]),onSubmit:t=>calls.push(['submit',t])},session:{isStreaming:streaming,isCompacting:false,prompt:async(t,o)=>calls.push(['prompt',t,o])},updatePendingMessagesDisplay(){calls.push(['display']);},ui:{requestRender(){}}};
  await InteractiveMode.prototype.handleFollowUp.call(shell);
  assert.equal(text,'');assert(calls.some(c=>streaming?c[0]==='prompt'&&c[2].streamingBehavior==='followUp':c[0]==='submit'));cases.push({streaming,calls});
 }
 return cases;
});
await record('Pi pending rows take first line, truncate by display width and keep grapheme clusters',async()=>{
 const children=[];
 const shell={pendingMessagesContainer:{clear(){children.length=0;},addChild(child){children.push(child);}},getAllQueuedMessages(){return{steering:['e\u0301ax','🧑‍💻abc'],followUp:['first\nsecond']};},getAppKeyDisplay:()=> 'Alt+Up'};
 InteractiveMode.prototype.updatePendingMessagesDisplay.call(shell);
 assert.equal(children.length,5);
 // TruncatedText has one cell of horizontal padding on each side here.
 const plain=(index,width)=>children[index].render(width)[0].replace(/\x1b\[[0-9;]*m/g,'').trim().replace(/ +$/,'');
 const cases=[
  {index:1,width:14,want:'Steering:...'},
  {index:2,width:15,want:'Steering: ...'},
  {index:2,width:32,want:'Steering: 🧑‍💻abc'},
  {index:3,width:32,want:'Follow-up: first'},
  {index:4,width:32,want:'↳ Alt+Up to edit all queued...'},
 ];
 for(const item of cases)assert.equal(plain(item.index,item.width),item.want,JSON.stringify(item));
 return cases.map(({index,width,want})=>({index,width,rendered:plain(index,width),want}));
});
await record('Pi dequeue restores all steering then followUps before newer editor text',async()=>{
 let text='newer draft';let cleared=0,aborted=0;
 const shell={clearAllQueues(){cleared++;return{steering:['steer one','steer two'],followUp:['follow one','follow two']};},editor:{getText:()=>text,setText:t=>text=t},updatePendingMessagesDisplay(){},session:{abort:async()=>aborted++}};
 const count=InteractiveMode.prototype.restoreQueuedMessagesToEditor.call(shell,{abort:true});
 assert.equal(count,4);assert.equal(text,'steer one\n\nsteer two\n\nfollow one\n\nfollow two\n\nnewer draft');assert.equal(cleared,1);assert.equal(aborted,1);return{count,text,cleared,aborted};
});
await record('Pi session queues have distinct delivery paths and preserve image blocks',async()=>{
 const delivered=[];const shell={_steeringMessages:[],_followUpMessages:[],_emitQueueUpdate(){},agent:{steer:m=>delivered.push(['steer',m]),followUp:m=>delivered.push(['followUp',m]),clearAllQueues(){delivered.push(['clear']);}}};
 const image={type:'image',data:'fixture',mimeType:'image/png'};
 await AgentSession.prototype._queueSteer.call(shell,'direction',[image]);await AgentSession.prototype._queueFollowUp.call(shell,'later');
 assert.equal(delivered[0][0],'steer');assert.deepEqual(delivered[0][1].content[1],image);assert.equal(delivered[1][0],'followUp');
 const restored=AgentSession.prototype.clearQueue.call(shell);assert.deepEqual(restored,{steering:['direction'],followUp:['later']});assert.equal(shell._steeringMessages.length+shell._followUpMessages.length,0);return{delivered,restored};
});
for(const mode of ['active','idle','ended-before-queue','handoff-while-queue-call','store-failure','absent'])await record('Piclaw queue Steer: '+mode,async()=>{
 let item=mode==='absent'?null:{rowId:41,queuedContent:' queued instruction ',queuedAt:'2026-01-01',threadId:7,mediaIds:[9],contentBlocks:[{type:'text',text:'metadata'}],screenHint:'fixture'};const calls=[];const events=[];
 const lifecycle={async removeQueuedFollowupForAction(chat,row){calls.push(['remove',chat,row]);let removed=item;item=null;return{removed,source:'deferred'};},getQueuedFollowupCount(){return item?1:0;},prependQueuedFollowupItem(chat,row){item=row;calls.push(['restore',chat]);}};
 const options={defaultChatJid:'web:default',defaultAgentId:'default',json:(data,status=200)=>Response.json(data,{status}),queuedFollowupLifecycle:lifecycle,agentPool:{isStreaming:()=>mode!=='idle',queueStreamingMessage:async(chat,text,kind)=>{calls.push(['queue',chat,text,kind]);if(mode==='handoff-while-queue-call')calls.push(['run-ended-before-queue-return']);return{queued:mode==='active'};}},getInflightMessageId:()=>null,storeMessage:(chat,text,bot,media,opts)=>{calls.push(['store',chat,text,bot,media,opts]);return mode==='store-failure'?null:{id:101,timestamp:'2026-01-02',data:{thread_id:7}};},broadcastEvent:(type,data)=>events.push({type,data}),queuePendingSteering:(...a)=>calls.push(['pending',...a]),queue:{enqueue(fn,key,scope){calls.push(['enqueue',key,scope]);}},processChat:async()=>{throw Error('fixture must not execute inference');}};
 const response=await new WebAgentControlPlaneService(options).handleAgentQueueSteer(new Request('http://fixture/agent/queue-steer',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({chat_jid:'web:default',row_id:41})}));const body=await response.json();
 if(mode==='absent'){assert.equal(response.status,200);assert.equal(body.removed,false);assert.equal(calls.length,1);}
 else if(mode==='store-failure'){assert.equal(response.status,500);assert(item);assert(calls.some(c=>c[0]==='restore'));assert(!calls.some(c=>c[0]==='enqueue'||c[0]==='queue'));}
 else{assert.equal(response.status,201);assert.equal(body.removed,true);assert.equal(item,null);assert.deepEqual(calls.find(c=>c[0]==='store')[4],[9]);if(mode==='active'){assert.equal(body.queued,'steer');assert(!calls.some(c=>c[0]==='enqueue'));}else assert(calls.some(c=>c[0]==='enqueue'));if(mode==='handoff-while-queue-call')assert(calls.findIndex(c=>c[0]==='run-ended-before-queue-return')<calls.findIndex(c=>c[0]==='enqueue'));}
 return{mode,status:response.status,body,calls,events};
});
await fs.writeFile(path.join(out,'evidence.json'),JSON.stringify({pi:pkg.version,piclaw:'3.2.4',scope:'Installed methods with in-memory collaborators. Does not prove keyboard dispatch, queue persistence, tool execution ordering or complete UI parity.',results},null,2));console.log(JSON.stringify({out,checks:results.map(r=>r.name)},null,2));
