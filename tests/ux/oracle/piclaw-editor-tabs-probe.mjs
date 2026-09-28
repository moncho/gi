// Installed Piclaw 3.2.4 Classic editor tabs with disposable workspace files.
// No live file writes: the only edit is browser-local and the dirty close is cancelled.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {chromium,webkit} from 'playwright';
import {installPixelHost} from '../support/pixel-adapter.mjs';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const reference=JSON.parse(await fs.readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const fixture=JSON.parse(await fs.readFile(new URL('../fixtures/compose-pixel-state.json',import.meta.url),'utf8'));
const html=(await fs.readFile(path.join(root,'app/runtime/web/static/classic/index.html'),'utf8')).replaceAll('__PICLAW_SANITIZE_SVG_FENCES_FLAG__','1');
const files={'oracle-a.txt':'First editor file','oracle-b.txt':'Second editor file'};
const cases=[];
for(const [browserName,type] of Object.entries({chromium,webkit}))for(const [viewportName,viewport] of Object.entries({phone:{width:390,height:844},tablet:{width:820,height:1180},desktop:{width:1440,height:900}})){
 const browser=await type.launch({headless:true}),page=await browser.newPage({viewport,serviceWorkers:'block'});
 const state={...fixture,theme:'light',sessionId:'web:default'};
 const host=await installPixelHost({page,host:'piclaw',root,state,reference,allowPresenceBeacon:true});
 const fileReads=[],saveRequests=[];
 try{
  await page.route(host.origin+'/',r=>r.fulfill({contentType:'text/html',body:html}));
  await page.route('**/agent/picker-pins',r=>r.fulfill({json:{scope:'editor-tabs',revision:0,models:[],sessions:[]}}));
  await page.route('**/agent/settings/quick-actions',r=>r.fulfill({json:{ok:true,settings:{workspaceCommands:null,slashCommands:null}}}));
  await page.route('**/workspace/tree?*',r=>r.fulfill({json:{root:{name:'fixture',path:'.',type:'dir',children:Object.keys(files).map(file=>({name:file,path:file,type:'file',size:files[file].length}))},truncated:false}}));
  await page.route('**/workspace/file?*',r=>{const file=new URL(r.request().url()).searchParams.get('path');fileReads.push(file);return r.fulfill({json:{path:file,name:file,kind:'text',content_type:'text/plain',size:files[file]?.length||0,mtime:state.now,text:files[file],truncated:false}})});
  await page.route('**/workspace/raw?*',r=>{const file=new URL(r.request().url()).searchParams.get('path');return r.fulfill({contentType:'text/plain',body:files[file]||''})});
  await page.route('**/workspace/branch?*',r=>r.fulfill({json:{branch:'fixture',path:new URL(r.request().url()).searchParams.get('path')}}));
  page.on('request',r=>{if(r.method()!=='GET'&&r.url().includes('/workspace/')&&!r.url().endsWith('/workspace/visibility'))saveRequests.push(r.url())});
  await page.goto(host.origin);await host.connected();
  const input=page.locator('.compose-box textarea');await input.fill('editor draft kept');await input.blur();
  await page.locator('.timeline').click({position:{x:150,y:90}});await page.keyboard.press('w');
  const palette=page.locator('.timeline-quick-actions');await palette.waitFor();
  await palette.locator('.timeline-quick-actions-input').fill('Show workspace');
  await palette.locator('.timeline-quick-actions-item-workspace').filter({hasText:'Show workspace'}).first().click();
  const tree=page.locator('.workspace-sidebar .workspace-tree-list');await tree.waitFor();
  const open=async file=>{
   await tree.locator(`.workspace-row[data-path="${file}"]`).click();
   await page.locator('.workspace-preview-actions .workspace-edit').click();
   await page.locator('.editor-pane-container .cm-editor').waitFor();
  };
  await open('oracle-a.txt');await open('oracle-b.txt');
  const tabs=page.locator('.tab-strip .tab-item'),first=tabs.filter({hasText:'oracle-a.txt'}),second=tabs.filter({hasText:'oracle-b.txt'});
  await first.waitFor();await second.waitFor();assert.equal(await tabs.count(),2);
  const backdrop=page.locator('.workspace-drawer-backdrop');
  if(await backdrop.isVisible())await backdrop.click({position:{x:viewport.width-24,y:Math.floor(viewport.height/2)}});
  const editor=page.locator('.editor-pane-container .cm-editor');
  await second.click();await editor.locator('.cm-content').getByText('Second editor file').waitFor();
  await first.dispatchEvent('mousedown',{button:0});
  await first.waitFor({state:'visible'});assert((await first.getAttribute('class')).includes('active'));
  await editor.locator('.cm-content').getByText('First editor file').waitFor();
  await editor.click();await page.keyboard.type(' local edit');
  await first.locator('.tab-close').waitFor();
  await page.waitForFunction(()=>document.querySelector('.tab-strip .tab-item.active.dirty'));
  const before=await editor.locator('.cm-content').innerText();
  let promptText='';page.once('dialog',async dialog=>{promptText=dialog.message();await dialog.dismiss();});
  await first.locator('.tab-close').click();
  assert(promptText.includes('unsaved changes'));
  assert.equal(await tabs.count(),2);assert((await first.getAttribute('class')).includes('dirty'));
  assert.equal(await editor.locator('.cm-content').innerText(),before);
  assert.deepEqual(saveRequests,[]);assert.equal(await input.inputValue(),'editor draft kept');host.assert();
  cases.push({browser:browserName,viewport:viewportName,tabCount:2,dirtyClose:'dismissed',pointerDownActivation:true,fileReads,result:'pass'});
 }finally{await host.dispose();await browser.close();}
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4 shipped editor, disposable file reads, browser-local unsaved edit; no live writes/provider/physical pointer acceptance',cases},null,2));
