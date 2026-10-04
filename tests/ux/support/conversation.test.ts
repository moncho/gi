import {test,expect} from 'bun:test';
import {projectConversationMessage,projectConversationEvent,projectActivityStatus,projectResponsePhase,SYSTEM_AGENT_ID} from '../../../web/src/gi-conversation.ts';
test('raw internal roles never fall through to user presentation',()=>{
 for(const role of ['tool_result','unknown',null])expect(projectConversationMessage({role,content:'raw'},'s')).toBeNull();
 const system=projectConversationMessage({id:'s',role:'system',content:'Inference error: fixture'},'session');
 expect(system?.sender).toBe('system');expect(system?.is_from_me).toBe(false);expect(system?.data.type).toBe('agent_response');expect(system?.data.agent_id).toBe(SYSTEM_AGENT_ID);
 const live=projectConversationEvent({id:'s',sender:'system',data:{type:'system_message',content:'notice'}});expect(live.data.agent_id).toBe(SYSTEM_AGENT_ID);expect(live.is_from_me).toBe(false);
});
test('queued prompt notices stay in raw history but not the conversation',()=>{
 const queued={id:'q',role:'system',content:'Queued prompt: unique follow-up',payload:{kind:'queue',turn_id:'t'}};
 expect(projectConversationMessage(queued,'s')).toBeNull();
 expect(projectConversationEvent({id:'q',sender:'system',data:{type:'system_message',kind:'queue',content:queued.content}})).toBeNull();
 expect(projectConversationMessage({...queued,role:'user'},'s')?.content).toBe(queued.content);
});
test('only known tool-call summaries strip synthetic suffixes; prose/media survive',()=>{
 const m={id:'m',role:'assistant',content:'**Inspecting.**\n[tool_call: shell]',payload:{kind:'tool_calls',media:[{media_id:1,session_id:'s',filename:'file.txt',mime_type:'text/plain'}]}};
 const p=projectConversationMessage(m,'s');expect(p?.data.content).toBe('**Inspecting.**');expect(p?.data.media_ids).toEqual([1]);expect(m.content).toContain('[tool_call:');
 expect(projectConversationMessage({...m,content:'[tool_call: shell]'},'s')).toBeNull();
 expect(projectConversationMessage({...m,payload:{}},'s')?.data.content).toBe(m.content);
 expect(projectConversationMessage({...m,payload:{kind:'tool_calls',display_text:'literal [tool_call: quote]'}},'s')?.data.content).toBe('literal [tool_call: quote]');
});
test('idle activity metadata never creates a working/completed panel',()=>{
 expect(projectActivityStatus({status:'idle',tool:{state:'completed'}})).toBeNull();
 expect(projectActivityStatus({status:'running',tool:{state:'completed'}})?.title).toBe('Waiting for model...');
 expect(projectActivityStatus({status:'running',tool:{state:'failed'}})?.title).toBe('Reviewing failed tool result...');
 const retry={status:'running',phase:'retry_wait',title:'Retrying'};expect(projectActivityStatus(retry)).toBe(retry);
 expect(projectActivityStatus({status:'cancelling'})?.title).toBe('Cancelling…');
});
test('an active response draft names its phase without overwriting tool, retry or compaction',()=>{
 const running={status:'running',title:'Working…'};
 expect(projectResponsePhase(running,{text:'Partial reply'}).title).toBe('Writing response');
 expect(projectResponsePhase(running,{text:'   '})).toBe(running);
 for(const value of [{...running,tool:{state:'running'}},{...running,tool_name:'shell'},{...running,phase:'retry_wait'},{...running,phase:'compacting'}]){
  expect(projectResponsePhase(value,{text:'Partial reply'})).toBe(value);
 }
 expect(projectResponsePhase({status:'idle'},{text:'Partial reply'}).title).toBeUndefined();
});
test('user-authored tool syntax and metadata never hide or replace their words',()=>{
 for(const kind of ['tool_calls','tool_result'])expect(projectConversationMessage({role:'user',content:'literal [tool_call: example]',payload:{kind,display_text:'wrong'}},'s')?.content).toBe('literal [tool_call: example]');
});
test('running tool projects output and command independently; retry/wait never reuse output',()=>{
 const tool={state:'running',name:'shell',preview:'printf output',started_at:'2026-09-28T00:00:00Z',output_preview:'actual\noutput',output_total_lines:2};
 const active=projectActivityStatus({status:'running',turn_id:'t',tool});
 expect(active.type).toBe('tool_status');expect(active.output_preview).toBe('actual\noutput');expect(active.tool_args.command).toBe('printf output');
 expect(projectActivityStatus({status:'running',turn_id:'t',tool:{...tool,state:'completed'}}).output_preview).toBeUndefined();
 expect(projectActivityStatus({status:'idle',tool})).toBeNull();
 expect(projectActivityStatus({status:'running',phase:'retry_wait',title:'Retrying',tool}).output_preview).toBeUndefined();
});

 test('assistant reply identity survives conversation projection',()=>{
 const reply=projectConversationMessage({id:'reply',role:'assistant',session_id:'s',content:'answer',reply_to_id:'prompt',payload:{turn_id:'turn'}});
 expect(reply.data.thread_id).toBe('prompt');
 });
