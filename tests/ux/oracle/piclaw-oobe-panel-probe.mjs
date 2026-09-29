// Installed Piclaw 3.2.4 provider-missing OOBE, disposable model catalogue.
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
 const browser=await type.launch({headless:true});
 try{
  for(const mode of ['missing','available','current-hint']){
   const page=await browser.newPage({viewport,serviceWorkers:'block'});
   const model=mode==='available'?{...fixture.model,current:null,model:null}
    :{...fixture.model,current:mode==='current-hint'?'test/fixture':null,model:null,model_options:[],available_model_count:0};
   const state={...fixture,sessionId:'web:default',theme:'dark',model};
   const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
   try{
    await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
    await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'oobe-oracle',revision:0,models:[],sessions:[]}}));
    await page.route('**/agent/client-perf',r=>r.fulfill({json:{ok:true}}));
    await page.route('**/agent/settings-data',r=>r.fulfill({json:{}}));
    const statusResponse=page.waitForResponse(r=>new URL(r.url()).pathname==='/agent/status'&&r.status()===200);
    await page.goto(host.origin);await statusResponse;await host.connected();
    await page.locator('.compose-box textarea').waitFor();
    const pane=page.locator('.oobe-panel-provider-missing');
    if(mode==='missing'){
     await pane.waitFor();assert.match(await pane.innerText(),/Getting started/i);
     const setup=pane.getByRole('button',{name:'Open settings'}),dismiss=pane.getByRole('button',{name:'Dismiss'});
     assert.equal(await setup.isEnabled(),true);await setup.click();await page.locator('.settings-dialog').waitFor();
     await page.keyboard.press('Escape');await page.locator('.settings-dialog').waitFor({state:'hidden'});
     await dismiss.click();await pane.waitFor({state:'hidden'});
     assert.equal(await page.evaluate(()=>localStorage.getItem('piclaw:oobe:provider-missing:dismissed')),'true');
    }else assert.equal(await page.locator('.oobe-panel').count(),0,`${mode} must hide OOBE`);
    host.assert();cases.push({browser:browserName,viewport:viewportName,mode,panel:mode==='missing'?'dismissed':'hidden'});
   }finally{await host.dispose();await page.close();}
  }
 }finally{await browser.close();}
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4 Classic UI with disposable status/model catalogue; no real provider setup, reload persistence, unresolved readiness, popout or physical input',cases},null,2));
