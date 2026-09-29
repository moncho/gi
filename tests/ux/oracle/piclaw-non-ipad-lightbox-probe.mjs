// Installed Piclaw 3.2.4 Classic non-iPad stored-image activation against disposable timeline/media reads.
// Compares the lightbox Gi already has; no iPad annotator or persistent-highlight acceptance.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const base=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const image=await fs.readFile(path.join(root,'app/runtime/web/static/icon-192.png'));
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true});
 const context=await browser.newContext({viewport,serviceWorkers:'block',hasTouch:true,userAgent:'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1'});
 const page=await context.newPage(),state={...base,sessionId:'web:default',theme:'dark'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 const mediaReads=[],writes=[];
 page.on('request',r=>{const url=new URL(r.url());if(!['GET','HEAD'].includes(r.method()))writes.push(`${r.method()} ${url.pathname}`);});
 try{
  const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
  const posts=[{id:731,chat_jid:state.sessionId,timestamp:state.now,data:{type:'user_message',content:'Fixture image',media_ids:[731],content_blocks:[{type:'image',mime_type:'image/png'}]}}];
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'non-ipad-lightbox',revision:0,models:[],sessions:[]}}));
  await page.route('**/timeline?*',r=>r.fulfill({json:{posts,has_more:false}}));
  await page.route(/\/media\/731(?:\/.*)?$/,r=>{const u=new URL(r.request().url());mediaReads.push(u.pathname);if(u.pathname.endsWith('/info'))return r.fulfill({json:{id:731,mime_type:'image/png',content_type:'image/png',filename:'fixture.png',width:192,height:192}});return r.fulfill({contentType:'image/png',body:image});});
  await page.goto(host.origin);await host.connected();
  const identity=await page.evaluate(()=>({ua:navigator.userAgent,platform:navigator.platform,touch:navigator.maxTouchPoints}));
  assert.equal(/iPad/i.test(identity.ua)||(identity.platform==='MacIntel'&&identity.touch>1),false);
  const thumbnail=page.locator('#post-731 .media-preview img');await thumbnail.waitFor({state:'visible'});
  await page.waitForFunction(()=>{const img=document.querySelector('#post-731 .media-preview img');return img?.complete&&img.naturalWidth>0;});
  const draft=page.locator('.compose-box textarea');await draft.fill('keep unsent lightbox draft');
  const modal=page.locator('.image-modal');
  await thumbnail.click();await modal.waitFor({state:'visible'});
  assert.equal(await page.locator('.image-annotator').count(),0);
  assert.equal(new URL(await modal.locator('img').getAttribute('src'),host.origin).pathname,'/media/731');
  await page.waitForFunction(()=>{const img=document.querySelector('.image-modal img');return img?.complete&&img.naturalWidth>0;});
  await page.keyboard.press('Escape');await modal.waitFor({state:'hidden'});
  await thumbnail.tap();await modal.waitFor({state:'visible'});assert.equal(await page.locator('.image-annotator').count(),0);
  assert.equal(await draft.inputValue(),'keep unsent lightbox draft');
  assert.deepEqual(writes.filter(x=>!['POST /agent/push/presence','POST /workspace/visibility'].includes(x)),[]);
  assert(mediaReads.includes('/media/731/thumbnail')&&mediaReads.includes('/media/731'));
  host.assert();cases.push({browser:browserName,viewport:viewportName,nonIPad:true,clickAndTapOpenLightbox:true,annotatorAbsent:true,escapeCloses:true,draftRetained:true,mediaPaths:[...new Set(mediaReads)]});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Mounted installed Piclaw 3.2.4 Classic, disposable post and PNG responses, emulated non-iPad user agent and browser tap. No Gi iPad-positive path, real media store, physical touch, annotation upload or full @ux-timeline-006 acceptance.',cases},null,2));
