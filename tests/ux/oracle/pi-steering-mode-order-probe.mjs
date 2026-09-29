// Installed Pi 0.87.1 agent-loop steering dequeue with disposable responses.
// This checks request boundaries, not TUI dispatch, persistence or Piclaw HTTP.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {pathToFileURL} from 'node:url';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
const pi=path.join(root,'app/node_modules/@earendil-works/pi-coding-agent');
assert.equal(JSON.parse(await fs.readFile(path.join(pi,'package.json'),'utf8')).version,'0.87.1');
const {runAgentLoop}=await import(pathToFileURL(path.join(root,'app/node_modules/@earendil-works/pi-agent-core/dist/agent-loop.js')).href);
const {createAssistantMessageEventStream}=await import(pathToFileURL(path.join(root,'app/node_modules/@earendil-works/pi-ai/dist/utils/event-stream.js')).href);
const cases=[];
for(const mode of ['one-at-a-time','all']) {
 let queue=[],polls=0;const requests=[];
 const config={model:{id:'fixture',provider:'openai',api:'openai-completions'},convertToLlm:async messages=>messages,
  getSteeringMessages:async()=>{polls++;const count=mode==='all'?queue.length:1;return queue.splice(0,count);}};
 const streamFn=(_model,context)=>{
  const index=requests.length+1;
  requests.push(context.messages.filter(m=>m.role==='user').map(m=>m.content?.find(b=>b.type==='text')?.text));
  if(index===1)queue=['steer one','steer two'].map(text=>({role:'user',content:[{type:'text',text}],timestamp:Date.now()}));
  const message={role:'assistant',content:[{type:'text',text:`answer ${index}`}],api:'openai-completions',provider:'openai',model:'fixture',usage:{input:0,output:0,cacheRead:0,cacheWrite:0,totalTokens:0,cost:{input:0,output:0,cacheRead:0,cacheWrite:0,total:0}},stopReason:'stop',timestamp:Date.now()};
  const stream=createAssistantMessageEventStream();queueMicrotask(()=>stream.push({type:'done',reason:'stop',message}));return stream;
 };
 await runAgentLoop([{role:'user',content:[{type:'text',text:'first request'}],timestamp:Date.now()}],{systemPrompt:'fixture',messages:[],tools:[]},config,()=>{},undefined,streamFn);
 assert.deepEqual(requests,mode==='all'
  ?[['first request'],['first request','steer one','steer two']]
  :[['first request'],['first request','steer one'],['first request','steer one','steer two']]);
 cases.push({mode,requests,polls});
}
console.log(JSON.stringify({scope:'Installed Pi 0.87.1 agent-loop method, steering queued during first direct reply; no TUI, live provider or durable store.',cases},null,2));
