// Read-only capture of the isolated manual validation instance; no production target.
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {chromium,webkit} from 'playwright';
const browserName=process.env.ORACLE_BROWSER||'chromium';
assert(['chromium','webkit'].includes(browserName));
const origin='http://192.168.1.153:19151';
const session='session_1790541984277283331';
const output=path.resolve(`test-results/ux-oracle/chat-baseline/${browserName}/run-${Date.now()}`);
await fs.mkdir(output,{recursive:true});
const password=(await fs.readFile('/workspace/tmp/gi-autosave-validation/lan-password','utf8')).trim();
const browser=await ({chromium,webkit}[browserName]).launch({headless:true});
const page=await browser.newPage({viewport:{width:1213,height:688},httpCredentials:{username:'validation',password},serviceWorkers:'block'});
const blocked=[],failures=[];
await page.addInitScript(id=>localStorage.setItem('gi_session_id',id),session);
await page.route('**/*',async r=>{const q=r.request();if(new URL(q.url()).origin!==origin||q.method()!=='GET'){blocked.push({method:q.method(),path:new URL(q.url()).pathname});return r.abort();}return r.continue();});
page.on('pageerror',e=>failures.push(e.message));
const report={origin,session,browser:browserName,scope:'Read-only DOM and GET capture of disposable manual-validation instance. All mutating and off-origin requests blocked.',blocked,failures};
try{
 await page.goto(origin);await page.locator('.compose-box textarea').waitFor();await page.locator('.post').first().waitFor();
 await page.waitForFunction(()=>document.querySelectorAll('.post').length>=5);
 report.messages=await page.evaluate(async id=>(await(await fetch(`/api/sessions/${id}/messages`)).json()).messages.map(m=>({id:m.id,role:m.role,content:m.content,payload:m.payload})),session);
 report.posts=await page.locator('.post').evaluateAll(es=>es.map(e=>({id:e.id,author:e.querySelector('.post-author')?.textContent,text:e.querySelector('.post-content')?.textContent,html:e.innerHTML})));
 report.status=await page.locator('.agent-status-panel,.gi-tool-activity').evaluateAll(es=>es.map(e=>({text:e.textContent,html:e.outerHTML})));
 const byId=new Map(report.posts.map(p=>[p.id.replace(/^post-/,''),p]));
 report.defects={nonUserRowsDisplayedAsUser:report.messages.filter(m=>m.role!=='user'&&byId.get(m.id)?.author==='Validation User').map(m=>({id:m.id,role:m.role})),internalToolCallMarkers:report.posts.filter(p=>p.text?.includes('[tool_call:')).map(p=>p.id),rawToolResultsInTimeline:report.messages.filter(m=>m.role==='tool_result'&&byId.has(m.id)).map(m=>m.id),terminalFooter:report.status.filter(p=>p.text?.includes('Completed:')).map(p=>p.text)};
 assert(report.defects.nonUserRowsDisplayedAsUser.length>=3,'Reported attribution failure no longer reproduces; inspect before changing baseline');
 assert(report.defects.internalToolCallMarkers.length>0);
 assert(report.defects.rawToolResultsInTimeline.length>0);
 assert(report.defects.terminalFooter.length>0);
 await page.screenshot({path:path.join(output,'gi-baseline.png'),fullPage:true});
 report.result='reported defects reproduced';
}catch(e){report.result='capture failed';report.error=String(e);process.exitCode=1;}
finally{await fs.writeFile(path.join(output,'evidence.json'),JSON.stringify(report,null,2));await browser.close();}
console.log(JSON.stringify({result:report.result,error:report.error,defects:report.defects,blocked,failures,output},null,2));
