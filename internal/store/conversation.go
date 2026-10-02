package store

// These SQL expressions apply only to the opt-in conversation view. Raw message
// storage/export and model history keep their original content and roles.
const conversationContentSQL = `case when role='assistant' and json_extract(payload_json,'$.kind')='tool_calls' then
 case when json_type(payload_json,'$.display_text')='text' then json_extract(payload_json,'$.display_text')
 when substr(content,1,11)='[tool_call:' then ''
 when instr(content,char(10)||'[tool_call:')>0 then substr(content,1,instr(content,char(10)||'[tool_call:')-1)
 else content end else content end`

const conversationVisibleSQL = `role in ('user','assistant','system')
 and (role='user' or coalesce(json_extract(payload_json,'$.kind'),'') != 'tool_result')
 and not (role='system' and json_extract(payload_json,'$.kind') is 'queue')
 and not (role='assistant' and json_extract(payload_json,'$.kind') is 'tool_calls'
 and trim(` + conversationContentSQL + `)='')`
