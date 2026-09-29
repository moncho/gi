// Installed Piclaw 3.2.4 standalone login policy and code sign-in slice.
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
 const server=createServer((req,res)=>{res.writeHead(404).end();});await new Promise(r=>server.listen(0,'127.0.0.1',r));const origin=`http://127.0.0.1:${server.address().port}`;
 const failures=[],requests=[];page.on('pageerror',e=>failures.push(e.message));
 const policy=(mode,totp,passkey)=>({mode,auth_enabled:true,totp,passkey,username_required:mode==='family-shared'&&totp});
 let current=policy('family-shared',true,false),failOptions=false;
 try{
  await page.addInitScript(()=>{window.PublicKeyCredential=undefined;});
  await page.route('**/*',route=>{
   const r=route.request(),u=new URL(r.url());requests.push({path:u.pathname,method:r.method(),body:r.postDataJSON?.()});
   if(u.origin!==origin)throw Error(`Unexpected origin ${u.origin}`);
   if(u.pathname==='/login' || u.pathname==='/')return route.fulfill({contentType:'text/html',body:html});
   if(u.pathname==='/static/common/dist/login.bundle.js')return route.fulfill({contentType:'application/javascript',body:bundle});
   if(u.pathname==='/static/common/dist/login.bundle.css')return route.fulfill({contentType:'text/css',body:stylesheet});
   if(u.pathname==='/auth/options')return route.fulfill(failOptions?{status:503,json:{error:'Fixture options failure'}}:{json:current});
   if(u.pathname==='/auth/verify')return route.fulfill({status:503,json:{error:'Fixture verify failure'}});
   throw Error(`Unexpected request ${r.method()} ${u.pathname}`);
  });
  await page.goto(origin+'/login');
  const username=page.locator('#username'),code=page.locator('#code'),form=page.locator('#login-form'),passkey=page.locator('#passkey-button'),verify=page.locator('#verify-button');
  await username.waitFor({state:'visible'});assert.equal(await username.getAttribute('required'),'');
  assert.equal(await code.isEnabled(),true);assert.equal(await verify.isEnabled(),true);
  assert.match(await page.locator('#login-description').textContent(),/account username and authenticator code/);
  await username.fill(' Alice ');await code.fill('123456');await verify.click();
  await page.locator('#error').getByText(/Fixture verify failure/).waitFor();
  const family=requests.find(r=>r.path==='/auth/verify');assert.deepEqual(family.body,{username:'alice',code:'123456'});
  current=policy('single-user',true,false);await page.goto(origin+'/login');await code.waitFor({state:'visible'});
  assert.equal(await page.locator('#username-field').isHidden(),true);assert.equal(await username.isDisabled(),true);
  assert.match(await page.locator('#login-description').textContent(),/six-digit code/);
  await code.fill('123456');await verify.click();await page.locator('#error').getByText(/Fixture verify failure/).waitFor();
  assert.deepEqual(requests.filter(r=>r.path==='/auth/verify').at(-1).body,{code:'123456'});
  current=policy('single-user',false,true);await page.goto(origin+'/login');await passkey.waitFor({state:'visible'});
  assert.equal(await form.isHidden(),true);assert.match(await page.locator('#login-description').textContent(),/passkey registered for this site/);
  failOptions=true;await page.goto(origin+'/login');await page.locator('#retry-options').waitFor({state:'visible'});
  assert.equal(await form.isHidden(),true);assert.equal(await passkey.isHidden(),true);assert.match(await page.locator('#login-description').textContent(),/Sign-in options could not be loaded/);assert.match(await page.locator('#error').textContent(),/Retry before entering credentials/);
  failOptions=false;current=policy('single-user',true,false);await page.locator('#retry-options').click();await code.waitFor({state:'visible'});
  assert.deepEqual(failures,[]);cases.push({browser:browserName,viewport:viewportName,familyNormalized:true,singleCodeOnly:true,passkeyOnlyHiddenForm:true,failedOptionsRetry:true});
 }finally{await browser.close();server.closeAllConnections();await new Promise(r=>server.close(r));}
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4 login bundle with disposable options/verify; no real TOTP, WebAuthn ceremony, family accounts, authenticated session or physical device',cases},null,2));
