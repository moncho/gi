import {projectMessageMedia} from './gi-message-media.js';
import {projectLinkPreviews} from './gi-message-links.js';

export const SYSTEM_AGENT_ID = '__gi_system__';
export const SYSTEM_AGENT = {id:SYSTEM_AGENT_ID,name:'System',avatar_url:null};

// The wire/storage role is independent from the shared Post component's two
// visual roles. Non-user notices use bot presentation with explicit identity.
export function projectConversationMessage(m: any, fallbackSession?: string) {
    if (!['user','assistant','system'].includes(m?.role) || (m.role !== 'user' && m.payload?.kind === 'tool_result')) return null;
    let content = typeof m.content === 'string' ? m.content : '';
    if (m.role === 'assistant' && m.payload?.kind === 'tool_calls') {
        if (typeof m.payload.display_text === 'string') content = m.payload.display_text;
        else content = content.split(/(?:^|\n)\[tool_call:/, 1)[0];
        if (!content.trim()) return null;
    }
    const session = m.session_id || fallbackSession;
    const user = m.role === 'user';
    return {
        id:m.id,chat_jid:`gi:${session}`,timestamp:m.created_at,content,
        sender:user?'user':m.role==='system'?'system':'agent',
        is_from_me:user,is_bot_message:!user,
        data:{type:user?'user_message':'agent_response',content,thread_id:null,
            agent_id:m.role==='system'?SYSTEM_AGENT_ID:(m.payload?.agent_id||(user?null:'agent')),
            ...projectMessageMedia(m.payload,session),link_previews:projectLinkPreviews(m.payload),
            content_meta:null,kind:m.payload?.kind||null,source:m.payload?.source||null,clipped:m.payload?.clipped||false},
    };
}

// new_post system_message frames previously bypassed the history projection.
export function projectConversationEvent(post: any) {
    if (post?.data?.type !== 'system_message' && post?.sender !== 'system') return post;
    return {...post,is_from_me:false,is_bot_message:true,sender:'system',
        data:{...post.data,type:'agent_response',agent_id:SYSTEM_AGENT_ID}};
}

// Activity snapshots still retain full native metadata for Stop/queue controls.
// Only current running activity belongs in the transient status panel.
export function projectActivityStatus(activity: any) {
    if (!activity || !['running','cancelling'].includes(activity.status)) return null;
    if(activity.status==='cancelling')return {...activity,type:'intent',title:'Cancelling…'};
    if(activity.phase==='retry_wait')return activity;
    if(activity.tool && activity.tool.state !== 'running')return {...activity,type:'waiting',title:'Waiting for model…'};
    return activity;
}
