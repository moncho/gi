import {projectMessageMedia} from './gi-message-media.js';
import {projectLinkPreviews} from './gi-message-links.js';

export const SYSTEM_AGENT_ID = '__gi_system__';
export const SYSTEM_AGENT = {id:SYSTEM_AGENT_ID,name:'System',avatar_url:null};

// The wire/storage role is independent from the shared Post component's two
// visual roles. Non-user notices use bot presentation with explicit identity.
export function projectConversationMessage(m: any, fallbackSession?: string) {
    if (!['user','assistant','system'].includes(m?.role) || (m.role !== 'user' && m.payload?.kind === 'tool_result')
        || (m.role === 'system' && m.payload?.kind === 'queue')) return null;
    let content = typeof m.content === 'string' ? m.content : '';
    if (m.role === 'assistant' && m.payload?.kind === 'tool_calls') {
        if (typeof m.payload.display_text === 'string') content = m.payload.display_text;
        else content = content.split(/(?:^|\n)\[tool_call:/, 1)[0];
        if (!content.trim()) return null;
    }
    if (m.role === 'assistant' && m.payload?.kind === 'compaction') content = compactionReport(content, m.payload);
    const session = m.session_id || fallbackSession;
    const user = m.role === 'user';
    return {
        id:m.id,display_row_id:m.display_row_id,chat_jid:`gi:${session}`,timestamp:m.created_at,content,
        sender:user?'user':m.role==='system'?'system':'agent',
        is_from_me:user,is_bot_message:!user,
        data:{type:user?'user_message':'agent_response',content,thread_id:null,
            agent_id:m.role==='system'?SYSTEM_AGENT_ID:(m.payload?.agent_id||(user?null:'agent')),
            ...projectMessageMedia(m.payload,session),link_previews:projectLinkPreviews(m.payload),
            content_meta:null,kind:m.payload?.kind||null,source:m.payload?.source||null,clipped:m.payload?.clipped||false},
    };
}

// Piclaw's /compact reply: "Compaction complete." and the tokens before it.
// Gi stores the summary itself (it is the context checkpoint), so show it below.
function compactNumber(value: number): string {
    const format = (divisor: number, suffix: string) => { const raw = (value / divisor).toFixed(1); return `${raw.endsWith('.0') ? raw.slice(0, -2) : raw}${suffix}`; };
    const abs = Math.abs(value);
    if (abs >= 1_000_000_000) return format(1_000_000_000, 'B');
    if (abs >= 1_000_000) return format(1_000_000, 'M');
    if (abs >= 1_000) return format(1_000, 'K');
    return String(value);
}
export function compactionReport(summary: string, payload: any): string {
    const tokens = Number(payload?.tokens_before);
    const lines = ['Compaction complete.'];
    if (Number.isFinite(tokens) && tokens > 0) lines.push(`Tokens before: ${compactNumber(tokens)}`);
    return summary.trim() ? `${lines.join('\n')}\n\n${summary}` : lines.join('\n');
}

// new_post system_message frames previously bypassed the history projection.
export function projectConversationEvent(post: any) {
    if (post?.data?.kind === 'queue' && (post?.sender === 'system' || post?.data?.type === 'system_message')) return null;
    if (post?.data?.type !== 'system_message' && post?.sender !== 'system') return post;
    return {...post,is_from_me:false,is_bot_message:true,sender:'system',
        data:{...post.data,type:'agent_response',agent_id:SYSTEM_AGENT_ID}};
}

// Activity snapshots still retain full native metadata for Stop/queue controls.
// A response draft has its own visible phase; never overwrite tool, retry or
// compaction activity with this label.
export function projectResponsePhase(status: any, draft: any) {
    const text = typeof draft === 'string' ? draft : draft?.fullText || draft?.text;
    if (!text?.trim() || status?.status !== 'running' || status?.tool || status?.tool_name
        || status?.phase === 'retry_wait' || status?.phase === 'compacting') return status;
    return {...status, type: 'response', title: 'Writing response'};
}

// Only current running activity belongs in the transient status panel.
export function projectActivityStatus(activity: any) {
    if (!activity || !['running','cancelling'].includes(activity.status)) return null;
    if(activity.status==='cancelling')return {...activity,type:'intent',title:'Cancelling…'};
    if(activity.phase==='retry_wait')return activity;
    if(activity.tool && activity.tool.state !== 'running')return {status:'running',turn_id:activity.turn_id,type:'waiting',
        title:activity.tool.state==='failed'?'Reviewing failed tool result...':'Waiting for model...'};
    if(activity.tool?.state === 'running') {
        const tool=activity.tool;
        return {status:'running',turn_id:activity.turn_id,type:tool.output_preview?'tool_status':'tool_call',
            title:`${tool.name}${tool.preview?`: ${tool.preview}`:''}`,tool_name:tool.name,
            tool_args:tool.preview?{command:tool.preview}:{},tool_status:tool.output_preview?'Streaming output...':'Running',
            active_tool_count:1,started_at:tool.started_at,last_event_at:tool.started_at,
            output_preview:tool.output_preview,output_total_lines:tool.output_total_lines};
    }
    return activity;
}
