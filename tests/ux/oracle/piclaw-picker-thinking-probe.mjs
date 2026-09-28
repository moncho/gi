import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';assert.equal((await fs.readFile(root+'/VERSION','utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const fixture=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const output=path.resolve('test-results/ux-oracle/picker-thinking');await fs.mkdir(output,{recursive:true});const cases=[];
for(const [browserName,type]of Object.entries({chromium,webkit}))for(const [viewportName,viewport]of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,theme:'dark',sessionId:'web:default',model:{...fixture.model,thinking_level:'low',thinking_level_label:'low',supports_thinking:true,model_options:[{...fixture.model.model_options[0],reasoning:true,current:true,thinking_levels:['low','high'],thinking_level_labels:['Low','High']}]}};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});const commands=[];let release;let held=false;
 try{
  const html=(await fs.readFile(root+'/app/runtime/web/static/classic/index.html','utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'thinking',revision:0,models:[],sessions:[]}}));
  await page.route('**/agent/models?*',r=>r.fulfill({json:{...state.model,models:state.model.model_options,available_model_count:1}}));
  const gate=new Promise(r=>release=r);
  await page.route('**/agent/default/message?*',async r=>{const body=r.request().postDataJSON();commands.push(body);held=true;await gate;assert.equal(body.content,'/thinking high');state.model.thinking_level='high';state.model.thinking_level_label='high';await r.fulfill({json:{status:'ok'}});});
  await page.goto(host.origin);await host.connected();const input=page.locator('.compose-box textarea');await input.fill('keep oracle draft');const trigger=page.getByRole('button',{name:'Open model picker',exact:true});await trigger.click();
  const select=page.getByRole('combobox',{name:'Thinking level',exact:true});await select.waitFor();assert(await select.isEnabled());assert.equal(await select.inputValue(),'low');await select.focus();await select.selectOption('high');
  await page.waitForFunction(()=>document.querySelector('[aria-label="Thinking level"]')?.disabled);assert(held);assert.equal(commands.length,1);release();
  await page.waitForFunction(()=>!document.querySelector('[aria-label="Thinking level"]')?.disabled&&document.querySelector('[aria-label="Thinking level"]')?.value==='high');
  const focused=await select.evaluate(e=>document.activeElement===e);assert.equal(await input.inputValue(),'keep oracle draft');assert.equal(commands.length,1);
  // Changing thinking does not close the picker, and explicit Escape does.
  assert.equal(await page.getByRole('listbox',{name:'Models',exact:true}).count(),1);await select.focus();await page.keyboard.press('Escape');await trigger.waitFor();await page.waitForFunction(()=>!document.querySelector('[aria-label="Thinking level"]'));await page.waitForFunction(()=>document.activeElement?.getAttribute('aria-label')==='Open model picker');host.assert();
  cases.push({browser:browserName,viewport:viewportName,result:'pass',commands,thinkingFocusedAfter:focused,pickerRemainedOpen:true,draft:'keep oracle draft'});
 }finally{release?.();await fs.writeFile(path.join(output,'evidence.json'),JSON.stringify({scope:'Installed Piclaw3.2.4 shipped picker with isolated model/command responses; no real model mutation.',cases},null,2));await host.dispose();await browser.close();}
}
console.log(JSON.stringify({output,cases}));
