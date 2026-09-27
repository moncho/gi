// Shipped Classic composer and theme SSE, backed only by a disposable API fixture.
// No installed theme backend, production HTTP route, database or live chat writes.
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import path from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'../../..');
const oracleRoot=path.resolve(process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current');
const browserName=process.env.ORACLE_BROWSER||'chromium';
if(!['chromium','webkit'].includes(browserName))throw Error('Unsupported browser');
const reference=JSON.parse(await readFile(path.join(root,'tests/ux/oracle/piclaw-3.2.4-reference.json'),'utf8'));
assert.equal((await readFile(path.join(oracleRoot,'VERSION'),'utf8')).trim(),'3.2.4');
const map=await readFile(path.join(oracleRoot,'app/runtime/web/static/classic/dist/app.bundle.js.map'));
assert.equal(createHash('sha256').update(map).digest('hex'),reference.map.sha256);
const state=JSON.parse(await readFile(path.join(root,'tests/ux/fixtures/compose-pixel-state.json'),'utf8'));
state.sessionId='web:default';state.theme='light';
const source=pathToFileURL(path.join(oracleRoot,'app/runtime/src/channels/web/theming/ui-theme-commands.ts')).href;
const {handleUiThemeCommand}=await import(source);
const browser=await (browserName==='chromium'?chromium:webkit).launch({headless:true});
const page=await browser.newPage({viewport:{width:1440,height:900},serviceWorkers:'block'});
const host=await installPixelHost({page,host:'piclaw',root:oracleRoot,state,reference});
const requests=[],outcomes=[];
let timeline=[];
const originalHTML=await readFile(path.join(oracleRoot,'app/runtime/web/static/classic/index.html'),'utf8');
const input=page.locator('.compose-box textarea');
async function submit(command){
 await input.fill(command);
 await input.press('Enter');
 for(let n=0;n<40&&requests.length<outcomes.length+1;n++)await page.waitForTimeout(25);
 assert.equal(requests.at(-1)?.content,command);
 const result=handleUiThemeCommand(command);
 assert.ok(result);
 outcomes.push({command,status:result.status,payload:result.payload??null});
 return result;
}
try{
 await page.route(host.origin+'/',route=>route.fulfill({contentType:'text/html',body:originalHTML.replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1')}));
 await page.route('**/agent/picker-pins',route=>route.fulfill({json:{scope:state.sessionId,revision:1,models:[],sessions:[]}}));
 await page.route('**/timeline?*',route=>route.fulfill({json:{posts:timeline,has_more:false}}));
 await page.route('**/agent/default/message?*',route=>{
  const data=route.request().postDataJSON();
  requests.push({method:route.request().method(),url:route.request().url(),content:data.content});
  const result=handleUiThemeCommand(data.content);
  assert.ok(result);
  if(result.payload)host.emit('ui_theme',{theme:result.payload.theme,tint:result.payload.tint});
  const post={id:100+requests.length,chat_jid:state.sessionId,timestamp:state.now,
   data:{type:'agent_response',content:result.message,agent_id:'default',is_bot_message:true}};
  timeline=[...timeline,post];
  host.emit('new_post',post);
  return route.fulfill({json:{ok:true}});
 });
 await page.goto(host.origin);await input.waitFor();await host.connected();
 await submit('/theme ristretto');
 await page.waitForFunction(()=>document.documentElement.getAttribute('data-color-theme')==='ristretto');
 assert.equal(await page.evaluate(()=>localStorage.getItem('piclaw_theme')),'ristretto');
 await submit('/tint #e11d48');
 await page.waitForFunction(()=>document.documentElement.getAttribute('data-tint')==='#e11d48');
 assert.equal(await page.evaluate(()=>localStorage.getItem('piclaw_tint')),'#e11d48');
 await page.locator('.timeline').getByText('Tint set to',{exact:false}).waitFor();
 assert.equal(await page.locator('.timeline').getByText('Tint set to',{exact:false}).count(),1);
 host.assert();
 console.log(JSON.stringify({browserName,version:'3.2.4',mapSha256:reference.map.sha256,
  scope:'shipped Classic UI with disposable /agent fixture and installed parser; no backend/production HTTP/live writes',requests,outcomes,
  theme:await page.evaluate(()=>({colorTheme:document.documentElement.getAttribute('data-color-theme'),tint:document.documentElement.getAttribute('data-tint'),storedTheme:localStorage.getItem('piclaw_theme'),storedTint:localStorage.getItem('piclaw_tint')}))},null,2));
}finally{await host.dispose();await browser.close();}
