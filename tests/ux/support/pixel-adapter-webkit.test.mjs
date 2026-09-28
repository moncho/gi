import {test,expect} from 'bun:test';
import {EventEmitter} from 'node:events';
import {installPixelHost} from './pixel-adapter.mjs';
import state from '../fixtures/compose-pixel-state.json';
import reference from '../fixtures/compose-pixel-reference.json';
test('WebKit cancellation is admitted only once for unscoped Gi bootstrap; scoped failure is fatal',async()=>{
 const page=new EventEmitter();page.addInitScript=async()=>{};page.route=async()=>{};page.unroute=async()=>{};
 const a=await installPixelHost({page,host:'gi',root:process.cwd(),state,reference});
 try{
  const fail=path=>page.emit('requestfailed',{url:()=>a.origin+path,failure:()=>({errorText:'Load request cancelled'})});
  fail('/sse/stream');expect(a.streamAborts).toHaveLength(1);expect(a.failures).toHaveLength(0);expect(()=>a.assert()).toThrow('No connected fixture stream');
  fail('/sse/stream?chat_jid=gi%3Amain');fail('/sse/stream');expect(a.failures).toHaveLength(2);
 }finally{await a.dispose();}
});
