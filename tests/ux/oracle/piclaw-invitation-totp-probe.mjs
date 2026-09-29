// Installed Piclaw 3.2.4 invitation page; synthetic one-use TOTP ceremony.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {createServer} from 'node:http';
import {chromium,webkit} from 'playwright';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const html=(await fs.readFile(path.join(root,'app/runtime/web/static/invitation.html'),'utf8')).replaceAll('__LOGIN_ASSET_VERSION__','fixture');
const bundle=await fs.readFile(path.join(root,'app/runtime/web/static/common/dist/invitation.bundle.js'));
const css=await fs.readFile(path.join(root,'app/runtime/web/static/common/dist/login.bundle.css'));
const token='A'.repeat(43),enrolmentToken='B'.repeat(43),secret='C'.repeat(32);
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const server=createServer((_req,res)=>res.writeHead(404).end());await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 const origin=`http://127.0.0.1:${server.address().port}`,browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'}),calls=[],failures=[];
 page.on('pageerror',e=>failures.push(e.message));
 try{
  await page.route('**/*',route=>{
   const r=route.request(),u=new URL(r.url());calls.push({method:r.method(),path:u.pathname,url:r.url(),body:r.postDataJSON?.()});
   if(u.origin!==origin)throw Error(`Unexpected origin ${u.origin}`);
   if(u.pathname==='/auth/invitation')return route.fulfill({contentType:'text/html',body:html});
   if(u.pathname==='/static/common/dist/invitation.bundle.js')return route.fulfill({contentType:'application/javascript',body:bundle});
   if(u.pathname==='/static/common/dist/login.bundle.css')return route.fulfill({contentType:'text/css',body:css});
   if(u.pathname==='/auth/invitation/claim')return route.fulfill({json:{enrolment_token:enrolmentToken,secret,username:'fixture-account',expires_at:Date.now()+240000,qr_data_url:'data:image/svg+xml;base64,PHN2Zy8+'}});
   if(u.pathname==='/auth/invitation/confirm')return route.fulfill({json:{enrolled:true,login_required:true}});
   throw Error(`Unexpected request ${r.method()} ${u.pathname}`);
  });
  await page.goto(`${origin}/auth/invitation?lookup=fixture#method=totp&token=${token}`);
  const claim=page.locator('#claim-invitation');await claim.waitFor({state:'visible'});
  assert.equal(await page.evaluate(()=>location.pathname+location.search+location.hash),'/auth/invitation');
  assert.equal(calls.filter(c=>c.path.startsWith('/auth/invitation/')&&c.method==='POST').length,0);
  assert.equal(calls.filter(c=>c.url.includes(token)&&c.path!=='/auth/invitation').length,0);
  await claim.click();await page.locator('#enrolment').waitFor({state:'visible'});
  assert.deepEqual(calls.filter(c=>c.path==='/auth/invitation/claim').map(c=>[c.method,c.body]),[['POST',{token}]]);
  assert.equal(await page.locator('#enrolment-secret').textContent(),secret);
  await page.locator('#confirmation-code').fill('123456');await page.locator('#confirm-invitation').click();
  await page.locator('#invitation-status').getByText('Account setup complete. Sign in to continue.').waitFor();
  assert.deepEqual(calls.filter(c=>c.path==='/auth/invitation/confirm').map(c=>[c.method,c.body]),[['POST',{token,enrolment_token:enrolmentToken,code:'123456'}]]);
  assert.equal(await page.locator('#enrolment-secret').textContent(),'');
  assert.equal(await page.locator('#enrolment-qr').getAttribute('src'),null);
  assert.equal(await page.locator('#confirmation-code').inputValue(),'');
  assert.equal(await page.locator('#enrolment').isVisible(),false);
  assert.equal(await page.locator('#invitation-login').isVisible(),true);
  assert.equal(await page.evaluate(()=>location.pathname+location.search+location.hash),'/auth/invitation');
  assert.deepEqual(failures,[]);
  cases.push({browser:browserName,viewport:viewportName,urlStripped:true,claimOnlyOnClick:true,secretCleared:true,claimPosts:1,confirmPosts:1});
 }finally{await browser.close();server.closeAllConnections();await new Promise(resolve=>server.close(resolve));}
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4 invitation bundle with disposable claim/confirm HTTP; no real one-use token, TOTP verification, account, session, physical input or persistence',cases},null,2));
