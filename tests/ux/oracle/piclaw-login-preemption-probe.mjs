// Installed Piclaw 3.2.4 login: conditional → explicit → TOTP preemption.
// Browser credential API and auth endpoints are disposable stubs.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {createServer} from 'node:http';
import {chromium,webkit} from 'playwright';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const html=(await fs.readFile(path.join(root,'app/runtime/web/static/login.html'),'utf8')).replaceAll('__LOGIN_ASSET_VERSION__','fixture');
const bundle=await fs.readFile(path.join(root,'app/runtime/web/static/common/dist/login.bundle.js'));
const stylesheet=await fs.readFile(path.join(root,'app/runtime/web/static/common/dist/login.bundle.css'));
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const server=createServer((req,res)=>res.writeHead(404).end());await new Promise(r=>server.listen(0,'127.0.0.1',r));const origin=`http://127.0.0.1:${server.address().port}`;
 const calls=[],failures=[];page.on('pageerror',e=>failures.push(e.message));
 try{
  await page.addInitScript(()=>{
   window.__passkeyCalls=[];
   window.PublicKeyCredential=class {static isConditionalMediationAvailable=async()=>true;};
   Object.defineProperty(navigator,'credentials',{configurable:true,value:{get:({mediation,signal})=>{
    window.__passkeyCalls.push({mediation,signal});
    return new Promise(resolve=>signal.addEventListener('abort',()=>resolve(null),{once:true}));
   }}});
  });
  await page.route('**/*',route=>{
   const r=route.request(),u=new URL(r.url());calls.push({path:u.pathname,method:r.method(),body:r.postDataJSON?.()});
   if(u.origin!==origin)throw Error(`Unexpected origin ${u.origin}`);
   if(u.pathname==='/login'||u.pathname==='/')return route.fulfill({contentType:'text/html',body:html});
   if(u.pathname==='/static/common/dist/login.bundle.js')return route.fulfill({contentType:'application/javascript',body:bundle});
   if(u.pathname==='/static/common/dist/login.bundle.css')return route.fulfill({contentType:'text/css',body:stylesheet});
   if(u.pathname==='/auth/options')return route.fulfill({json:{mode:'single-user',auth_enabled:true,totp:true,passkey:true,username_required:false}});
   if(u.pathname==='/auth/webauthn/login/start')return route.fulfill({json:{token:'fixture-token',options:{challenge:'AQ',allowCredentials:[]}}});
   if(u.pathname==='/auth/verify')return route.fulfill({status:503,json:{error:'Fixture verify failure'}});
   throw Error(`Unexpected request ${r.method()} ${u.pathname}`);
  });
  await page.goto(origin+'/login');
  await page.waitForFunction(()=>window.__passkeyCalls.length===1);
  assert.equal(await page.evaluate(()=>window.__passkeyCalls[0].mediation),'conditional');
  await page.locator('#passkey-button').click();
  await page.waitForFunction(()=>window.__passkeyCalls.length===2);
  const afterExplicit=await page.evaluate(()=>window.__passkeyCalls.map(c=>({mediation:c.mediation,aborted:c.signal.aborted})));
  assert.deepEqual(afterExplicit,[{mediation:'conditional',aborted:true},{mediation:'required',aborted:false}]);
  await page.locator('#code').fill('123456');await page.locator('#verify-button').click();
  await page.locator('#error').getByText(/Fixture verify failure/).waitFor();
  const afterCode=await page.evaluate(()=>window.__passkeyCalls.map(c=>({mediation:c.mediation,aborted:c.signal.aborted})));
  assert.equal(afterCode[1].aborted,true);
  const startCalls=calls.filter(c=>c.path==='/auth/webauthn/login/start');assert.equal(startCalls.length,2);
  assert.deepEqual(calls.filter(c=>c.path==='/auth/verify').map(c=>c.body),[{code:'123456'}]);
  assert.deepEqual(failures,[]);cases.push({browser:browserName,viewport:viewportName,ambientAbortedBeforeExplicit:true,explicitAbortedBeforeCode:true,starts:2,codePosts:1});
 }finally{await browser.close();server.closeAllConnections();await new Promise(r=>server.close(r));}
}
console.log(JSON.stringify({scope:'Installed 3.2.4 standalone login bundle, browser credential stub with abort signals and disposable HTTP; no real WebAuthn/TOTP, session or physical prompt',cases},null,2));
