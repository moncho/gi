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
const calls=[];
try{
 await page.route(host.origin+'/',route=>route.fulfill({contentType:'text/html',body:sourceHtml.replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1')}));
 await page.route('**/agent/picker-pins',route=>route.fulfill({json:{scope:state.sessionId,revision:1,models:[],sessions:[]}}));
 await page.route('**/__oracle_plan_addon.js',route=>route.fulfill({contentType:'text/javascript',body:addonBytes}));
 await page.route('**/agent/addons/api/plan-sidebar/plan?*',route=>{
  const r=route.request(),url=new URL(r.url());
  assert.equal(url.searchParams.get('chat_jid'),state.sessionId);
  calls.push({method:r.method(),chatJid:url.searchParams.get('chat_jid'),body:r.postDataJSON?.()??null});
  if(r.method()==='GET')return route.fulfill({json:{ok:true,...stored}});
  if(r.method()==='POST'){
   const body=r.postDataJSON();assert.equal(body.chat_jid,state.sessionId);
   stored.markdown=body.markdown;stored.updated_at='2026-09-27T00:01:00.000Z';
   return route.fulfill({json:{ok:true,plan:{...stored}}});
  }
  throw Error(`Unexpected Plan request ${r.method()}`);
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
 host.assert();
 console.log(JSON.stringify({browserName,release:reference.release,addon:packageJson.name,addonVersion:packageJson.version,
  addonSha256:createHash('sha256').update(addonBytes).digest('hex'),mapSha256:reference.map.sha256,
  scope:'installed addon web source on shipped Classic assets with disposable Plan API; no production backend/live writes',
  remoteWarning:true,dirtyTextBeforeRefresh:'- [ ] unsaved local',textAfterRefresh:await editor.textContent(),refreshReads:calls.slice(beforeRefresh),writeCount:0},null,2));
}finally{await host.dispose();await browser.close();}
