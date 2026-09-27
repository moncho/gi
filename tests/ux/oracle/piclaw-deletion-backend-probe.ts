// Installed Piclaw 3.2.4 backend probe. Memory-only: no HTTP server or live chat writes.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {mkdtemp, readFile, rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import path from 'node:path';
import {pathToFileURL} from 'node:url';

if (process.env.PICLAW_DB_IN_MEMORY !== '1') throw new Error('Set PICLAW_DB_IN_MEMORY=1 for this probe');
const oracleRoot=path.resolve(process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current');
const version=(await readFile(path.join(oracleRoot,'VERSION'),'utf8')).trim();
assert.equal(version,'3.2.4','Pinned Piclaw version changed');
const reference=JSON.parse(await readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const map=await readFile(path.join(oracleRoot,'app/runtime/web/static/classic/dist/app.bundle.js.map'));
assert.equal(createHash('sha256').update(map).digest('hex'),reference.map.sha256,'Shipped Classic map changed');
const workspace=await mkdtemp(path.join(tmpdir(),'piclaw-delete-probe-'));
process.env.PICLAW_WORKSPACE=workspace;
const source=(file: string)=>pathToFileURL(path.join(oracleRoot,'app/runtime/src',file)).href;
let closeDatabase: (()=>void)|undefined;
try {
  const connection=await import(source('db/connection.ts'));
  closeDatabase=connection.closeDatabase;
  const {storeChatMetadata,storeMessage}=await import(source('db/messages.ts'));
  const {deletePostResponse}=await import(source('channels/web/timeline-service.ts'));
  connection.initDatabase();
  const database=connection.getDb();
  assert.equal(database.prepare('PRAGMA database_list').all()[0]?.file,'','Probe must use an in-memory database');
  const chat='web:disposable-delete-probe';
  const now=new Date().toISOString();
  storeChatMetadata(chat,now);
  const insert=(id: string,thread_id?: number)=>storeMessage({
    id,chat_jid:chat,sender:'probe',sender_name:'probe',content:id,timestamp:now,thread_id,
  });
  const directParent=insert('direct-parent');
  const orphan=insert('direct-reply',directParent);
  const direct=deletePostResponse(chat,directParent,false);
  assert.equal(direct.status,200);
  assert.deepEqual(direct.deletedIds,[directParent]);
  const rows=()=>database.prepare('select rowid,thread_id from messages where chat_jid = ? order by rowid').all(chat);
  assert.deepEqual(rows(),[{rowid:orphan,thread_id:directParent}],
    'Direct deletion leaves a reply to a now-missing parent');
  const cascadeParent=insert('cascade-parent');
  const replies=['one','two','three'].map(label=>insert(`cascade-${label}`,cascadeParent));
  const cascade=deletePostResponse(chat,cascadeParent,true);
  assert.equal(cascade.status,200);
  assert.deepEqual(new Set(cascade.deletedIds),new Set([cascadeParent,...replies]));
  assert.deepEqual(rows(),[{rowid:orphan,thread_id:directParent}],
    'Cascade deletes the parent and three direct replies without touching another row');
  console.log(JSON.stringify({version,mapSha256:reference.map.sha256,
    scope:'installed Piclaw backend functions with isolated in-memory SQLite; no HTTP/UI/live writes',
    direct:{status:direct.status,deletedIds:direct.deletedIds,orphanReplyId:orphan},
    cascade:{status:cascade.status,deletedIds:cascade.deletedIds,remainingIds:rows().map((row: any)=>row.rowid)}},null,2));
} finally {
  closeDatabase?.();
  await rm(workspace,{recursive:true,force:true});
}
