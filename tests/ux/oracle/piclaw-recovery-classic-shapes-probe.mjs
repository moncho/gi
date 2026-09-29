// Piclaw 3.2.4 Classic-shaped recovery extras; disposable timeline only.
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
const marker={type:'turn_outcome_marker',kind:'recovery',severity:'info'};
const items=[
 {name:'empty',content:'',blocks:[marker],visible:false},
 {name:'media',content:'',blocks:[marker,{type:'file',name:'recovery-retained.txt',mime_type:'text/plain'}],media_ids:[71],visible:true},
 {name:'file-ref',content:'Files:\n- keep.md',blocks:[marker],visible:false},
 {name:'message-ref',content:'Referenced messages:\n- message:recovery-authored',blocks:[marker],visible:false},
 {name:'attachment-ref',content:'Attachments:\n- retained-name.txt',blocks:[marker],visible:false},
 {name:'resource',content:'',blocks:[marker,{type:'resource',uri:'text://recovery',name:'Retained resource',mimeType:'text/plain',text:'Resource bytes'}],visible:false},
 {name:'text-annotation',content:'',blocks:[marker,{type:'text',annotations:{priority:0.5}}],visible:false},
 {name:'authored',content:'Recovery authored text',blocks:[marker],visible:true},
];
const posts=items.map((x,i)=>({id:1201+i,timestamp:fixture.now,chat_jid:'web:default',data:{type:'agent_response',content:x.content,agent_id:'default',is_bot_message:true,content_blocks:x.blocks,...(x.media_ids?{media_ids:x.media_ids}:{})}}));
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,sessionId:'web:default',theme:'dark'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 try{
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'recovery-classic-shapes',revision:0,models:[],sessions:[]}}));
  await page.route('**/timeline?*',r=>r.fulfill({json:{posts,has_more:false}}));
  await page.route('**/media/71/info',r=>r.fulfill({json:{id:71,name:'recovery-retained.txt',filename:'recovery-retained.txt',mime_type:'text/plain',size:22}}));
  await page.goto(host.origin);await host.connected();await page.locator('#post-1208').waitFor();
  const observed=[];
  for(let i=0;i<items.length;i++){
   const x=items[i],visible=await page.locator(`#post-${1201+i}`).count();
   observed.push({name:x.name,visible:Boolean(visible)});
   assert.equal(visible,x.visible?1:0,`${x.name}: mounted post count`);
  }
  host.assert();cases.push({browser:browserName,viewport:viewportName,observed});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed Classic UI, synthetic recovery posts with native media_ids and Classic content parsing; no stored media bytes, backend recovery or physical-device acceptance',cases},null,2));
