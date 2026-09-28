import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(root+'/VERSION','utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
assert.equal(createHash('sha256').update(await fs.readFile(root+'/app/runtime/web/static/classic/dist/app.bundle.js.map')).digest('hex'),reference.map.sha256);
const fixture=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const output=path.resolve('test-results/ux-oracle/quick-action-prefill');await fs.mkdir(output,{recursive:true});const cases=[];
for(const [browserName,type]of Object.entries({chromium,webkit}))for(const [viewportName,viewport]of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,theme:'dark',sessionId:'web:default',commands:[...fixture.commands,{name:'/skill:proof',description:'Oracle skill fixture'}]};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});const requests=[];
 try{
  const html=(await fs.readFile(root+'/app/runtime/web/static/classic/index.html','utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'quick-actions',revision:0,models:[],sessions:[]}}));
  await page.route('**/agent/default/message?*',r=>{requests.push(r.request().postDataJSON());return r.fulfill({status:500,json:{error:'unexpected send'}})});
  await page.goto(host.origin);await host.connected();const input=page.locator('.compose-box textarea');await input.waitFor();
  const media=page.locator('.compose-box input[type=file]');await media.setInputFiles({name:'retained.txt',mimeType:'text/plain',buffer:Buffer.from('keep attachment')});const pill=page.locator('.compose-file-pill[title="retained.txt"]');await pill.waitFor();
  const open=async letter=>{await input.blur();await page.locator('.timeline').click({position:{x:150,y:90}});await page.keyboard.press(letter);await page.locator('.timeline-quick-actions-input').waitFor();};
  await input.fill('ORACLE OLD MODEL DRAFT');await open('m');await page.locator('.timeline-quick-actions-input').fill('/model');await page.locator('.timeline-quick-actions-item-slash').filter({hasText:'/model'}).first().click();await page.locator('.timeline-quick-actions').waitFor({state:'hidden'});
  await page.waitForFunction(()=>document.activeElement===document.querySelector('.compose-box textarea'));
  const model={value:await input.inputValue(),focused:await input.evaluate(e=>document.activeElement===e),cursor:await input.evaluate(e=>e.selectionStart),mediaPills:await pill.count()};
  await input.fill('ORACLE OLD SKILL DRAFT');await open('p');await page.locator('.timeline-quick-actions-input').fill('/skill:proof');await page.locator('.timeline-quick-actions-item-slash').filter({hasText:'/skill:proof'}).first().click();await page.locator('.timeline-quick-actions').waitFor({state:'hidden'});
  await page.waitForFunction(()=>document.activeElement===document.querySelector('.compose-box textarea'));
  const skill={value:await input.inputValue(),focused:await input.evaluate(e=>document.activeElement===e),cursor:await input.evaluate(e=>e.selectionStart),mediaPills:await pill.count()};
  const result={browser:browserName,viewport:viewportName,model,skill,requests:requests.length};cases.push(result);
  assert.equal(model.value,'/model');assert.equal(skill.value,'/skill:proof');assert.equal(model.focused,true);assert.equal(skill.focused,true);assert.equal(model.mediaPills,1);assert.equal(skill.mediaPills,1);assert.equal(requests.length,0);host.assert();
  await page.screenshot({path:path.join(output,`${browserName}-${viewportName}.png`)});
 }finally{await fs.writeFile(path.join(output,'evidence.json'),JSON.stringify({scope:'Installed Piclaw3.2.4 UI with isolated APIs; no provider mutation or physical keyboard claim.',mapSha256:reference.map.sha256,cases},null,2));await host.dispose();await browser.close()}
}
console.log(JSON.stringify({output,cases}));
