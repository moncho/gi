// Shipped Classic UI + installed backend functions, joined by disposable HTTP fixtures.
// Run with PICLAW_DB_IN_MEMORY=1. Never reaches Piclaw's live HTTP server or store.
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import path from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';

if(process.env.PICLAW_DB_IN_MEMORY!=='1')throw Error('Set PICLAW_DB_IN_MEMORY=1 for this probe');
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'../../..');
const oracleRoot=path.resolve(process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current');
const browserName=process.env.ORACLE_BROWSER||'chromium';
if(!['chromium','webkit'].includes(browserName))throw Error('Unsupported browser');
const reference=JSON.parse(await readFile(path.join(root,'tests/ux/oracle/piclaw-3.2.4-reference.json'),'utf8'));
assert.equal((await readFile(path.join(oracleRoot,'VERSION'),'utf8')).trim(),'3.2.4');
const map=await readFile(path.join(oracleRoot,'app/runtime/web/static/classic/dist/app.bundle.js.map'));
assert.equal(createHash('sha256').update(map).digest('hex'),reference.map.sha256);
const state=JSON.parse(await readFile(path.join(root,'tests/ux/fixtures/compose-pixel-state.json'),'utf8'));
state.sessionId='web:default'; // The pinned Classic fixture uses this selected chat.
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
const reply=storeMessage({id:'combined-reply',chat_jid:state.sessionId,sender:'probe',sender_name:'probe',content:'combined-reply',thread_id:parent,timestamp:now});
const requests=[],dialogs=[];
try{
 await page.route('**/timeline?*',route=>{
  // A restricted current view includes the real parent row, while the reply
  // still exists in the in-memory store. This route is an explicit UI fixture.
  const result=getTimelineResponse(state.sessionId,50);
  const posts=result.body.posts.filter(post=>post.id===parent);
  assert.equal(posts.length,1);
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
 page.on('dialog',async dialog=>{dialogs.push(dialog.message());await dialog.dismiss();});
 await page.goto(host.origin);
 const post=page.getByText('combined-parent',{exact:true});
 await post.waitFor();await host.connected();
 await page.locator('.post-delete-btn').first().click();
 await post.waitFor({state:'detached'});
 const remaining=connection.getDb().prepare('SELECT rowid,thread_id FROM messages WHERE chat_jid = ? ORDER BY rowid').all(state.sessionId);
 assert.deepEqual(dialogs,[],'No confirmation when reply is outside the loaded timeline');
 assert.deepEqual(requests,[{id:parent,cascade:false}]);
 assert.deepEqual(remaining,[{rowid:reply,thread_id:parent}]);
 assert.equal(await post.count(),0);
 host.assert();
 console.log(JSON.stringify({browserName,version:'3.2.4',mapSha256:reference.map.sha256,
  scope:'shipped Classic UI and installed backend functions joined by disposable routes and in-memory SQLite; no live HTTP or store',
  requests,dialogs,parentRemovedFromUI:true,orphanReply:remaining[0]},null,2));
}finally{await host.dispose();await context.close();await browser.close();connection.closeDatabase();}
