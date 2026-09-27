// Pinned shipped Classic UI assets with disposable timeline/DELETE fixtures; no Piclaw backend or live writes.
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'../../..');
const oracleRoot=path.resolve(process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current');
const browserName=process.env.ORACLE_BROWSER||'chromium';
if(!['chromium','webkit'].includes(browserName))throw Error('Unsupported browser');
const reference=JSON.parse(await readFile(path.join(root,'tests/ux/oracle/piclaw-3.2.4-reference.json'),'utf8'));
assert.equal((await readFile(path.join(oracleRoot,'VERSION'),'utf8')).trim(),'3.2.4');
const map=await readFile(path.join(oracleRoot,'app/runtime/web/static/classic/dist/app.bundle.js.map'));
assert.equal(createHash('sha256').update(map).digest('hex'),reference.map.sha256);
const state=JSON.parse(await readFile(path.join(root,'tests/ux/fixtures/compose-pixel-state.json'),'utf8'));
state.sessionId='web:default';
const html=await readFile(path.join(oracleRoot,'app/runtime/web/static/classic/index.html'),'utf8');
const browser=await (browserName==='chromium'?chromium:webkit).launch({headless:true});
const rows=[{id:101,chat_jid:state.sessionId,timestamp:state.now,data:{type:'user_message',content:'delete-probe-parent'}},
 ...[102,103,104].map(id=>({id,chat_jid:state.sessionId,timestamp:state.now,data:{type:'user_message',content:`delete-probe-reply-${id}`,thread_id:101}}))];

async function run({visibleReplies,confirm,backendReject}){
 const context=await browser.newContext({viewport:{width:1440,height:900},serviceWorkers:'block'});
 const page=await context.newPage();
 const host=await installPixelHost({page,host:'piclaw',root:oracleRoot,state,reference});
 let loaded=visibleReplies?[...rows]:[rows[0]];
 const requests=[],dialogs=[];
 try{
  await page.route('**/timeline?*',route=>route.fulfill({json:{posts:loaded,has_more:false}}));
  await page.route('**/post/*',route=>{
   const url=new URL(route.request().url());
   assert.equal(route.request().method(),'DELETE');
   assert.equal(url.searchParams.get('chat_jid'),state.sessionId);
   const cascade=url.searchParams.get('cascade');
   requests.push({id:Number(url.pathname.split('/').pop()),cascade});
   if(backendReject&&cascade==='false')return route.fulfill({status:409,json:{error:'Replies exist'}});
   const ids=cascade==='true'?[101,102,103,104]:[101];
   loaded=loaded.filter(post=>!ids.includes(post.id));
   return route.fulfill({json:{ids,deleted:ids.length}});
  });
  await page.route(host.origin+'/',route=>route.fulfill({contentType:'text/html',body:html.replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1')}));
  await page.route('**/agent/picker-pins',route=>route.fulfill({json:{scope:state.sessionId,revision:1,models:[],sessions:[]}}));
  await page.goto(host.origin);
  await page.locator('.post-delete-btn').first().waitFor();
  await host.connected();
  let dialogReady;const dialogGate=new Promise(resolve=>{dialogReady=resolve;});
  page.on('dialog',async dialog=>{dialogs.push(dialog.message());if(confirm)await dialog.accept();else await dialog.dismiss();dialogReady();});
  await page.locator('.post-delete-btn').first().click();
  await Promise.race([dialogGate,page.waitForTimeout(1200).then(()=>{throw Error('Expected deletion confirmation dialog');})]);
  if(confirm){
   for(let i=0;i<24&&requests.length!==(visibleReplies?1:2);i++)await page.waitForTimeout(50);
   assert.equal(requests.length,visibleReplies?1:2);
   await page.getByText('delete-probe-parent',{exact:true}).waitFor({state:'detached'});
  }else if(backendReject){
   for(let i=0;i<24&&requests.length!==1;i++)await page.waitForTimeout(50);
   assert.equal(requests.length,1);
  }
  const result={dialogs,requests,visibleParents:await page.getByText('delete-probe-parent',{exact:true}).count(),
   visibleReplies:await page.getByText('delete-probe-reply-102',{exact:true}).count()};
  host.assert();
  return result;
 }finally{await host.dispose();await context.close();}
}
try{
 const cancelledVisible=await run({visibleReplies:true,confirm:false,backendReject:false});
 assert.deepEqual(cancelledVisible.requests,[]);assert.match(cancelledVisible.dialogs[0],/Delete this message and its 3 replies\?/);
 assert.equal(cancelledVisible.visibleParents,1);assert.equal(cancelledVisible.visibleReplies,1);
 const confirmedVisible=await run({visibleReplies:true,confirm:true,backendReject:false});
 assert.deepEqual(confirmedVisible.requests,[{id:101,cascade:'true'}]);
 assert.equal(confirmedVisible.visibleParents,0);assert.equal(confirmedVisible.visibleReplies,0);
 const confirmedHidden=await run({visibleReplies:false,confirm:true,backendReject:true});
 assert.deepEqual(confirmedHidden.requests,[{id:101,cascade:'false'},{id:101,cascade:'true'}]);
 assert.match(confirmedHidden.dialogs[0],/Delete this message and its replies\?/);
 const cancelledHidden=await run({visibleReplies:false,confirm:false,backendReject:true});
 assert.deepEqual(cancelledHidden.requests,[{id:101,cascade:'false'}]);
 assert.match(cancelledHidden.dialogs[0],/Delete this message and its replies\?/);
 assert.equal(cancelledHidden.visibleParents,1);
 console.log(JSON.stringify({browserName,version:'3.2.4',mapSha256:reference.map.sha256,
  scope:'shipped Classic UI with disposable API fixtures; no Piclaw backend or live writes',
  cancelledVisible,confirmedVisible,confirmedHidden,cancelledHidden},null,2));
}finally{await browser.close();}
