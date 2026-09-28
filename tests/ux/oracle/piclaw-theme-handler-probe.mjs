// Installed Piclaw 3.2.4 single-user theme handler in an in-memory DB and
// temporary workspace. Calls the handler directly; no live HTTP server/chat.
import assert from 'node:assert/strict';
import {mkdtemp,readFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import path from 'node:path';
import {pathToFileURL} from 'node:url';

assert.equal(process.env.PICLAW_DB_IN_MEMORY,'1','set PICLAW_DB_IN_MEMORY=1');
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const workspace=await mkdtemp(path.join(tmpdir(),'piclaw-theme-handler-'));
process.env.PICLAW_WORKSPACE=workspace;
const source=file=>pathToFileURL(path.join(root,'app/runtime/src',file)).href;
const {initDatabase,getDb,closeDatabase}=await import(source('db/connection.ts'));
const {extensionKvGet}=await import(source('db/extension-kv.ts'));
const {handleAgentMessage}=await import(source('channels/web/handlers/agent.ts'));
const {getServerUiThemeConfig}=await import(source('channels/web/ui-state.ts'));
try{
 initDatabase();const db=getDb();
 assert.equal(db.query('PRAGMA database_list').all().find(row=>row.name==='main').file,'');
 const events=[],timeline=[];let agentCalls=0;
 const channel={
  agentPool:{isStreaming:()=>{agentCalls++;return false;},isActive:()=>{agentCalls++;return false;}},
  getQueuedFollowupCount:()=>0,
  broadcastEvent:(type,payload)=>events.push({type,payload}),
  sendMessage:async(chat,text,options)=>{timeline.push({chat,text,options});},
  json:(body,status=200)=>Response.json(body,{status}),
 };
 const run=async content=>{
  const req=new Request('http://fixture/agent/default/message?chat_jid=web%3Adefault',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({content})});
  const response=await handleAgentMessage(channel,req,'/agent/default/message','web:default','default');
  return {status:response.status,body:await response.json()};
 };
 const theme=await run('/theme ristretto');
 assert.equal(theme.status,200);assert.equal(theme.body.ui_only,true);
 assert.deepEqual(theme.body.command.payload,{theme:'ristretto',tint:null});
 assert.deepEqual(getServerUiThemeConfig(),{theme:'ristretto',tint:null});
 assert.deepEqual(extensionKvGet('piclaw-ui','theme','global'),{theme:'ristretto',tint:null});
 assert.equal(JSON.parse(await readFile(path.join(workspace,'.piclaw','config.json'),'utf8')).ui.theme,'ristretto');
 const tint=await run('/tint orange');
 assert.equal(tint.status,200);assert.deepEqual(tint.body.command.payload,{theme:'default',tint:'orange'});
 assert.deepEqual(getServerUiThemeConfig(),{theme:'default',tint:'orange'});
 assert.deepEqual(extensionKvGet('piclaw-ui','theme','global'),{theme:'default',tint:'orange'});
 const invalid=await run('/tint $$notacolor');
 assert.equal(invalid.status,200);assert.equal(invalid.body.command.status,'error');assert.equal(invalid.body.command.payload,undefined);
 assert.deepEqual(getServerUiThemeConfig(),{theme:'default',tint:'orange'});
 const listed=await run('/theme');
 assert.equal(listed.status,200);assert.match(listed.body.command.message,/Available themes/);
 assert.deepEqual(getServerUiThemeConfig(),{theme:'default',tint:'orange'});
 assert.deepEqual(events.map(event=>event.type),['ui_theme','ui_theme']);
 assert.deepEqual(timeline.map(row=>row.chat),Array(4).fill('web:default'));
 assert(timeline.every(row=>row.options?.forceRoot===true));
 assert.equal(agentCalls,8,'handler reads active flags but does not enqueue or invoke an agent');
 console.log(JSON.stringify({oracle:'Installed Piclaw 3.2.4 handler, in-memory DB and temporary config',commands:[theme.body.command,tint.body.command,invalid.body.command,{status:listed.body.command.status,message:'Available themes'}],events:events.map(event=>event.type),timeline:timeline.map(row=>({chat:row.chat,forceRoot:row.options.forceRoot})),persisted:getServerUiThemeConfig(),result:'pass'},null,2));
}finally{closeDatabase();await rm(workspace,{recursive:true,force:true});}
