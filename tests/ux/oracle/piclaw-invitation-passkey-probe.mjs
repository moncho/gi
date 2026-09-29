// Installed Piclaw 3.2.4 passkey invitation client; no real WebAuthn or server grant.
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
const token='A'.repeat(43),enrolmentToken='B'.repeat(43),account='fixture-account',userId='user-17',userIdEncoded=Buffer.from(userId).toString('base64url');
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true});
 try{
  for(const mode of ['success','page-cancel','outside-blur']){
   const server=createServer((_req,res)=>res.writeHead(404).end());await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
   const origin=`http://127.0.0.1:${server.address().port}`,page=await browser.newPage({viewport,serviceWorkers:'block'}),calls=[],failures=[];
   page.on('pageerror',e=>failures.push(e.message));
   try{
    await page.addInitScript(()=>{
     window.__credentialCalls=[];window.PublicKeyCredential=class {};
     Object.defineProperty(navigator,'credentials',{configurable:true,value:{create:({signal,publicKey})=>{
      window.__credentialCalls.push({signal,publicKey});
      if(window.__fixtureMode!=='success')return new Promise(resolve=>signal.addEventListener('abort',()=>resolve(null),{once:true}));
      return Promise.resolve({id:'fixture-key',rawId:new Uint8Array([1]).buffer,type:'public-key',authenticatorAttachment:'platform',getClientExtensionResults:()=>({}),response:{clientDataJSON:new Uint8Array([2]).buffer,attestationObject:new Uint8Array([3]).buffer,getTransports:()=>['internal']}});
     }}});
    });
    await page.route('**/*',route=>{
     const r=route.request(),u=new URL(r.url());calls.push({method:r.method(),path:u.pathname,url:r.url(),body:r.postDataJSON?.()});
     if(u.origin!==origin)throw Error(`Unexpected origin ${u.origin}`);
     if(u.pathname==='/auth/invitation')return route.fulfill({contentType:'text/html',body:html});
     if(u.pathname==='/static/common/dist/invitation.bundle.js')return route.fulfill({contentType:'application/javascript',body:bundle});
     if(u.pathname==='/static/common/dist/login.bundle.css')return route.fulfill({contentType:'text/css',body:css});
     if(u.pathname==='/auth/invitation/passkey/claim')return route.fulfill({json:{enrolment_token:enrolmentToken,username:account,user_id:userId,expires_at:Date.now()+240000,options:{challenge:'AQ',user:{id:userIdEncoded,name:account,displayName:account},authenticatorSelection:{residentKey:'required',userVerification:'required'},excludeCredentials:[]}}});
     if(u.pathname==='/auth/invitation/passkey/check')return route.fulfill({json:{valid:true}});
     if(u.pathname==='/auth/invitation/passkey/confirm')return route.fulfill({json:{enrolled:true,login_required:true}});
     throw Error(`Unexpected request ${r.method()} ${u.pathname}`);
    });
    await page.goto(`${origin}/auth/invitation?lookup=fixture#method=passkey&token=${token}`);
    const claim=page.locator('#claim-invitation');await claim.waitFor({state:'visible'});
    assert.equal(await page.evaluate(()=>location.pathname+location.search+location.hash),'/auth/invitation');
    assert.equal(calls.filter(c=>c.method==='POST').length,0);
    assert.equal(await page.locator('#passkey-enrolment-account').textContent(),'');
    await claim.click();await page.locator('#passkey-enrolment').waitFor({state:'visible'});
    assert.equal(await page.locator('#passkey-enrolment-account').textContent(),`Account: ${account}`);
    assert.deepEqual(calls.filter(c=>c.path.endsWith('/claim')).map(c=>c.body),[{token}]);
    await page.evaluate(value=>window.__fixtureMode=value,mode);
    if(mode==='outside-blur')await page.evaluate(()=>window.dispatchEvent(new Event('blur')));
    else{
     await page.locator('#create-invitation-passkey').click();
     await page.waitForFunction(()=>window.__credentialCalls.length===1);
     if(mode==='page-cancel')await page.locator('#cancel-invitation-passkey').click();
    }
    if(mode==='success'){
     await page.locator('#invitation-status').getByText('Account setup complete. Sign in to continue.').waitFor();
     const posts=calls.filter(c=>c.method==='POST');
     assert.deepEqual(posts.map(c=>c.path),['/auth/invitation/passkey/claim','/auth/invitation/passkey/check','/auth/invitation/passkey/confirm']);
     assert.deepEqual(posts[1].body,{token,enrolment_token:enrolmentToken});
     assert.equal(posts[2].body.token,token);assert.equal(posts[2].body.enrolment_token,enrolmentToken);
     assert.equal(posts[2].body.credential.id,'fixture-key');
     assert.equal(await page.locator('#invitation-login').isVisible(),true);
     assert.deepEqual(await page.context().cookies(),[]);
    }else{
     await page.locator('#invitation-status').getByText('Passkey setup discarded.').waitFor();
     assert.equal(await page.locator('#passkey-enrolment').isVisible(),false);
     assert.match(await page.locator('#invitation-error').innerText(),/new invitation/i);
     assert.equal(calls.filter(c=>c.path.endsWith('/confirm')).length,0);
     if(mode==='page-cancel')assert.equal(await page.evaluate(()=>window.__credentialCalls[0].signal.aborted),true);
    }
    assert.equal(await page.evaluate(()=>location.pathname+location.search+location.hash),'/auth/invitation');
    assert.deepEqual(failures,[]);
    cases.push({browser:browserName,viewport:viewportName,mode,posts:calls.filter(c=>c.method==='POST').map(c=>c.path)});
   }finally{await page.close();server.closeAllConnections();await new Promise(resolve=>server.close(resolve));}
  }
 }finally{await browser.close();}
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4 invitation bundle with synthetic credential and HTTP; no real WebAuthn ceremony, server binding/one-use verification, authenticated session or physical blur',cases},null,2));
