// Installed 3.2.4 Classic presence lifecycle; browser storage, not OS delivery.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const fixture=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,sessionId:'web:default',theme:'dark'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 const presence=()=>page.evaluate(()=>Object.keys(localStorage).filter(k=>k.startsWith('piclaw.notifications.presence.')).map(k=>JSON.parse(localStorage.getItem(k))));
 try{
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'notification-presence',revision:0,models:[],sessions:[]}}));
  await page.goto(host.origin);await host.connected();
  await page.waitForFunction(()=>Object.keys(localStorage).some(k=>k.startsWith('piclaw.notifications.presence.')));
  const current=await presence();assert.equal(current.length,1);assert.equal(current[0].chatJid,state.sessionId);assert.equal(current[0].visibilityState,'visible');
  const published=host.calls.filter(c=>c.method==='POST'&&c.path==='/agent/push/presence').length;assert(published>0);
  await page.evaluate(()=>window.dispatchEvent(new Event('pagehide')));
  await page.waitForFunction(()=>!Object.keys(localStorage).some(k=>k.startsWith('piclaw.notifications.presence.')));
  assert.deepEqual(await presence(),[]);host.assert();cases.push({browser:browserName,viewport:viewportName,published,selectedChatScoped:true,withdrawn:true});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed 3.2.4 Classic UI, synthetic presence API and local browser storage. No Notification permission/delivery, server presence fanout, OS or physical-device acceptance.',cases},null,2));
