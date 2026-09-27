// Installed Piclaw 3.2.4 search trigger against isolated in-memory SQLite.
// Hook records background requests; it never launches an index child or touches live files.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {mkdtemp,readFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import path from 'node:path';
import {pathToFileURL} from 'node:url';

if(process.env.PICLAW_DB_IN_MEMORY!=='1')throw Error('Set PICLAW_DB_IN_MEMORY=1');
const oracleRoot=path.resolve(process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current');
assert.equal((await readFile(path.join(oracleRoot,'VERSION'),'utf8')).trim(),'3.2.4');
const reference=JSON.parse(await readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const map=await readFile(path.join(oracleRoot,'app/runtime/web/static/classic/dist/app.bundle.js.map'));
assert.equal(createHash('sha256').update(map).digest('hex'),reference.map.sha256);
const workspace=await mkdtemp(path.join(tmpdir(),'piclaw-index-trigger-'));
process.env.PICLAW_WORKSPACE=workspace;
const source=(file:string)=>pathToFileURL(path.join(oracleRoot,'app/runtime/src',file)).href;
let closeDatabase:(()=>void)|undefined;
let resetHook:(()=>void)|undefined;
try{
 const connection=await import(source('db/connection.ts'));
 closeDatabase=connection.closeDatabase;
 const {searchWorkspace,setBackgroundWorkspaceIndexRefreshRequesterForTests}=await import(source('workspace-search.ts'));
 connection.initDatabase();
 const db=connection.getDb();
 assert.equal(db.prepare('PRAGMA database_list').all()[0]?.file,'','Probe must use an in-memory database');
 const requests:any[]=[];
 setBackgroundWorkspaceIndexRefreshRequesterForTests(params=>requests.push(params));
 resetHook=()=>setBackgroundWorkspaceIndexRefreshRequesterForTests(null);
 db.prepare('INSERT INTO workspace_fts(content,path,size_bytes,mtime_ms) VALUES(?,?,?,?)')
   .run('oracle indigo committed hit','notes/oracle-index.txt',27,1);
 const query={query:'indigo',scope:'notes',limit:5};
 const cold=await searchWorkspace(query);
 assert.equal(cold.error,undefined);
 assert.equal(cold.rows.length,1);
 assert.equal(cold.rows[0].path,'notes/oracle-index.txt');
 assert.deepEqual(requests,[{scope:'notes',max_kb:undefined}]);
 const blank=await searchWorkspace({...query,query:' '});
 assert.equal(blank.error,'Provide a query.');
 assert.equal(requests.length,1,'Blank search must not schedule indexing');
 const now=new Date().toISOString();
 const setState=db.prepare(`INSERT INTO workspace_index_status(scope,state,indexed_file_count,roots_json,updated_at)
  VALUES('notes',?,1,'[]',?) ON CONFLICT(scope) DO UPDATE SET state=excluded.state,updated_at=excluded.updated_at`);
 setState.run('ready',now);
 const ready=await searchWorkspace(query);
 assert.equal(ready.rows.length,1);
 assert.equal(requests.length,1,'Ready index must not request refresh');
 setState.run('stale',now);
 const stale=await searchWorkspace(query);
 assert.equal(stale.rows.length,1);
 assert.deepEqual(requests,[{scope:'notes',max_kb:undefined},{scope:'notes',max_kb:undefined}]);
 console.log(JSON.stringify({release:reference.release,mapSha256:reference.map.sha256,
  scope:'installed search function and in-memory SQLite; background requester intercepted, no child/live files',
  cold:{hits:cold.rows.length,requests:1},blank:{error:blank.error,requests:0},
  ready:{hits:ready.rows.length,requests:0},stale:{hits:stale.rows.length,requests:1},
  recordedRequests:requests},null,2));
}finally{
 resetHook?.();closeDatabase?.();
 await rm(workspace,{recursive:true,force:true});
}
