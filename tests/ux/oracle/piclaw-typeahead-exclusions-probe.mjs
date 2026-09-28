// Shipped Piclaw 3.2.4 browser assets with isolated keyboard targets.
// DOM-only excluded classes do not prove real editor/panel integration.
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(root+'/VERSION','utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const fixture=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const output=path.resolve('test-results/ux-oracle/typeahead-exclusions');await fs.mkdir(output,{recursive:true});const cases=[];
const targets=[
 ['input','<input>'],['textarea','<textarea></textarea>'],['select','<select><option>one</option></select>'],
 ['editable','<div contenteditable="true"></div>'],['composer','<div class="compose-box"></div>'],
 ['editor','<div class="editor-pane-container"></div>'],['workspace','<div class="workspace-sidebar"></div>'],
 ['dialog','<div class="settings-dialog"></div>'],
 ['popup','<div class="compose-model-popup"></div>'],['dock','<div class="dock-panel"></div>'],
];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,theme:'light',sessionId:'web:default'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 try{
  const html=(await fs.readFile(root+'/app/runtime/web/static/classic/index.html','utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'typeahead',revision:0,models:[],sessions:[]}}));
  await page.route('**/agent/settings/quick-actions',r=>r.fulfill({json:{ok:true,settings:{workspaceCommands:null,slashCommands:null}}}));
  await page.route('**/workspace/tree?*',r=>r.fulfill({json:{root:{name:'fixture',path:'.',type:'dir',children:[{name:'oracle-note.txt',path:'oracle-note.txt',type:'file',size:11}]},truncated:false}}));
  await page.route('**/workspace/file?*',r=>r.fulfill({json:{path:'oracle-note.txt',name:'oracle-note.txt',kind:'text',content_type:'text/plain',size:11,mtime:state.now,text:'oracle text',truncated:false}}));
  await page.route('**/workspace/raw?*',r=>r.fulfill({contentType:'text/plain',body:'oracle text'}));
  await page.route('**/workspace/branch?*',r=>r.fulfill({json:{branch:'fixture',path:'oracle-note.txt'}}));
  await page.goto(host.origin);await host.connected();const palette=page.locator('.timeline-quick-actions');
  const input=page.locator('.compose-box textarea');await input.fill('kept draft');
  await page.locator('.timeline').click({position:{x:150,y:90}});await page.keyboard.press('q');await palette.waitFor();
  const query=palette.locator('.timeline-quick-actions-input');assert.equal(await query.inputValue(),'q');
  // The shipped popup binds the open-state capture listener in a passive effect.
  await page.waitForFunction(()=>document.activeElement===document.querySelector('.timeline-quick-actions-input'));
  await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
  await query.press('Escape');await palette.waitFor({state:'hidden'});
  await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
  await input.focus();await input.press('End');await input.press('z');assert.equal(await input.inputValue(),'kept draftz');assert.equal(await palette.count(),0);
  const live=[];
  // Open the shipped session picker and verify its native search owns typing.
  const sessionTrigger=page.getByTestId('session-switcher');
  await sessionTrigger.click();
  const sessionSearch=page.getByRole('searchbox',{name:/Search sessions/});
  await sessionSearch.waitFor();await sessionSearch.fill('q');
  assert.equal(await palette.count(),0);live.push('session search');
  await sessionSearch.press('Escape');
  await page.locator('.compose-session-popup').waitFor({state:'hidden'});
  await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
  // Exercise the shipped workspace tree, not only an injected sidebar class.
  await page.locator('.timeline').click({position:{x:150,y:90}});
  await page.keyboard.press('w');await palette.waitFor();
  await palette.locator('.timeline-quick-actions-input').fill('Show workspace');
  await palette.locator('.timeline-quick-actions-item-workspace').filter({hasText:'Show workspace'}).first().click();
  await page.waitForFunction(()=>document.querySelector('.app-shell')?.classList.contains('workspace-collapsed')===false);
  const tree=page.locator('.workspace-sidebar .workspace-tree-list');await tree.waitFor();
  await tree.focus();assert(await tree.evaluate(el=>document.activeElement===el));
  await tree.press('q');assert.equal(await palette.count(),0);live.push('workspace tree');
  await tree.locator('.workspace-row[data-path="oracle-note.txt"]').click();
  await page.locator('.workspace-preview-actions .workspace-edit').click();
  const editor=page.locator('.editor-pane-container .cm-editor');await editor.waitFor();
  // Mobile workspace drawer overlays the pane. Close it through its shipped
  // backdrop before focusing CodeMirror; never force-click through it.
  const backdrop=page.locator('.workspace-drawer-backdrop');
  if(await backdrop.isVisible())await backdrop.click({position:{x:viewport.width-24,y:Math.floor(viewport.height/2)}});
  await editor.click();
  const editorFocus=await page.evaluate(()=>({inside:!!document.activeElement?.closest?.('.cm-editor'),editable:document.activeElement?.getAttribute('contenteditable')}));
  assert.equal(editorFocus.inside,true);
  await page.keyboard.press('q');assert.equal(await palette.count(),0);
  assert.equal((await editor.locator('.cm-content').innerText()).replace('q',''),'oracle text');
  live.push('CodeMirror editor');
  const excluded=[];
  for(const [name,markup] of targets){
   await page.evaluate(markup=>{
    const wrap=document.createElement('div');wrap.id='oracle-typeahead-target';wrap.innerHTML=markup;document.querySelector('.timeline').append(wrap);
    wrap.firstElementChild.dispatchEvent(new KeyboardEvent('keydown',{key:'q',bubbles:true,cancelable:true}));
    wrap.remove();
   },markup);
   await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
   const count=await palette.count();excluded.push({name,opened:Boolean(count)});
   assert.equal(count,0,`Quick Actions opened from ${name}`);
  }
  assert.equal(await input.inputValue(),'kept draftz');host.assert();
  cases.push({browser:browserName,viewport:viewportName,result:'pass',openedFromTimeline:true,editorText:await input.inputValue(),live,excluded});
 }finally{await fs.writeFile(path.join(output,'evidence.json'),JSON.stringify({scope:'Shipped Piclaw3.2.4 assets and isolated browser keyboard fixture. Real composer/session search/workspace tree/CodeMirror editor plus synthetic overlay selectors; no provider or physical keyboard acceptance.',cases},null,2));await host.dispose();await browser.close();}
}
console.log(JSON.stringify({output,cases}));
