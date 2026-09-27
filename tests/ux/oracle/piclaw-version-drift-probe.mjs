import fs from 'node:fs/promises';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {createHash} from 'node:crypto';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'../../..');
const oracleRoot=path.resolve(process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current');
const browserName=process.env.ORACLE_BROWSER||'chromium';
if(!['chromium','webkit'].includes(browserName))throw Error('Unsupported browser');
const reference=JSON.parse(await fs.readFile(path.join(root,'tests/ux/oracle/piclaw-3.2.4-reference.json'),'utf8'));
if((await fs.readFile(path.join(oracleRoot,'VERSION'),'utf8')).trim()!=='3.2.4')throw Error('Release version changed');
const map=await fs.readFile(path.join(oracleRoot,'app/runtime/web/static/classic/dist/app.bundle.js.map'));
if(createHash('sha256').update(map).digest('hex')!==reference.map.sha256)throw Error('Shipped Classic map changed');
const state=JSON.parse(await fs.readFile(path.join(root,'tests/ux/fixtures/compose-pixel-state.json'),'utf8'));
state.sessionId='web:default';
const browser=await (browserName==='chromium'?chromium:webkit).launch({headless:true});
const context=await browser.newContext({viewport:{width:1440,height:900},serviceWorkers:'block'});
const page=await context.newPage();const host=await installPixelHost({page,host:'piclaw',root:oracleRoot,state,reference});
const checks={browserName,mapHash:reference.map.sha256,assetVersion:reference.assetVersion,scope:'shipped Classic assets and disposable API/SSE; no Piclaw backend or physical acceptance'};
try{
 const originalHTML=await fs.readFile(path.join(oracleRoot,'app/runtime/web/static/classic/index.html'),'utf8');
 await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:originalHTML.replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1')}));
 await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'web:default',revision:1,models:[],sessions:[]}}));
 await page.goto(host.origin);await page.locator('.compose-box textarea').waitFor();await host.connected();
 await page.waitForTimeout(300);
 const warning=page.locator('.agent-thinking-intent').filter({hasText:'New UI available'});
 if(await warning.count())throw Error('Warning appeared before version change');
 checks.bundle=await page.locator('script[src*="app.bundle.js"]').getAttribute('src');
 if(!checks.bundle?.includes(`v=${reference.assetVersion}`))throw Error('Classic bundle version changed');
 let navigations=0;page.on('framenavigated',f=>{if(f===page.mainFrame())navigations++;});
 const newVersion=reference.assetVersion+'-next';
 // Confirm that the injected version event reaches the shipped SSE dispatcher.
 await page.evaluate(()=>{window.__uxSseConnected=0;window.addEventListener('piclaw:sse-connected',()=>window.__uxSseConnected++);});
 await host.connected({app_asset_version:newVersion});
 await page.waitForTimeout(200);
 const delivered=await page.evaluate(()=>window.__uxSseConnected);
 if(delivered<1)throw Error(`SSE connected event did not reach shipped dispatcher (${delivered})`);
 await warning.waitFor({timeout:2500});
 const text=await warning.innerText();
 if(!text.includes('Reload this page to apply the latest interface update.'))throw Error(`Missing manual reload text: ${text}`);
 await host.connected({app_asset_version:newVersion});
 await page.waitForTimeout(1000);
 const warnings=await warning.count();
 if(warnings!==1||navigations!==0)throw Error(`Repeated warning or automatic navigation: ${warnings}/${navigations}`);
 host.assert();checks.result={warning:text,repeatCount:warnings,navigations,fixture:'confirmed SSE event delivery, pinned shipped assets and stable pin-sync GET'};
 console.log(JSON.stringify(checks,null,2));
}finally{await host.dispose();await browser.close();}
