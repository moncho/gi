// Installed Pi 0.87.1 agent-core method with a disposable provider and tools.
// Delivers one steering message during the first of two sequential tool calls.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {pathToFileURL} from 'node:url';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
const pi=path.join(root,'app/node_modules/@earendil-works/pi-coding-agent');
assert.equal(JSON.parse(await fs.readFile(path.join(pi,'package.json'),'utf8')).version,'0.87.1');
const {runAgentLoop}=await import(pathToFileURL(path.join(root,'app/node_modules/@earendil-works/pi-agent-core/dist/agent-loop.js')).href);
const {createAssistantMessageEventStream}=await import(pathToFileURL(path.join(root,'app/node_modules/@earendil-works/pi-ai/dist/utils/event-stream.js')).href);
const events=[], trace=[];let steering=[],requests=0;
const fixtureSteer={role:'user',content:[{type:'text',text:'steer at tool boundary'}],timestamp:Date.now()};
const model={id:'fixture',provider:'openai',api:'openai-completions'};
const tools=['one','two'].map(name=>({name,label:name,description:name,parameters:{type:'object',properties:{}},executionMode:'sequential',async execute(id,args){trace.push('execute:'+name);if(name==='one')steering=[fixtureSteer];return{content:[{type:'text',text:'result '+name}],details:{}};}}));
const config={model,tools,convertToLlm:async messages=>messages,getSteeringMessages:async()=>{trace.push('poll-steering');return steering.splice(0);},toolExecution:'sequential'};
const streamFn=()=>{
  requests++;trace.push('request:'+requests);
  const calls=requests===1?tools.map((tool,i)=>({type:'toolCall',id:'call-'+i,name:tool.name,arguments:{}})):[];
  const message={role:'assistant',content:calls.length?calls:[{type:'text',text:'done'}],api:'openai-completions',provider:'openai',model:'fixture',usage:{input:0,output:0,cacheRead:0,cacheWrite:0,totalTokens:0,cost:{input:0,output:0,cacheRead:0,cacheWrite:0,total:0}},stopReason:calls.length?'toolUse':'stop',timestamp:Date.now()};
  const stream=createAssistantMessageEventStream();queueMicrotask(()=>stream.push({type:'done',reason:message.stopReason,message}));return stream;
};
await runAgentLoop([{role:'user',content:[{type:'text',text:'first'}],timestamp:Date.now()}],{systemPrompt:'fixture',messages:[],tools},config,event=>{events.push(event);trace.push('event:'+event.type+(event.toolName?':'+event.toolName:''));},undefined,streamFn);
const index=value=>trace.indexOf(value);
assert.equal(requests,2);
assert(index('execute:one')<index('execute:two'));
assert(index('execute:two')<index('event:turn_end'));
assert(index('event:turn_end')<trace.lastIndexOf('poll-steering'));
assert.equal(events.filter(e=>e.type==='tool_execution_start').length,2);
assert.equal(events.filter(e=>e.type==='tool_execution_end').length,2);
assert.equal(events.filter(e=>e.type==='message_end'&&e.message===fixtureSteer).length,1);
assert.equal(events.filter(e=>e.type==='message_end'&&e.message?.role==='toolResult').length,2);
console.log(JSON.stringify({scope:'Installed Pi 0.87.1 agent loop, two sequential disposable tools; steering queued during first. No TUI, Piclaw backend or provider.',requests,trace},null,2));
