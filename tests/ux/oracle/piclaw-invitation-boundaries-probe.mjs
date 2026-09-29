// Installed Piclaw 3.2.4 invitation: recovery-only and rejected/invalid display.
// Disposable HTTP stubs; server invitation validity and one-use enforcement are untested.
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
const token='A'.repeat(43),enrolmentToken='B'.repeat(43),secret='C'.repeat(32),cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true});
 try{
  for(const mode of ['recovery-only','invalid-fragment','rejected-claim']){
   const server=createServer((_req,res)=>res.writeHead(404).end());await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
   const origin=`http://127.0.0.1:${server.address().port}`,page=await browser.newPage({viewport,serviceWorkers:'block'}),calls=[],failures=[];
   page.on('pageerror',e=>failures.push(e.message));
   try{
    await page.route('**/*',route=>{
     const r=route.request(),u=new URL(r.url());calls.push({method:r.method(),path:u.pathname,body:r.postDataJSON?.()});
     if(u.origin!==origin)throw Error(`Unexpected origin ${u.origin}`);
     if(u.pathname==='/auth/invitation')return route.fulfill({contentType:'text/html',body:html});
     if(u.pathname==='/static/common/dist/invitation.bundle.js')return route.fulfill({contentType:'application/javascript',body:bundle});
     if(u.pathname==='/static/common/dist/login.bundle.css')return route.fulfill({contentType:'text/css',body:css});
     if(u.pathname==='/auth/invitation/claim')return mode==='rejected-claim'?route.fulfill({status:403,json:{error:'Rejected fixture claim'}}):route.fulfill({json:{enrolment_token:enrolmentToken,secret,username:'fixture-account',expires_at:Date.now()+240000,qr_data_url:'data:image/svg+xml;base64,PHN2Zy8+'}});
     if(u.pathname==='/auth/invitation/confirm')return route.fulfill({json:{enrolled:true,login_required:true,recovery_only:true}});
     throw Error(`Unexpected request ${r.method()} ${u.pathname}`);
    });
    const fragment=mode==='invalid-fragment'?'#method=totp&token=bad':`#method=totp&token=${token}`;
    await page.goto(`${origin}/auth/invitation${fragment}`);
    assert.equal(await page.evaluate(()=>location.pathname+location.search+location.hash),'/auth/invitation');
    const claim=page.locator('#claim-invitation');
    if(mode==='invalid-fragment'){
     await page.locator('#invitation-status').getByText('No valid invitation was provided.').waitFor();
     assert.equal(await claim.isVisible(),false);
     assert.match(await page.locator('#invitation-error').innerText(),/administrator/i);
     assert.equal(calls.filter(c=>c.path==='/auth/invitation/claim').length,0);
    }else{
     await claim.waitFor({state:'visible'});assert.equal(calls.filter(c=>c.path==='/auth/invitation/claim').length,0);
     await claim.click();
     if(mode==='rejected-claim'){
      await page.locator('#invitation-status').getByText('Setup could not begin.').waitFor();
      assert.match(await page.locator('#invitation-error').innerText(),/administrator.*new link.*do not retry/i);
      assert.equal(await claim.isVisible(),false);
      await page.waitForTimeout(350);
      assert.equal(calls.filter(c=>c.path==='/auth/invitation/claim').length,1);
     }else{
      await page.locator('#enrolment').waitFor({state:'visible'});
      await page.locator('#confirmation-code').fill('123456');await page.locator('#confirm-invitation').click();
      await page.locator('#invitation-status').getByText(/Account recovery complete/).waitFor();
      assert.equal(await page.locator('#invitation-login').isVisible(),false);
      assert.equal(await page.locator('#enrolment-secret').textContent(),'');
      assert.equal(await page.locator('#enrolment-qr').getAttribute('src'),null);
      assert.equal(await page.locator('#confirmation-code').inputValue(),'');
      assert.equal(calls.filter(c=>c.path==='/auth/invitation/confirm').length,1);
     }
    }
    assert.equal(await page.locator('#enrolment').isVisible(),false);
    assert.deepEqual(failures,[]);cases.push({browser:browserName,viewport:viewportName,mode,claimPosts:calls.filter(c=>c.path==='/auth/invitation/claim').length,confirmPosts:calls.filter(c=>c.path==='/auth/invitation/confirm').length});
   }finally{await page.close();server.closeAllConnections();await new Promise(resolve=>server.close(resolve));}
  }
 }finally{await browser.close();}
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4 invitation bundle, disposable HTTP success/rejection; no server one-use enforcement, real TOTP/recovery, account/session, reload or physical input',cases},null,2));
