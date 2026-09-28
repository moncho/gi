// Installed Piclaw 3.2.4 messages-tool probe. All rows, ownership and login state
// live in a disposable in-memory DB; this never calls the live HTTP server/store.
import assert from 'node:assert/strict';
import {mkdtemp, mkdir, writeFile, rm, readFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import path from 'node:path';
import {pathToFileURL} from 'node:url';

assert.equal(process.env.PICLAW_DB_IN_MEMORY, '1', 'set PICLAW_DB_IN_MEMORY=1');
const oracleRoot=process.env.PICLAW_ORACLE_ROOT || '/opt/piclaw/current';
assert.equal((await readFile(path.join(oracleRoot,'VERSION'),'utf8')).trim(), '3.2.4');
const workspace=await mkdtemp(path.join(tmpdir(),'piclaw-get-oracle-'));
process.env.PICLAW_WORKSPACE=workspace;
const source=file=>pathToFileURL(path.join(oracleRoot,'app/runtime/src',file)).href;
const {initDatabase,getDb,closeDatabase}=await import(source('db/connection.ts'));
const {storeChatMetadata,storeMessage}=await import(source('db/messages.ts'));
const {createUser}=await import(source('db/users.ts'));
const {assignRootOwner,provisionUserHome}=await import(source('db/session-ownership.ts'));
const {ensureChatBranch}=await import(source('db/chat-branches.ts'));
const {migrateOwnedSessionHandles}=await import(source('db/session-handles.ts'));
const {authoriseExecutionIdentity}=await import(source('agent-pool/execution-identity.ts'));
const {withExecutionIdentity}=await import(source('core/execution-context.ts'));
const {withChatContext}=await import(source('core/chat-context.ts'));
const {runMessagesTool}=await import(source('extensions/messages-crud.ts'));
try {
  initDatabase();
  const db=getDb();
  assert.equal(db.query('PRAGMA database_list').all().find(row=>row.name==='main').file, '');
  const now=new Date().toISOString();
  const owner=createUser(db,{username:'owner',displayName:'Owner'});
  const stranger=createUser(db,{username:'stranger',displayName:'Stranger'});
  for(const id of [owner.id,stranger.id]) db.query('UPDATE users SET enabled=1 WHERE id=?').run(id);
  const chats=['web:owner-home','web:owner-child','web:owner-other','web:stranger'];
  for(const jid of chats) storeChatMetadata(jid,now);
  const root=ensureChatBranch({chat_jid:chats[0]});
  ensureChatBranch({chat_jid:chats[1],root_chat_jid:chats[0],parent_branch_id:root.branch_id});
  provisionUserHome(db,owner.id,chats[0]);
  assignRootOwner(db,chats[2],owner.id);
  provisionUserHome(db,stranger.id,chats[3]);
  migrateOwnedSessionHandles(db);
  const insert=(jid,n,content=`${jid} message ${n}`)=>storeMessage({id:`${jid}:${n}`,chat_jid:jid,sender:'human',sender_name:'Human',content,timestamp:new Date(Date.now()+n*1000).toISOString()});
  const home1=insert(chats[0],1),child1=insert(chats[1],1),home2=insert(chats[0],2,'first\nsecond\nthird'),other1=insert(chats[2],1),foreign=insert(chats[3],1),home3=insert(chats[0],3);
  const get=(params,defaultChat=chats[0])=>runMessagesTool({action:'get',...params},defaultChat).details;
  const diff=(params)=>runMessagesTool({action:'diff',...params},chats[0]).details;
  const ids=details=>details.messages.map(item=>item.message.rowid);
  const single=get({row_ids:[foreign,home2,home2,-1,0,999999],context_before:1,context_after:1,content_lines:'2-3'});
  assert.deepEqual(ids(single),[foreign,home2]);
  assert.deepEqual(single.missing_row_ids,[999999]);
  assert.deepEqual(single.messages[1].context_before.map(row=>row.rowid),[home1]);
  assert.deepEqual(single.messages[1].context_after.map(row=>row.rowid),[home3]);
  assert.deepEqual(single.messages[1].line_view.lines.map(line=>line.content),['second','third']);
  const singleExplicit=get({row_ids:[foreign,home2],chat_jid:chats[0]});
  assert.deepEqual(ids(singleExplicit),[home2]);
  assert.deepEqual(singleExplicit.missing_row_ids,[foreign]);
  assert.equal(get({row_ids:[home2],content_lines:'bad'}).error,'invalid_content_lines');
  const capped=get({row_ids:[home2],context_before:100,context_after:100});
  assert.equal(capped.context_before,20);
  assert.equal(capped.context_after,20);
  const singleWindow=diff({before_row:home3,chat_jid:'all',limit:2});
  assert.deepEqual(singleWindow.messages.map(row=>row.rowid),[other1,foreign]);

  await mkdir(path.join(workspace,'.piclaw'),{recursive:true});
  await writeFile(path.join(workspace,'.piclaw','config.json'),JSON.stringify({domains:{access:{mode:'family-shared'}}}));
  const login='fixture-login';
  db.query('INSERT INTO web_sessions(token,user_id,auth_method,created_at,expires_at,session_id) VALUES (?,?,?,?,?,?)')
    .run('fixture-token',owner.id,'passkey',now,new Date(Date.now()+3600000).toISOString(),login);
  const provenance={actorUserId:owner.id,ownerUserId:owner.id,chatJid:chats[0],kind:'interactive',authenticationSessionId:login};
  const identity=authoriseExecutionIdentity(db,'family-shared',chats[0],provenance);
  assert.equal(identity.toolPolicy.allowed.includes('messages'),true);
  const underIdentity=fn=>withChatContext(chats[0],'web',()=>withExecutionIdentity(identity,fn));
  const denied=details=>assert.equal(details.error,'access_denied');
  denied(get({row_ids:[home2]})); // no trusted execution context
  const owned=await underIdentity(()=>get({row_ids:[foreign,child1,other1,home2],context_before:1,context_after:1}));
  assert.ok(owned.messages,JSON.stringify(owned));
  assert.deepEqual(ids(owned),[child1,other1,home2]);
  assert.deepEqual(owned.missing_row_ids,[foreign]);
  assert.deepEqual(owned.messages.find(item=>item.message.rowid===home2).context_before.map(row=>row.rowid),[home1]);
  const all=await underIdentity(()=>get({row_ids:[foreign,child1,other1,home2],chat_jid:'all'}));
  assert.deepEqual(ids(all),[child1,other1,home2]);
  const explicit=await underIdentity(()=>get({row_ids:[foreign,other1],chat_jid:chats[2]}));
  assert.deepEqual(ids(explicit),[other1]);
  assert.deepEqual(explicit.missing_row_ids,[foreign]);
  denied(await underIdentity(()=>get({row_ids:[foreign],chat_jid:chats[3]})));
  denied(await underIdentity(()=>get({row_ids:[foreign],chat_jid:chats[3]},chats[3])));
  const noForeignContext=await underIdentity(()=>get({row_ids:[foreign],context_before:20,context_after:20}));
  assert.deepEqual(noForeignContext.missing_row_ids,[foreign]);
  assert.deepEqual(noForeignContext.messages,[]);
  const defaultWindow=await underIdentity(()=>diff({after_row:home1,limit:20}));
  assert.deepEqual(defaultWindow.messages.map(row=>row.rowid),[home2,home3]);
  const ownedWindow=await underIdentity(()=>diff({after_row:home1,chat_jid:'all',limit:20}));
  assert.deepEqual(ownedWindow.messages.map(row=>row.rowid),[child1,home2,other1,home3]);
  const limitedWindow=await underIdentity(()=>diff({before_row:home3,chat_jid:'all',limit:2}));
  assert.deepEqual(limitedWindow.messages.map(row=>row.rowid),[home2,other1]);
  denied(await underIdentity(()=>diff({after_row:home1,chat_jid:chats[3]})));
  db.query('UPDATE web_sessions SET expires_at=? WHERE session_id=?').run(new Date(Date.now()-1000).toISOString(),login);
  denied(await underIdentity(()=>get({row_ids:[home2]}))); // cached run identity cannot survive login expiry
  console.log(JSON.stringify({oracle:'Piclaw 3.2.4 installed runtime; in-memory only',single_user:{unqualified_row_ids:ids(single),specific_chat_ids:ids(singleExplicit),context_before:1,context_after:1,clamp:capped.context_after,limited_window_ids:singleWindow.messages.map(row=>row.rowid)},family_shared:{owned_ids:ids(owned),missing_ids:owned.missing_row_ids,all_ids:ids(all),explicit_ids:ids(explicit),default_window_ids:defaultWindow.messages.map(row=>row.rowid),owned_window_ids:ownedWindow.messages.map(row=>row.rowid),limited_window_ids:limitedWindow.messages.map(row=>row.rowid),unowned_explicit:'denied',missing_identity:'denied',expired_login:'denied'},assertions:'passed'},null,2));
} finally {
  closeDatabase();
  await rm(workspace,{recursive:true,force:true});
}
