// Shipped Piclaw 3.2.4 persisted-widget browser slice; disposable timeline only.
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
const widgets=[
 {title:'HTML empty',artifact:{kind:'html',html:''},usable:false},
 {title:'HTML ready',artifact:{kind:'html',html:'<p>Stored HTML proof</p>'},usable:true},
 {title:'SVG empty',artifact:{kind:'svg',svg:''},usable:false},
 {title:'SVG ready',artifact:{kind:'svg',svg:'<svg xmlns="http://www.w3.org/2000/svg"><text y="20">Stored SVG proof</text></svg>'},usable:true},
];
const posts=widgets.map((item,index)=>({id:801+index,timestamp:fixture.now,chat_jid:'web:default',data:{type:'agent_response',content:'Widget artifact fixture',agent_id:'default',is_bot_message:true,content_blocks:[{type:'generated_widget',widget_id:`persisted-${index}`,title:item.title,capabilities:['interactive'],artifact:item.artifact}]}}));
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,sessionId:'web:default',theme:'dark'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 try{
  // No service worker is needed for this persisted-timeline fixture.
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'widget-persisted',revision:0,models:[],sessions:[]}}));
  await page.route('**/timeline?*',r=>r.fulfill({json:{posts,has_more:false}}));
  await page.goto(host.origin);await host.connected();await page.locator('#post-804').waitFor();
  const observed=[];
  for(let index=0;index<widgets.length;index++){
   const item=widgets[index],post=page.locator(`#post-${801+index}`);
   const button=post.getByRole('button',{name:'Open widget'});
   await button.waitFor();assert.equal(await button.isEnabled(),item.usable,`${item.title}: launch state`);
   if(item.usable){
    await button.click();const pane=page.locator('.floating-widget-pane');await pane.waitFor();
    const frame=pane.locator('iframe.floating-widget-frame');await frame.waitFor();
    assert((await frame.getAttribute('srcdoc'))?.includes(index===1?'Stored HTML proof':'Stored SVG proof'),`${item.title}: stored artifact in iframe`);
    await frame.contentFrame().locator(index===1?'p':'.widget-svg-shell svg').waitFor();
    await pane.getByRole('button',{name:'Close widget'}).click();await pane.waitFor({state:'hidden'});
   }
   observed.push({title:item.title,launchEnabled:item.usable});
  }
  const expectedSandboxErrors=host.failures.filter(e=>/^page: (?:Failed to read the 'serviceWorker' property from 'Navigator': )?Service [Ww]orker is disabled because the context is sandboxed and lacks the 'allow-same-origin' flag\.?$/.test(e));
  assert.equal(expectedSandboxErrors.length,1,'one opaque SVG sandbox getter error');
  assert.deepEqual(host.failures,expectedSandboxErrors,'no unrelated page or network errors');
  host.failures.length=0;host.assert();cases.push({browser:browserName,viewport:viewportName,observed,opaqueSvgSandboxErrors:expectedSandboxErrors.length});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4, disposable persisted timeline; no live widget SSE/backend persistence/provider/queue or physical input',cases},null,2));
