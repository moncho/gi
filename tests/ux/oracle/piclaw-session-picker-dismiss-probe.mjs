// Mounted installed Piclaw 3.2.4 Classic with a disposable two-chat catalogue.
// Search, Escape and filtered keyboard checks use read-only fixtures, not live backend branch actions or physical input.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const base=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const chats=[{chat_jid:'web:default',agent_name:'Default fixture',is_active:true},{chat_jid:'web:research',agent_name:'Research fixture',is_active:false}];
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...base,sessionId:'web:default',theme:'dark'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 try{
  const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
  await page.route('**/agent/active-chats*',r=>r.fulfill({json:{chats}}));
  await page.route('**/agent/branches*',r=>r.fulfill({json:{chats}}));
  // Desktop prewarm may request the second chat without navigating to it.
  await page.route('**/timeline?*',r=>new URL(r.request().url()).searchParams.get('chat_jid')==='web:research'?r.fulfill({json:{posts:[],has_more:false}}):r.fallback());
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.goto(host.origin);await host.connected();
  const trigger=page.locator('[data-testid="session-switcher"]').first(),popup=page.locator('[data-testid="session-popup"]'),search=popup.locator('.compose-session-search');
  await trigger.waitFor();await trigger.click();await search.waitFor();await search.fill('Research fixture');
  await page.waitForFunction(()=>{const results=document.querySelector('[data-testid="session-popup"] .compose-session-popup-results');return results?.textContent?.includes('web:research')&&!results?.textContent?.includes('web:default');});
  assert.equal(await search.evaluate(el=>el===document.activeElement),true);
  await search.press('Escape');await popup.waitFor({state:'detached'});await page.waitForFunction(()=>document.activeElement?.getAttribute('data-testid')==='session-switcher');
  assert.equal(new URL(page.url()).searchParams.get('chat_jid'),null);
  await trigger.click();await search.waitFor();assert.equal(await search.inputValue(),'');
  await page.waitForFunction(()=>{const results=document.querySelector('[data-testid="session-popup"] .compose-session-popup-results');return results?.textContent?.includes('web:research')&&results?.textContent?.includes('web:default');});
  await search.press('Escape');await popup.waitFor({state:'detached'});await page.waitForFunction(()=>document.activeElement?.getAttribute('data-testid')==='session-switcher');
  assert.equal(new URL(page.url()).searchParams.get('chat_jid'),null);
  await trigger.click();await search.waitFor();await search.fill('Research fixture');
  const option=popup.locator('[role="option"][data-testid="session-item"]');
  await option.waitFor();assert.equal(await option.count(),1);
  for(const key of ['Home','End','ArrowDown']){
   await search.press(key);
   assert.equal(await option.getAttribute('aria-selected'),'true',`${key} must keep sole filtered entry selected`);
   assert.equal(await search.evaluate(el=>el===document.activeElement),true,'search retains keyboard focus');
  }
  // Classic treats Tab as activation when an entry is selected; Gi uses native
  // focus traversal. Switch only into the disposable, read-only second chat.
  // The adapter enforces the selected chat scope on subsequent GET/SSE reads.
  host.assert();state.sessionId='web:research';
  await search.press('Tab');await popup.waitFor({state:'detached'});
  await page.waitForURL(u=>new URL(u).searchParams.get('chat_jid')==='web:research');
  await page.waitForTimeout(200); // Let the old scoped stream report its expected cancellation.
  assert.deepEqual(host.failures.filter(error=>!/^network: .*\/sse\/stream\?chat_jid=web%3Adefault (?:Load request cancelled|net::ERR_ABORTED)$/.test(error)),[]);
  assert.equal(host.calls.some(c=>c.method!=='GET' && /\/agent\/(?:branches|active-chats)/.test(c.path)),false,'keyboard selection must not mutate the catalogue');
  cases.push({browser:browserName,viewport:viewportName,filtered:true,escapeDismissed:true,focusRestored:true,queryReset:true,chatUnchangedBeforeTab:true,filteredKeyboardSelected:true,tabSelectedDisposableResearchChat:true});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Mounted installed Piclaw 3.2.4 Classic bundle with disposable read-only two-chat catalogue. Tab selects the fixture chat URL; no real history, backend branch action, typeahead/IME, or physical-input acceptance.',cases},null,2));
