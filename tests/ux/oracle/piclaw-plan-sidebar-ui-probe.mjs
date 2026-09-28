// Installed Piclaw Plan Sidebar 0.1.25, shipped Classic assets and disposable API fixtures.
// No production Plan API, chat writes, Gi Plan implementation or physical acceptance.
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'../../..');
const oracleRoot=path.resolve(process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current');
const addonRoot=path.resolve(process.env.PICLAW_PLAN_ADDON_ROOT||'/workspace/.pi/extensions/node_modules/@rcarmo/piclaw-addon-plan-sidebar');
const browserName=process.env.ORACLE_BROWSER||'chromium';
if(!['chromium','webkit'].includes(browserName))throw Error('Unsupported browser');
const reference=JSON.parse(await readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
assert.equal((await readFile(path.join(oracleRoot,'VERSION'),'utf8')).trim(),'3.2.4');
assert.equal(createHash('sha256').update(await readFile(path.join(oracleRoot,'app/runtime/web/static/classic/dist/app.bundle.js.map'))).digest('hex'),reference.map.sha256);
const packageJson=JSON.parse(await readFile(path.join(addonRoot,'package.json'),'utf8'));
assert.equal(packageJson.name,'@rcarmo/piclaw-addon-plan-sidebar');
assert.equal(packageJson.version,'0.1.25');
const addonBytes=await readFile(path.join(addonRoot,'web/index.ts'));
const state=JSON.parse(await readFile(path.join(root,'tests/ux/fixtures/compose-pixel-state.json'),'utf8'));
state.sessionId='web:default';
const browser=await (browserName==='chromium'?chromium:webkit).launch({headless:true});
const page=await browser.newPage({viewport:{width:1440,height:900},serviceWorkers:'block'});
const host=await installPixelHost({page,host:'piclaw',root:oracleRoot,state,reference});
const sourceHtml=await readFile(path.join(oracleRoot,'app/runtime/web/static/classic/index.html'),'utf8');
const stored={markdown:'- [ ] stored original',updated_at:'2026-09-27T00:00:00.000Z'};
const calls=[],messages=[];
let rejectSave=false,holdSave=false,releaseSave=()=>{},saveHeld=()=>{};
try{
 await page.route(host.origin+'/',route=>route.fulfill({contentType:'text/html',body:sourceHtml.replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1')}));
 await page.route('**/agent/picker-pins',route=>route.fulfill({json:{scope:state.sessionId,revision:1,models:[],sessions:[]}}));
 await page.route('**/__oracle_plan_addon.js',route=>route.fulfill({contentType:'text/javascript',body:addonBytes}));
 await page.route('**/agent/addons/api/plan-sidebar/plan?*',async route=>{
  const r=route.request(),url=new URL(r.url());
  assert.equal(url.searchParams.get('chat_jid'),state.sessionId);
  calls.push({method:r.method(),chatJid:url.searchParams.get('chat_jid'),body:r.postDataJSON?.()??null});
  if(r.method()==='GET')return route.fulfill({json:{ok:true,...stored}});
  if(r.method()==='POST'){
   const body=r.postDataJSON();assert.equal(body.chat_jid,state.sessionId);
   if(holdSave){saveHeld();await new Promise(resolve=>releaseSave=resolve);holdSave=false;}
   if(rejectSave)return route.fulfill({status:503,json:{error:'fixture save refused'}});
   stored.markdown=body.markdown;stored.updated_at='2026-09-27T00:01:00.000Z';
   return route.fulfill({json:{ok:true,plan:{...stored}}});
  }
  throw Error(`Unexpected Plan request ${r.method()}`);
 });
 await page.route('**/agent/default/message?*',route=>{
  const r=route.request(),url=new URL(r.url());assert.equal(r.method(),'POST');assert.equal(url.searchParams.get('chat_jid'),state.sessionId);
  messages.push(r.postDataJSON());return route.fulfill({json:{ok:true,turn_id:'fixture-plan-turn'}});
 });
 await page.goto(host.origin);await page.locator('.compose-box textarea').waitFor();await host.connected();
 await page.addScriptTag({type:'module',url:host.origin+'/__oracle_plan_addon.js'});
 const toggle=page.getByRole('button',{name:/Show plan/});await toggle.waitFor();await toggle.click();
 const editor=page.locator('.plan-sidebar-editor .cm-content');await editor.waitFor();
 await page.waitForFunction(()=>document.querySelector('.plan-sidebar-editor .cm-content')?.textContent?.includes('stored original'));
 await editor.fill('- [ ] unsaved local');
 await page.getByText('web:default • unsaved',{exact:true}).waitFor();
 stored.markdown='- [ ] remote version';stored.updated_at='2026-09-27T00:02:00.000Z';
 await page.evaluate(()=>window.dispatchEvent(new CustomEvent('piclaw-extension-ui:status',{detail:{payload:{key:'plan.changes',chat_jid:'web:default',source:'tool'}}})));
 await page.getByText('Plan changed remotely; save or refresh to update.',{exact:true}).waitFor();
 assert.equal(await editor.textContent(),'- [ ] unsaved local');
 const beforeRefresh=calls.length;
 await page.getByRole('button',{name:'Refresh',exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('.plan-sidebar-editor .cm-content')?.textContent==='- [ ] remote version');
 assert.equal(await editor.textContent(),'- [ ] remote version');
 assert.equal(calls.slice(beforeRefresh).filter(c=>c.method==='GET').length,1);
 assert.equal(calls.filter(c=>c.method==='POST').length,0);
 const save=page.getByRole('button',{name:'Save',exact:true}),submit=page.getByRole('button',{name:'Submit to model',exact:true});
 // Hold the first Save response and edit again: the newer draft must stay dirty.
 await editor.fill('- [ ] saved snapshot');
 let signal;const held=new Promise(resolve=>signal=resolve);saveHeld=signal;holdSave=true;
 await save.click();await held;
 await editor.fill('- [ ] newer unsaved edit');releaseSave();
 await page.getByText('Saved previous edits; newer changes are unsaved.',{exact:true}).waitFor();
 assert.equal(await editor.textContent(),'- [ ] newer unsaved edit');assert.equal(stored.markdown,'- [ ] saved snapshot');
 assert.deepEqual(calls.at(-1).body,{chat_jid:state.sessionId,markdown:'- [ ] saved snapshot'});
 assert.equal(messages.length,0);
 // Save failure prevents Submit from sending anything to the model.
 rejectSave=true;await submit.click();
 await page.getByText('fixture save refused',{exact:false}).waitFor();
 assert.equal(messages.length,0);assert.equal(await editor.textContent(),'- [ ] newer unsaved edit');
 rejectSave=false;await submit.click();
 await page.getByText('Submitted to model.',{exact:true}).waitFor();
 assert.equal(stored.markdown,'- [ ] newer unsaved edit');assert.equal(messages.length,1);
 assert.equal(messages[0].mode,'auto');assert(messages[0].content.includes('- [ ] newer unsaved edit'));
 const methods=calls.filter(c=>c.method==='POST').map(c=>c.body.markdown);
 assert.deepEqual(methods,['- [ ] saved snapshot','- [ ] newer unsaved edit','- [ ] newer unsaved edit']);
 await editor.fill('   ');await submit.click();
 await page.getByText('Plan is empty; nothing to submit.',{exact:true}).waitFor();
 assert.equal(messages.length,1);
 host.assert();
 console.log(JSON.stringify({browserName,release:reference.release,addon:packageJson.name,addonVersion:packageJson.version,
  addonSha256:createHash('sha256').update(addonBytes).digest('hex'),mapSha256:reference.map.sha256,
  scope:'installed addon web source on shipped Classic assets with disposable Plan API; no production backend/live writes',
  remoteWarning:true,dirtyTextBeforeRefresh:'- [ ] unsaved local',textAfterRefresh:'- [ ] remote version',
  refreshReads:calls.slice(beforeRefresh).filter(c=>c.method==='GET'),savePosts:calls.filter(c=>c.method==='POST'),
  messageCount:messages.length,emptyPlanBlocked:true},null,2));
}finally{releaseSave();await host.dispose();await browser.close();}
