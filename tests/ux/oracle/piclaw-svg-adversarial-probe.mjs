// Shipped Piclaw 3.2.4 SVG safety checks with a disposable timeline.
// The browser runs the installed renderer, not Gi's pinned adapter.
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const fixture=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
const fences=[
 {name:'safe',source:'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20"><title>Safe vector</title><rect width="10" height="10" fill="red"/></svg>\n',preview:true},
 {name:'strip-attributes',source:'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20"><rect style="fill:url(https://example.invalid/svg-tracker)" onclick="window.svgAttack=true" width="10" height="10" fill="red"/></svg>\n',preview:true},
 {name:'script-and-external',source:'<svg xmlns="http://www.w3.org/2000/svg" onload="window.svgAttack=true"><script>window.svgAttack=true</script><image href="https://example.invalid/svg-tracker"/></svg>\n',preview:false},
 {name:'malformed',source:'<svg xmlns="http://www.w3.org/2000/svg"><text>unescaped & text</text></svg>\n',preview:false},
 {name:'doctype',source:'<!DOCTYPE svg [<!ENTITY bad SYSTEM "https://example.invalid/svg-tracker">]><svg xmlns="http://www.w3.org/2000/svg"><text>&bad;</text></svg>\n',preview:false},
 {name:'oversized',source:'<svg xmlns="http://www.w3.org/2000/svg"><text>'+'x'.repeat(256*1024)+'</text></svg>\n',preview:false},
];
const posts=fences.map((item,index)=>({id:301+index,timestamp:fixture.now,chat_jid:'web:default',data:{type:'agent_response',content:'```svg\n'+item.source+'```',agent_id:'default',is_bot_message:true}}));
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit})) for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,sessionId:'web:default',theme:'light'},host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 const requested=[];page.on('request',r=>{if(r.url().includes('svg-tracker'))requested.push(r.url());});
 try{
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'svg-adversarial',revision:0,models:[],sessions:[]}}));
  await page.route('**/timeline?*',r=>r.fulfill({json:{posts,has_more:false}}));
  await page.goto(host.origin);await host.connected();
  await page.locator('#post-306').waitFor();
  const observed=[];
  for(let index=0;index<fences.length;index++){
   const item=fences[index],post=page.locator(`#post-${301+index}`);
   const previews=post.locator('.model-svg-block img.model-svg-image');
   assert.equal(await previews.count(),item.preview?1:0,`${item.name}: image count`);
   assert.equal(await post.locator('.post-content .model-svg-block > svg').count(),0,`${item.name}: no privileged inline model SVG`);
   if(item.preview){
    const image=previews.first();assert.match(await image.getAttribute('src'),/^data:image\/svg\+xml;base64,/);
    const xml=Buffer.from((await image.getAttribute('src')).split(',')[1],'base64').toString('utf8');
    const source=await post.locator('.model-svg-source [data-svg-source]').getAttribute('data-svg-source');
    assert.equal(Buffer.from(source,'base64').toString('utf8'),item.source,`${item.name}: source retained`);
    if(item.name==='strip-attributes'){
     assert(xml.includes('<rect'));for(const unsafe of ['style=','onclick=','svg-tracker'])assert(!xml.includes(unsafe),`${unsafe} leaked into image`);
    }
    observed.push({name:item.name,preview:true,label:await image.getAttribute('alt')});
   }else{
    const escaped=post.locator('pre code.language-svg');assert.equal(await escaped.count(),1,`${item.name}: escaped fallback`);
    assert.equal(await escaped.textContent(),item.source,`${item.name}: exact source`);
    observed.push({name:item.name,preview:false});
   }
  }
  assert.equal(await page.evaluate(()=>!!window.svgAttack),false);assert.deepEqual(requested,[]);host.assert();
  cases.push({browser:browserName,viewport:viewportName,observed,result:'pass'});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4 shipped UI, disposable messages and native sanitizer; no live provider/HTTP store or physical-device acceptance',cases},null,2));
