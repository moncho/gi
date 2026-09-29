import {test,expect} from 'bun:test';
import {projectConversationMessage,projectConversationEvent,projectActivityStatus,SYSTEM_AGENT_ID} from '../../../web/src/gi-conversation.ts';
test('raw internal roles never fall through to user presentation',()=>{
 for(const role of ['tool_result','unknown',null])expect(projectConversationMessage({role,content:'raw'},'s')).toBeNull();
 const system=projectConversationMessage({id:'s',role:'system',content:'Inference error: fixture'},'session');
 expect(system?.sender).toBe('system');expect(system?.is_from_me).toBe(false);expect(system?.data.type).toBe('agent_response');expect(system?.data.agent_id).toBe(SYSTEM_AGENT_ID);
 const live=projectConversationEvent({id:'s',sender:'system',data:{type:'system_message',content:'notice'}});expect(live.data.agent_id).toBe(SYSTEM_AGENT_ID);expect(live.is_from_me).toBe(false);
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
