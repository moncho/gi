// Installed Pi provider, disposable Anthropic client: no external requests.
import fs from 'node:fs/promises';
import {pathToFileURL} from 'node:url';
import assert from 'node:assert/strict';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
const pkg=JSON.parse(await fs.readFile(root+'/app/node_modules/@earendil-works/pi-ai/package.json','utf8'));
const {stream}=await import(pathToFileURL(root+'/app/node_modules/@earendil-works/pi-ai/dist/api/anthropic-messages.js'));
const requests=[];
const events=[{type:'message_start',message:{id:'fixture',usage:{input_tokens:2,output_tokens:0}}},{type:'content_block_start',index:0,content_block:{type:'tool_use',id:'empty',name:'noop',input:{}}},{type:'content_block_stop',index:0},{type:'message_delta',delta:{stop_reason:'tool_use'},usage:{output_tokens:1}},{type:'message_stop'}];
const create=(payload)=>{requests.push(payload);return {asResponse:async()=>new Response(events.map(event=>`event: ${event.type}\ndata: ${JSON.stringify(event)}\n\n`).join(''),{headers:{'Content-Type':'text/event-stream'}})};};
const client={messages:{create},beta:{messages:{create}}};
const model={id:'fixture',name:'Fixture',api:'anthropic-messages',provider:'anthropic',baseUrl:'http://fixture',reasoning:false,input:['text'],contextWindow:10000,maxTokens:64,cost:{input:0,output:0,cacheRead:0,cacheWrite:0}};
const context={messages:[{role:'user',content:'noop',timestamp:Date.now()}],tools:[{name:'noop',description:'No arguments',parameters:{type:'object',properties:{}}}]};
const first=await stream(model,context,{client}).result();assert.equal(first.stopReason,'toolUse',first.errorMessage);assert.deepEqual(first.content[0].arguments,{});
context.messages.push(first,{role:'toolResult',toolCallId:'empty',toolName:'noop',content:[{type:'text',text:'ok'}],isError:false,timestamp:Date.now()});
await stream(model,context,{client}).result();
const block=requests[1].messages.flatMap(m=>Array.isArray(m.content)?m.content:[]).find(b=>b.type==='tool_use');assert.deepEqual(block.input,{});
console.log(JSON.stringify({version:pkg.version,result:'pass',arguments:first.content[0].arguments,nextRequestInput:block.input,scope:'Installed Pi Anthropic stream/encoder with injected client, no live provider.'}));
