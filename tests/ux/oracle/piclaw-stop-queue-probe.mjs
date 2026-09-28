import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import assert from 'node:assert/strict';
const out=path.resolve('test-results/ux-oracle/stop-queue');await fs.mkdir(out,{recursive:true});
const isolated=await fs.mkdtemp(path.join(os.tmpdir(),'piclaw-stop-oracle-'));
process.env.PICLAW_DB_IN_MEMORY='1';process.env.PICLAW_CONFIG_PATH=path.join(isolated,'config.json');await fs.writeFile(process.env.PICLAW_CONFIG_PATH,JSON.stringify({access:{mode:'single-user'}}));
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';assert.equal((await fs.readFile(root+'/VERSION','utf8')).trim(),'3.2.4');
const {WebAgentControlPlaneService}=await import(root+'/app/runtime/src/channels/web/agent/agent-control-plane-service.ts');
const {handleAbort}=await import(root+'/app/runtime/src/agent-control/handlers/control.ts');
const {finalizeSuccessfulProcessChatRun}=await import(root+'/app/runtime/src/channels/web/runtime/process-chat-finalization-runtime.ts');
const {initDatabase,closeDatabase}=await import(root+'/app/runtime/src/db.ts');
initDatabase();const cases=[];
try {
 for(const failure of [false,true]){
  const jid='web:stop-oracle-'+failure;let queue=[{rowId:42,queuedContent:'next instruction',queuedAt:'2026-01-01T00:00:00Z',mediaIds:[9],threadId:7}];const calls=[];
  const session={isCompacting:false,isStreaming:true,abort:async()=>{calls.push('abort');session.isStreaming=false;},abortRetry(){},abortBash(){},clearQueue(){throw Error('Stop must not clear queued work')}};
  const service=new WebAgentControlPlaneService({defaultChatJid:jid,json:(body,status=200)=>Response.json(body,{status}),getActiveTurnId:()=> 'observed',agentPool:{applyControlCommand:async(chat,command)=>{assert.equal(chat,jid);assert.equal(command.type,'abort');return handleAbort(session,command)}}});
  const response=await service.handleAgentRunAbort(new Request('http://fixture/agent/run-abort',{method:'POST',body:JSON.stringify({chat_jid:jid,turn_id:'observed'}),headers:{'Content-Type':'application/json'}}));assert.equal(response.status,200);assert.deepEqual(calls,['abort']);assert.equal(queue.length,1);
  const channel={agentPool:{getContextUsageForChat:async()=>null},consumePendingSteering:()=>[],saveState(){},setContextUsage(){},peekQueuedFollowupItem:()=>queue[0]||null,consumeQueuedFollowupItem:()=>queue.shift()||null,prependQueuedFollowupItem:(chat,item)=>queue.unshift(item),replaceQueuedFollowupItem:(chat,item)=>{queue[0]=item;return true},storeMessage:(chat,text,bot,media,options)=>{calls.push('store');assert.equal(text,'next instruction');assert.deepEqual(media,[9]);if(failure)return null;assert.equal(options.consumeDeferredFollowupRowId,42);queue.shift();return{id:100,timestamp:'2026-01-02',data:{}}},broadcastEvent(){},resumeChat:(chat,id)=>{assert.equal(chat,jid);assert.equal(id,100);calls.push('resumeChat')},sendMessage:async()=>{},updateAgentStatus(){},retryFailedOnModelSwitch(){}};
  await finalizeSuccessfulProcessChatRun({channel,emitter:{status:()=>calls.push('done')},chatJid:jid,agentId:'default',turnId:'observed',threadId:7,prevCursor:'',recovery:null});
  if(failure){assert.equal(queue.length,1);assert(!calls.includes('resumeChat'));assert.equal(queue[0].queuedContent,'next instruction')}else{assert.equal(queue.length,0);assert(calls.includes('resumeChat'))}
  cases.push({failure,calls,remaining:queue.length,requiresUserResume:false});
 }
 await fs.writeFile(path.join(out,'evidence.json'),JSON.stringify({version:'3.2.4',scope:'Installed abort service/control handler and finalization functions; in-memory DB and disposable collaborators, no live provider or DB.',cases},null,2));console.log(JSON.stringify({out,cases}));
}finally{closeDatabase();await fs.rm(isolated,{recursive:true,force:true})}
