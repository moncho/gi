// Shipped Piclaw 3.2.4 UI, isolated upload and message endpoints.
// Does not exercise production storage, provider delivery or retry deduplication.
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(root+'/VERSION','utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const fixture=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const output=path.resolve('test-results/ux-oracle/upload-state');await fs.mkdir(output,{recursive:true});const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,sessionId:'web:default',theme:'light'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 const uploads=[],messages=[];let fail=false;
 try{
  const html=(await fs.readFile(root+'/app/runtime/web/static/classic/index.html','utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'upload',revision:0,models:[],sessions:[]}}));
  await page.route('**/media/upload',r=>{
   assert.equal(r.request().method(),'POST');
   const body=r.request().postDataBuffer();uploads.push({failed:fail,hasFilename:body.includes(Buffer.from(fail?'rejected.txt':'accepted.txt'))});
   assert(uploads.at(-1).hasFilename);
   return fail?r.fulfill({status:503,json:{error:'fixture upload refused'}}):r.fulfill({json:{id:71,name:'accepted.txt'}});
  });
  await page.route('**/agent/default/message?*',r=>{const url=new URL(r.request().url());assert.equal(url.searchParams.get('chat_jid'),'web:default');messages.push(r.request().postDataJSON());return r.fulfill({json:{ok:true,turn_id:'fixture-upload-turn'}});});
  await page.goto(host.origin);await host.connected();const input=page.locator('.compose-box textarea');await input.waitFor();
  const media=page.locator('.compose-box input[type=file]');await input.fill('send accepted file');
  await media.setInputFiles({name:'accepted.txt',mimeType:'text/plain',buffer:Buffer.from('accepted bytes')});
  await page.locator('.compose-file-pill[title="accepted.txt"]').waitFor();await input.press('Enter');
  await page.waitForFunction(()=>document.querySelector('.compose-box textarea')?.value==='');
  for(let i=0;i<50&&messages.length===0;i++)await page.waitForTimeout(40);
  assert.equal(uploads.length,1);assert.equal(messages.length,1);
  assert.deepEqual(messages[0].media_ids,[71]);assert(messages[0].content.includes('send accepted file'));
  fail=true;await input.fill('retain rejected file');
  await media.setInputFiles({name:'rejected.txt',mimeType:'text/plain',buffer:Buffer.from('rejected bytes')});
  await page.locator('.compose-file-pill[title="rejected.txt"]').waitFor();await input.press('Enter');
  await page.locator('.compose-inline-status-detail').getByText('fixture upload refused',{exact:true}).waitFor();
  assert.equal(uploads.length,2);assert.equal(messages.length,1);
  assert.equal(await input.inputValue(),'retain rejected file');
  assert.equal(await page.locator('.compose-file-pill[title="rejected.txt"]').count(),1);
  host.assert();cases.push({browser:browserName,viewport:viewportName,result:'pass',uploads,messages,failedDraft:await input.inputValue()});
 }finally{await fs.writeFile(path.join(output,'evidence.json'),JSON.stringify({scope:'Installed Piclaw3.2.4 browser UI with isolated upload/message fixtures; no native backend, provider or retry-deduplication claim.',cases},null,2));await host.dispose();await browser.close();}
}
console.log(JSON.stringify({output,cases}));
