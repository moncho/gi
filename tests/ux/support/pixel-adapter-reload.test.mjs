import {test,expect} from 'bun:test';
import {EventEmitter} from 'node:events';
import {installPixelHost} from './pixel-adapter.mjs';
import state from '../fixtures/compose-pixel-state.json';
import reference from '../fixtures/compose-pixel-reference.json';
function page(browser='webkit'){
 const p=new EventEmitter();p.addInitScript=async()=>{};p.route=async()=>{};p.unroute=async()=>{};
 p.waitForTimeout=ms=>new Promise(resolve=>setTimeout(resolve,ms));
 p.context=()=>({browser:()=>({browserType:()=>({name:()=>browser})})});return p;
}
function request(origin,path,method='GET'){
 return {url:()=>origin+path,method:()=>method,failure:()=>({errorText:'Load request cancelled'})};
}
const fixtureState={...state,sessionId:'web:default'};
test('explicit Piclaw WebKit reload admits old stream and one presence unload only with replacement',async()=>{
 const p=page(),a=await installPixelHost({page:p,host:'piclaw',root:process.cwd(),state:fixtureState,reference,allowWebkitReloadUnload:true});
 let oldReader,newReader;
 try{
  const old=request(a.origin,'/sse/stream?chat_jid=web%3Adefault');p.emit('request',old);
  oldReader=(await fetch(old.url())).body.getReader();await oldReader.read();a.assert();a.beginReload();
  await oldReader.cancel();p.emit('requestfailed',old);
  const presence=request(a.origin,'/agent/push/presence','POST');p.emit('request',presence);p.emit('requestfailed',presence);
  p.emit('pageerror',Error(`/${new URL(a.origin).host}/agent/push/presence due to access control checks.`));
  const next=request(a.origin,'/sse/stream?chat_jid=web%3Adefault');p.emit('request',next);
  newReader=(await fetch(next.url())).body.getReader();await newReader.read();
  await a.endReload();expect(a.failures).toEqual([]);
  expect(a.reloadUnloadEvents.map(event=>event.kind)).toEqual(['stream','presence','presencePageError','reloadEnd']);
  p.emit('requestfailed',request(a.origin,'/sse/stream?chat_jid=web%3Adefault'));
  p.emit('requestfailed',request(a.origin,'/agent/push/presence','POST'));
  expect(a.failures).toHaveLength(2);expect(()=>a.assert()).toThrow();
 }finally{await oldReader?.cancel().catch(()=>{});await newReader?.cancel().catch(()=>{});await a.dispose();}
});
test('WebKit reload rejects untracked stream, topics, unrelated writes and page errors',async()=>{
 const p=page(),a=await installPixelHost({page:p,host:'piclaw',root:process.cwd(),state:fixtureState,reference,allowWebkitReloadUnload:true});
 try{
  a.beginReload();
  p.emit('requestfailed',request(a.origin,'/sse/stream?chat_jid=web%3Adefault'));
  p.emit('requestfailed',request(a.origin,'/sse/topics'));
  p.emit('requestfailed',request(a.origin,'/agent/default/message','POST'));
  p.emit('pageerror',Error('unexpected'));
  expect(a.reloadUnloadEvents).toEqual([]);expect(a.failures).toHaveLength(4);expect(()=>a.assert()).toThrow();
 }finally{await a.dispose();}
});
test('repeated failures inside a WebKit reload window are fatal',async()=>{
 const p=page(),a=await installPixelHost({page:p,host:'piclaw',root:process.cwd(),state:fixtureState,reference,allowWebkitReloadUnload:true});
 let reader;
 try{
  const old=request(a.origin,'/sse/stream?chat_jid=web%3Adefault');p.emit('request',old);
  reader=(await fetch(old.url())).body.getReader();await reader.read();a.beginReload();
  await reader.cancel();p.emit('requestfailed',old);p.emit('requestfailed',old);
  const presence=request(a.origin,'/agent/push/presence','POST');p.emit('requestfailed',presence);p.emit('requestfailed',presence);
  p.emit('pageerror',Error(`/${new URL(a.origin).host}/agent/push/presence due to access control checks.`));
  p.emit('pageerror',Error(`/${new URL(a.origin).host}/agent/push/presence due to access control checks.`));
  expect(a.reloadUnloadEvents.map(event=>event.kind)).toEqual(['stream','presence','presencePageError']);
  expect(a.failures).toHaveLength(3);expect(()=>a.assert()).toThrow();
 }finally{await reader?.cancel().catch(()=>{});await a.dispose();}
});
test('non-WebKit hosts cannot open the WebKit reload allowance',async()=>{
 for(const browser of ['chromium','webkit']){
  const p=page(browser),a=await installPixelHost({page:p,host:browser==='webkit'?'gi':'piclaw',root:process.cwd(),state:fixtureState,reference,allowWebkitReloadUnload:true});
  try{expect(()=>a.beginReload()).toThrow();}finally{await a.dispose();}
 }
});
