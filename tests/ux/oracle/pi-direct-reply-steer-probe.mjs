// Installed Pi 0.87.1 agent loop with a disposable direct reply and steer.
// No tool calls, provider network, Piclaw backend, TUI, or durable storage.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {pathToFileURL} from 'node:url';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
const pi=path.join(root,'app/node_modules/@earendil-works/pi-coding-agent');
assert.equal(JSON.parse(await fs.readFile(path.join(pi,'package.json'),'utf8')).version,'0.87.1');
const {runAgentLoop}=await import(pathToFileURL(path.join(root,'app/node_modules/@earendil-works/pi-agent-core/dist/agent-loop.js')).href);
const {createAssistantMessageEventStream}=await import(pathToFileURL(path.join(root,'app/node_modules/@earendil-works/pi-ai/dist/utils/event-stream.js')).href);
const trace=[];let steering=[],requests=0;
const steer={role:'user',content:[{type:'text',text:'steer after direct reply'}],timestamp:Date.now()};
const config={model:{id:'fixture',provider:'openai',api:'openai-completions'},convertToLlm:async messages=>messages,getSteeringMessages:async()=>{trace.push('poll-steering');return steering.splice(0);}};
const streamFn=()=>{
 requests++;trace.push('request:'+requests);
 if(requests===1)steering=[steer];
 const message={role:'assistant',content:[{type:'text',text:requests===1?'first visible answer':'second answer'}],api:'openai-completions',provider:'openai',model:'fixture',usage:{input:0,output:0,cacheRead:0,cacheWrite:0,totalTokens:0,cost:{input:0,output:0,cacheRead:0,cacheWrite:0,total:0}},stopReason:'stop',timestamp:Date.now()};
 const stream=createAssistantMessageEventStream();queueMicrotask(()=>stream.push({type:'done',reason:'stop',message}));return stream;
};
await runAgentLoop([{role:'user',content:[{type:'text',text:'first request'}],timestamp:Date.now()}],{systemPrompt:'fixture',messages:[],tools:[]},config,event=>{if(event.type==='message_end')trace.push('message:'+event.message.role+':'+(event.message.content?.find(b=>b.type==='text')?.text||''));else trace.push('event:'+event.type);},undefined,streamFn);
const first=trace.indexOf('message:assistant:first visible answer'),steerIndex=trace.indexOf('message:user:steer after direct reply'),second=trace.indexOf('request:2');
assert.equal(requests,2);assert(first>=0&&steerIndex>first&&second>steerIndex);
console.log(JSON.stringify({scope:'Installed Pi 0.87.1 method with direct assistant reply and queued steering; no live provider, TUI or persistence.',requests,trace},null,2));
