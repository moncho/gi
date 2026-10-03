import {test,expect} from 'bun:test';
import {createComposeTransfers,bindComposeSending} from '../../../web/src/gi-compose-transfer';

test('transport phases are independent per session and concurrent completion cannot clear peers',()=>{
 const s=createComposeTransfers();let changes=0;const unsubscribe=s.subscribe(()=>changes++);
 const a=s.begin('a','upload','first.txt'),b=s.begin('b','upload'),peer=s.begin('a','upload','second.txt');
 a.progress(5,10,true);peer.progress(4,0,false);
 expect(s.snapshot('a')).toEqual({uploads:2,sending:0,loaded:9,total:10,computable:false,names:['first.txt','second.txt']});
 peer.end();a.progress(10,10,true);expect(s.snapshot('a')).toEqual({uploads:1,sending:0,loaded:10,total:10,computable:true,names:['first.txt']});
 a.end();const sending=s.begin('a','send');
 expect(s.snapshot('a').uploads).toBe(0);expect(s.snapshot('a').sending).toBe(1);expect(s.snapshot('b').uploads).toBe(1);
 a.progress(99,100,true);a.end();expect(s.snapshot('a').sending).toBe(1);
 const second=s.begin('a','send');sending.end();expect(s.snapshot('a').sending).toBe(1);second.end();b.end();
 expect(s.snapshot('a').sending).toBe(0);expect(s.snapshot('b').uploads).toBe(0);
 unsubscribe();const before=changes;s.begin('c','send').end();expect(changes).toBe(before);
});

test('transport cleanup never restores old Send labels over a newly active Stop control',()=>{
 const saved=globalThis.MutationObserver;
 globalThis.MutationObserver=class {observe(){} disconnect(){}} as any;
 try {
  const attrs=new Map([['aria-label','Send message'],['title','Send (Enter)']]);let active=false;
  const button:any={dataset:{},disabled:false,classList:{contains:()=>active},
   getAttribute:(k:string)=>attrs.get(k)??null,setAttribute:(k:string,v:string)=>attrs.set(k,v),removeAttribute:(k:string)=>attrs.delete(k)};
  const root:any={querySelector:(selector:string)=>selector.includes('textarea')?{value:'newer draft'}:button};
  const cleanup=bindComposeSending(root,false,true);
  expect(attrs.get('aria-label')).toBe('Sending message');expect(button.disabled).toBe(true);
  active=true;attrs.set('aria-label','Stop response');attrs.set('title','Stop response');button.disabled=false;
  cleanup();expect(attrs.get('aria-label')).toBe('Stop response');expect(attrs.get('title')).toBe('Stop response');expect(button.disabled).toBe(false);
 } finally {globalThis.MutationObserver=saved;}
});

test('unknown byte totals remain indeterminate and fresh reload has no work',()=>{
 const s=createComposeTransfers(),op=s.begin('a','upload');
 op.progress(NaN,Infinity,true);expect(s.snapshot('a')).toMatchObject({loaded:0,total:0,computable:false});
 op.progress(20,10,true);expect(s.snapshot('a')).toMatchObject({loaded:10,total:10,computable:true});
 expect(createComposeTransfers().snapshot('a')).toMatchObject({uploads:0,sending:0});
 op.end();expect(s.snapshot('a')).toMatchObject({uploads:0,sending:0});
});

test('upload batch cancellation is session-scoped, one-shot and cannot abort sending or newer batches',()=>{
 const transfers=createComposeTransfers(),a=transfers.beginUploadBatch('A'),b=transfers.beginUploadBatch('A'),other=transfers.beginUploadBatch('B');
 let later:any,events=0;a.signal.addEventListener('abort',()=>{events++;later=transfers.beginUploadBatch('A');});
 transfers.cancelUploads('A');expect(a.signal.aborted).toBe(true);expect(b.signal.aborted).toBe(true);expect(other.signal.aborted).toBe(false);expect(later.signal.aborted).toBe(false);expect(events).toBe(1);
 a.end();a.end();b.end();later.end();transfers.cancelUploads('A');expect(later.signal.aborted).toBe(false);
 const sending=transfers.beginUploadBatch('B');sending.end();transfers.cancelUploads('B');expect(sending.signal.aborted).toBe(false);expect(other.signal.aborted).toBe(true);
});
