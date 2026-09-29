// Mounted installed Piclaw 3.2.4 Classic Settings layering with a disposable workspace target.
// Browser hit-testing and a trusted pointer click; no real workspace file or physical pointer.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const base=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'}),state={...base,sessionId:'web:default',theme:'light'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 try{
  const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'settings-layering',revision:0,models:[],sessions:[]}}));
  await page.route('**/agent/client-perf',r=>r.fulfill({json:{ok:true}}));
  await page.route('**/agent/settings-data',r=>r.fulfill({json:{}}));
  await page.goto(host.origin);await host.connected();
  const target=await page.evaluate(()=>{const el=document.createElement('button');el.textContent='Fixture workspace control';el.dataset.fixtureWorkspace='';Object.assign(el.style,{position:'fixed',top:'4px',left:'4px',width:'70px',height:'35px',zIndex:'1'});el.addEventListener('click',()=>window.__fixtureWorkspaceClicks=(window.__fixtureWorkspaceClicks||0)+1);document.body.appendChild(el);return {x:39,y:21};});
  const point=()=>page.mouse.click(target.x,target.y);
  await point();assert.equal(await page.evaluate(()=>window.__fixtureWorkspaceClicks),1,'uncovered fixture control must receive pointer before Settings');
  await page.evaluate(()=>window.dispatchEvent(new CustomEvent('piclaw:open-settings')));
  const backdrop=page.locator('.settings-dialog-backdrop'),dialog=page.locator('[data-testid="settings-dialog"]');await dialog.waitFor({state:'visible'});
  const measured=await page.evaluate(point=>{
   const portal=document.querySelector('.settings-portal'),backdrop=document.querySelector('.settings-dialog-backdrop'),dialog=document.querySelector('[data-testid="settings-dialog"]');
   if(!backdrop||!dialog)throw Error(`Missing Settings elements: portal=${!!portal} backdrop=${!!backdrop} dialog=${!!dialog}`);
   const cover=backdrop.getBoundingClientRect(),box=dialog.getBoundingClientRect(),v={width:innerWidth,height:innerHeight};
   const center=document.elementFromPoint(box.x+box.width/2,box.y+box.height/2);
   return {portalPresent:Boolean(portal),backdropParent:backdrop.parentElement?.tagName,position:getComputedStyle(backdrop).position,backdropColor:getComputedStyle(backdrop).backgroundColor,zIndex:getComputedStyle(backdrop).zIndex,cover:{x:cover.x,y:cover.y,width:cover.width,height:cover.height},viewport:v,dialog:{x:box.x,y:box.y,width:box.width,height:box.height},centerInside:dialog.contains(center),targetCovered:Boolean(document.elementFromPoint(point.x,point.y)?.closest('.settings-dialog-backdrop'))};
  },target);
  assert.equal(measured.position,'fixed');assert.equal(measured.backdropColor,'rgba(0, 0, 0, 0.5)');assert.equal(measured.zIndex,'12000');
  assert.deepEqual(measured.cover,{x:0,y:0,...measured.viewport});
  // Wait out the installed slide-up animation before measuring the dialog bounds.
  await page.waitForTimeout(250);
  const settled=await dialog.boundingBox();assert(settled.x>=0&&settled.y>=0&&settled.x+settled.width<=measured.viewport.width+0.5&&settled.y+settled.height<=measured.viewport.height+0.5);
  assert.equal(measured.centerInside,true);assert.equal(measured.targetCovered,true);
  await point();assert.equal(await page.evaluate(()=>window.__fixtureWorkspaceClicks),1,'covered control must not receive trusted pointer');
  // Explicit backdrop dismissal uses a synthetic click; only pointer delivery
  // to the disposable control is tested with page.mouse.click above/below.
  if(await backdrop.isVisible())await page.evaluate(()=>{const backdrop=document.querySelector('.settings-dialog-backdrop');backdrop.dispatchEvent(new MouseEvent('click',{bubbles:true,cancelable:true}));});
  await backdrop.waitFor({state:'hidden'});
  await point();assert.equal(await page.evaluate(()=>window.__fixtureWorkspaceClicks),2,'dismissed overlay must restore pointer delivery');host.assert();
  cases.push({browser:browserName,viewport:viewportName,color:measured.backdropColor,fullViewport:true,dialogInsideViewport:true,centerHit:true,coveredClickBlocked:true,dismissRestoredClick:true});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4 mounted Classic Settings with disposable body button and read-only responses; browser geometry and trusted pointer at fixture control. No real workspace explorer/file read, OS/physical pointer, Visual skin or whole layering clause acceptance.',cases},null,2));
