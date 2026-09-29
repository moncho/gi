// Installed Piclaw 3.2.4 family client, disposable identity/timeline/logout endpoints.
// No real family account, server authorisation, OS notifications or physical focus.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {createServer} from 'node:http';
import {chromium,webkit} from 'playwright';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const html=(await fs.readFile(path.join(root,'app/runtime/web/static/family.html'),'utf8')).replaceAll('__FAMILY_ASSET_VERSION__','fixture');
const bundle=await fs.readFile(path.join(root,'app/runtime/web/static/common/dist/family.bundle.js'));
const staticRoot=path.join(root,'app/runtime/web/static');
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const server=createServer((req,res)=>{
  if(req.url?.startsWith('/sse/')){res.writeHead(200,{'Content-Type':'text/event-stream','Cache-Control':'no-store'});res.write(': fixture\n\n');return;}
  res.writeHead(404).end();
 });await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 const origin=`http://127.0.0.1:${server.address().port}`,browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'}),calls=[],failures=[];
 page.on('pageerror',e=>failures.push(e.message));
 const principal={kind:'user',mode:'family-shared',role:'member',userId:'fixture-user',username:'fixture',displayName:'Fixture User',authentication:{sessionId:'fixture-login'},homeChatJid:'web:fixture'};
 let logoutStatus=503,identityReads=0,releaseLogout;
 const logoutGate=new Promise(resolve=>releaseLogout=resolve);
 try{
  await page.route('**/*',route=>{
   const r=route.request(),u=new URL(r.url());calls.push({method:r.method(),path:u.pathname});
   if(u.origin!==origin)throw Error(`Unexpected origin ${u.origin}`);
   if(u.pathname==='/family')return route.fulfill({contentType:'text/html',body:html});
   if(u.pathname==='/static/common/dist/family.bundle.js')return route.fulfill({contentType:'application/javascript',body:bundle});
   if(u.pathname.startsWith('/static/')||['/manifest.json','/favicon.ico','/apple-touch-icon.png','/apple-touch-icon-180x180.png','/apple-touch-icon-167x167.png','/apple-touch-icon-152x152.png','/apple-touch-icon-precomposed.png'].includes(u.pathname)){
    const relative=u.pathname.replace(/^\/static\//,'').replace(/^\//,'');const file=path.resolve(staticRoot,relative);
    if(!file.startsWith(staticRoot+path.sep))throw Error('Asset outside fixture root');
    return fs.readFile(file).then(body=>route.fulfill({contentType:file.endsWith('.css')?'text/css':file.endsWith('.js')?'application/javascript':file.endsWith('.json')?'application/json':'application/octet-stream',body}));
   }
   if(u.pathname==='/auth/me'){identityReads++;return route.fulfill({json:{principal,capabilities:{manage_users:false}}});}
   if(u.pathname==='/auth/logout')return (logoutStatus===503?logoutGate:Promise.resolve()).then(()=>route.fulfill({status:logoutStatus,json:logoutStatus===200?{ok:true}:{error:'Fixture logout failure'}}));
   if(u.pathname==='/login')return route.fulfill({contentType:'text/html',body:'<!doctype html><title>Fixture sign-in</title>'});
   if(u.pathname==='/agent/branches')return route.fulfill({json:{branches:[]}});
   if(u.pathname==='/agent/picker-pins')return route.fulfill({json:{scope:'family-fixture',revision:0,models:[],sessions:[]}});
   if(u.pathname==='/agent/commands')return route.fulfill({json:{commands:[]}});
   if(u.pathname==='/timeline')return route.fulfill({json:{posts:[],has_more:false}});
   if(u.pathname==='/agent/message-recovery')return route.fulfill({json:{state:'idle'}});
   if(u.pathname==='/account/preferences')return route.fulfill({json:{user_id:'fixture-user',can_edit:true,preferences:{revision:0,theme:'system',response_guidance:''},defaults:{theme:'system',response_guidance:''}}});
   if(u.pathname==='/agent/models')return route.fulfill({json:{current:'test/fixture',model_options:[]}});
   if(u.pathname==='/agent/status')return route.fulfill({json:{status:{status:'idle'}}});
   if(u.pathname==='/agent/context')return route.fulfill({json:{}});
   if(u.pathname==='/agent/queue-state')return route.fulfill({json:{items:[],count:0}});
   if(u.pathname.startsWith('/sse/'))return route.continue();
   throw Error(`Unexpected request ${r.method()} ${u.pathname}`);
  });
  await page.goto(origin+'/family',{waitUntil:'domcontentloaded'});
  await page.locator('#account-name').getByText('Fixture User (@fixture)').waitFor();
  const logout=page.locator('#sign-out');assert.equal(await logout.isEnabled(),true);
  assert.ok(identityReads>=1);const beforeBlur=identityReads;
  await page.evaluate(()=>window.dispatchEvent(new Event('blur')));
  await page.waitForFunction(()=>document.querySelector('#account-name')?.textContent==='');
  assert.equal(await page.locator('#refresh').isEnabled(),false);
  await page.evaluate(()=>window.dispatchEvent(new Event('focus')));
  await page.locator('#account-name').getByText('Fixture User (@fixture)').waitFor();
  assert.equal(await logout.isEnabled(),true);
  assert.ok(identityReads>beforeBlur,'resume must recheck identity');
  await page.evaluate(()=>{Object.defineProperty(document,'hidden',{configurable:true,value:true});document.dispatchEvent(new Event('visibilitychange'));});
  await page.waitForFunction(()=>document.querySelector('#account-name')?.textContent==='');
  await page.evaluate(()=>{delete document.hidden;document.dispatchEvent(new Event('visibilitychange'));});
  await page.locator('#account-name').getByText('Fixture User (@fixture)').waitFor();
  await logout.click();await page.waitForFunction(()=>document.querySelector('#sign-out')?.disabled===true);
  assert.equal(calls.filter(c=>c.path==='/auth/logout'&&c.method==='POST').length,1);
  releaseLogout();await page.locator('#family-error').getByText('Sign out failed. Try again.').waitFor({timeout:6000});
  assert.equal(await logout.isEnabled(),true);
  assert.equal(calls.filter(c=>c.path==='/auth/logout'&&c.method==='POST').length,1);
  logoutStatus=200;await logout.click();await page.waitForURL(origin+'/login');
  assert.equal(calls.filter(c=>c.path==='/auth/logout'&&c.method==='POST').length,2);
  assert.deepEqual(failures,[]);
  cases.push({browser:browserName,viewport:viewportName,maskedOnSyntheticBlur:true,maskedOnSyntheticHidden:true,resumedOnSyntheticFocus:true,disabledDuringLogout:true,failedLogoutRetry:true,successNavigatedToLogin:true});
 }finally{releaseLogout();await browser.close();server.closeAllConnections();await new Promise(resolve=>server.close(resolve));}
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4 family bundle, synthetic focus and disposable identity/timeline/logout; no real family auth, notification cleanup, server masking enforcement or physical focus',cases},null,2));
