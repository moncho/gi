// Shipped Classic UI + installed backend functions, joined by disposable HTTP fixtures.
// Run with PICLAW_DB_IN_MEMORY=1. Never reaches Piclaw's live HTTP server or store.
import assert from 'node:assert/strict';
import {mkdtemp,readFile,rm} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import path from 'node:path';
import {tmpdir} from 'node:os';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';

if(process.env.PICLAW_DB_IN_MEMORY!=='1')throw Error('Set PICLAW_DB_IN_MEMORY=1 for this probe');
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'../../..');
const oracleRoot=path.resolve(process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current');
const browserName=process.env.ORACLE_BROWSER||'chromium';
const caseName=process.env.ORACLE_DELETE_CASE||'hidden-reply';
if(!['chromium','webkit'].includes(browserName))throw Error('Unsupported browser');
if(!['hidden-reply','visible-confirm','visible-cancel'].includes(caseName))throw Error('Unsupported deletion case');
const reference=JSON.parse(await readFile(path.join(root,'tests/ux/oracle/piclaw-3.2.4-reference.json'),'utf8'));
assert.equal((await readFile(path.join(oracleRoot,'VERSION'),'utf8')).trim(),'3.2.4');
const map=await readFile(path.join(oracleRoot,'app/runtime/web/static/classic/dist/app.bundle.js.map'));
assert.equal(createHash('sha256').update(map).digest('hex'),reference.map.sha256);
const state=JSON.parse(await readFile(path.join(root,'tests/ux/fixtures/compose-pixel-state.json'),'utf8'));
state.sessionId='web:default'; // The pinned Classic fixture uses this selected chat.
const workspace=await mkdtemp(path.join(tmpdir(),'piclaw-delete-combined-'));
process.env.PICLAW_WORKSPACE=workspace;
const source=file=>pathToFileURL(path.join(oracleRoot,'app/runtime/src',file)).href;
const connection=await import(source('db/connection.ts'));
const {storeChatMetadata,storeMessage}=await import(source('db/messages.ts'));
const {getTimelineResponse,deletePostResponse}=await import(source('channels/web/timeline-service.ts'));
connection.initDatabase();
assert.equal(connection.getDb().prepare('PRAGMA database_list').all()[0]?.file,'');
const browser=await (browserName==='chromium'?chromium:webkit).launch({headless:true});
const context=await browser.newContext({viewport:{width:1440,height:900},serviceWorkers:'block'});
const page=await context.newPage();
const host=await installPixelHost({page,host:'piclaw',root:oracleRoot,state,reference});
const now=new Date().toISOString();
storeChatMetadata(state.sessionId,now);
const parent=storeMessage({id:'combined-parent',chat_jid:state.sessionId,sender:'probe',sender_name:'probe',content:'combined-parent',timestamp:now});
const replyIds=Array.from({length:caseName==='hidden-reply'?1:3},(_,i)=>storeMessage({
 id:`combined-reply-${i+1}`,chat_jid:state.sessionId,sender:'probe',sender_name:'probe',
 content:`combined-reply-${i+1}`,thread_id:parent,timestamp:now,
}));
const requests=[],dialogs=[];
try{
 await page.route('**/timeline?*',route=>{
  const result=getTimelineResponse(state.sessionId,50);
  // Hidden case deliberately limits the current view; visible cases use all
  // real parent/reply rows returned by the installed backend function.
  const posts=caseName==='hidden-reply'?result.body.posts.filter(post=>post.id===parent):result.body.posts;
  assert.equal(posts.length,caseName==='hidden-reply'?1:4);
  return route.fulfill({status:result.status,json:{...result.body,posts}});
 });
 await page.route('**/post/*',route=>{
  const url=new URL(route.request().url());
  assert.equal(route.request().method(),'DELETE');
  assert.equal(url.searchParams.get('chat_jid'),state.sessionId);
  const cascade=url.searchParams.get('cascade')==='true';
  const id=Number(url.pathname.split('/').pop());
  requests.push({id,cascade});
  const result=deletePostResponse(state.sessionId,id,cascade);
  return route.fulfill({status:result.status,json:result.body});
 });
 const html=await readFile(path.join(oracleRoot,'app/runtime/web/static/classic/index.html'),'utf8');
 await page.route(host.origin+'/',route=>route.fulfill({contentType:'text/html',body:html.replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1')}));
 await page.route('**/agent/picker-pins',route=>route.fulfill({json:{scope:state.sessionId,revision:1,models:[],sessions:[]}}));
 page.on('dialog',async dialog=>{
  dialogs.push(dialog.message());
  if(caseName==='visible-confirm')await dialog.accept();else await dialog.dismiss();
 });
 await page.goto(host.origin);
 const post=page.getByText('combined-parent',{exact:true});
 await post.waitFor();await host.connected();
 await page.locator('.post-delete-btn').first().click();
 if(caseName==='visible-cancel'){
  assert.deepEqual(dialogs,['Delete this message and its 3 replies?']);
  assert.deepEqual(requests,[]);
  assert.equal(await post.count(),1);
 }else await post.waitFor({state:'detached'});
 const remaining=connection.getDb().prepare('SELECT rowid,thread_id FROM messages WHERE chat_jid = ? ORDER BY rowid').all(state.sessionId);
 if(caseName==='hidden-reply'){
  assert.deepEqual(dialogs,[],'No confirmation when reply is outside the loaded timeline');
  assert.deepEqual(requests,[{id:parent,cascade:false}]);
  assert.deepEqual(remaining,[{rowid:replyIds[0],thread_id:parent}]);
 }else if(caseName==='visible-confirm'){
  assert.deepEqual(dialogs,['Delete this message and its 3 replies?']);
  assert.deepEqual(requests,[{id:parent,cascade:true}]);
  assert.deepEqual(remaining,[]);
  for(let i=1;i<=3;i++)assert.equal(await page.getByText(`combined-reply-${i}`,{exact:true}).count(),0);
 }else{
  assert.deepEqual(remaining,[{rowid:parent,thread_id:null},...replyIds.map(rowid=>({rowid,thread_id:parent}))]);
  for(let i=1;i<=3;i++)assert.equal(await page.getByText(`combined-reply-${i}`,{exact:true}).count(),1);
 }
 host.assert();
 console.log(JSON.stringify({browserName,caseName,version:'3.2.4',mapSha256:reference.map.sha256,
  scope:'shipped Classic UI and installed backend functions joined by disposable routes and in-memory SQLite; no live HTTP or store',
  requests,dialogs,parentVisible:await post.count()===1,remaining},null,2));
}finally{await host.dispose();await context.close();await browser.close();connection.closeDatabase();await rm(workspace,{recursive:true,force:true});}
