import {test,expect} from 'bun:test';
import {EventEmitter} from 'node:events';
import {installPixelHost} from './pixel-adapter.mjs';
import state from '../fixtures/compose-pixel-state.json';
import reference from '../fixtures/compose-pixel-reference.json';
class Page extends EventEmitter{
 async addInitScript(){} async route(_,handler){this.handler=handler;} async unroute(){this.handler=null;}
 async request(origin,path,method='GET'){
  let response,continued=false;
  await this.handler({request:()=>({url:()=>origin+path,method:()=>method}),fulfill:async value=>{response=value;},continue:async()=>{continued=true;}});
  return {response,continued};
 }
}
async function fixture(host,run){const page=new Page(),adapter=await installPixelHost({page,host,root:process.cwd(),state,reference});try{await run(page,adapter);}finally{await adapter.dispose();expect(page.listenerCount('pageerror')).toBe(0);expect(page.listenerCount('requestfailed')).toBe(0);expect(page.handler).toBeNull();}}
test('native snapshot has array errors and equal model/session identities',()=>fixture('piclaw',async(page,a)=>{
 const {response}=await page.request(a.origin,'/agent/status?chat_jid=gi%3Amain&ui=1');
 expect(response.json.errors).toEqual([]);expect(response.json.model).toEqual(state.model);expect(response.json.agent_name).toBe(state.agentName);
 const sessions=await page.request(a.origin,'/agent/active-chats');expect(sessions.response.json.chats[0].agent_name).toBe(state.sessionLabel);expect(sessions.response.json.chats[0].model).toBe(state.model.current);
 expect(a.failures).toEqual([]);
}));
test('both pixel hosts receive identical known context capacity using native camelCase',async()=>{
 let native,classic;
 await fixture('gi',async(page,a)=>{native=(await page.request(a.origin,'/api/sessions/main/model')).response.json.context_usage;});
 await fixture('piclaw',async(page,a)=>{classic=(await page.request(a.origin,'/agent/context')).response.json;});
 expect(native).toEqual({tokens:0,contextWindow:65536,percent:0});
 expect(classic.contextWindow).toBe(native.contextWindow);expect(classic.tokens).toBe(native.tokens);expect(classic.percent).toBe(native.percent);
 expect(native.context_window).toBeUndefined();
});
test('undeclared writes, origins and wrong session scopes fail closed',()=>fixture('gi',async(page,a)=>{
 await page.request(a.origin,'/api/sessions','POST');await page.request('https://example.invalid','/api/sessions');await page.request(a.origin,'/api/sessions?chat_jid=wrong');
 expect(a.failures).toHaveLength(3);expect(()=>a.assert()).toThrow();
}));
test('page errors and failed network requests are capture failures',()=>fixture('gi',async(page,a)=>{
 page.emit('pageerror',Error('boom'));page.emit('requestfailed',{url:()=>a.origin+'/dist/app.js',failure:()=>({errorText:'net::ERR_ABORTED'})});
 expect(a.failures).toHaveLength(2);expect(()=>a.assert()).toThrow();
}));
test('expected stream abort is recorded but cannot replace a live stream',()=>fixture('gi',async(page,a)=>{
 page.emit('requestfailed',{url:()=>a.origin+'/sse/stream',failure:()=>({errorText:'net::ERR_ABORTED'})});
 expect(a.streamAborts).toHaveLength(1);expect(a.failures).toEqual([]);expect(()=>a.assert()).toThrow('No connected fixture stream');
 page.emit('requestfailed',{url:()=>a.origin+'/sse/stream',failure:()=>({errorText:'net::ERR_ABORTED'})});expect(a.failures).toHaveLength(1);
}));
test('native unload-beacon bypass is opt-in; existing routed presence and visibility calls stay explicit',async()=>{
 for(const allowPresenceBeacon of [false,true]){
  const page=new Page(),a=await installPixelHost({page,host:'piclaw',root:process.cwd(),state,reference,allowPresenceBeacon});
  try{
   const res=await fetch(a.origin+'/agent/push/presence',{method:'POST',body:'{}'});
   expect(res.status).toBe(allowPresenceBeacon?200:404);
   expect((await fetch(a.origin+'/agent/default/message',{method:'POST',body:'{}'})).status).toBe(404);
   expect(a.calls.filter(c=>c.nativeBeacon)).toHaveLength(allowPresenceBeacon?1:0);
   // Normal routed presence is an existing fixture API allowance. This option
   // only admits unload sendBeacon calls that bypass Playwright routing.
   expect((await page.request(a.origin,'/agent/push/presence','POST')).response.json).toEqual({ok:true});
   expect((await page.request(a.origin,'/workspace/visibility','POST')).response.json).toEqual({ok:true});
   expect((await page.request(a.origin,'/agent/default/message','POST')).response.status).toBe(500);
  }finally{await a.dispose();}
 }
});

test('Piclaw stream and topic aborts are never waived',async()=>{
 for(const host of ['piclaw','gi'])await fixture(host,async(page,a)=>{
  page.emit('requestfailed',{url:()=>a.origin+'/sse/topics',failure:()=>({errorText:'net::ERR_ABORTED'})});
  expect(a.failures).toHaveLength(1);
  if(host==='piclaw'){page.emit('requestfailed',{url:()=>a.origin+'/sse/stream',failure:()=>({errorText:'net::ERR_ABORTED'})});expect(a.failures).toHaveLength(2);}
 });
});
