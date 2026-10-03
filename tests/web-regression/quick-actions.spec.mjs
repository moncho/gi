import {test,expect} from '@playwright/test';

import {attachGiDeviation} from '../ux/support/gi-deviations.mjs';
const inputName='Message (Enter to send, Shift+Enter for newline)...';
async function fixture(page,request,info){
 const key=`quick-${info.project.name}-${Date.now()}`;
 const main=await(await request.post('/api/sessions',{data:{agent_id:key,title:key}})).json();
 const child=await(await request.post('/api/sessions',{data:{agent_id:key+'-child',title:key+'-child'}})).json();
 await page.addInitScript(id=>localStorage.setItem('gi_session_id',id),main.id);await page.goto('/');
 const input=page.getByRole('textbox',{name:inputName,exact:true});await expect(input).toBeVisible();await input.fill('untouched draft');
 await page.waitForTimeout(150); // Pinned component installs capture listeners in a passive effect.
 const palette=page.locator('.timeline-quick-actions'),query=page.locator('.timeline-quick-actions-input');
 const open=async key=>{await input.blur();await page.locator('.timeline').click({position:{x:160,y:180}});await page.keyboard.type(key);await expect(palette).toBeVisible();await expect(query).toBeFocused();await page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));};
 const turns=async()=>((await(await request.get(`/api/sessions/${main.id}/turns`)).json()).turns??[]);
 return {main,child,input,palette,query,open,turns};
}

test('Gi Quick Actions insert the Piclaw exact command while retaining media and focus',async({page,request},info)=>{
 await attachGiDeviation(info,'@gi-ux-001');const {main,child,input,palette,query,open,turns}=await fixture(page,request,info);
 await page.locator('.compose-box input[type=file]').setInputFiles({name:'kept.txt',mimeType:'text/plain',buffer:Buffer.from('kept bytes')});
 await open('m');await query.fill('/model');await expect(palette.locator('.timeline-quick-actions-item-slash')).toHaveCount(1);await expect(page.locator('.timeline-quick-actions-item.active .timeline-quick-actions-item-title')).toHaveText('/model');await page.waitForTimeout(150);await query.press('Enter');
 await expect(palette).toHaveCount(0);await expect(input).toHaveValue('/model');await expect(input).toBeFocused();expect(await input.evaluate(el=>[el.selectionStart,el.selectionEnd])).toEqual([6,6]);
 await expect(page.locator('.compose-file-pill').filter({hasText:'kept.txt'})).toBeVisible();expect(await turns()).toEqual([]);
 await page.screenshot({path:info.outputPath('quick-command-prefill.png')});
 await input.fill('newer text');await page.reload();await expect(input).toHaveValue('newer text');
 await open('c');await query.fill('/compact');await palette.locator('.timeline-quick-actions-item-slash').click();await expect(input).toHaveValue('/compact');await expect(input).toBeFocused();expect(await turns()).toEqual([]);
 await page.reload();await expect(input).toHaveValue('/compact');expect(await turns()).toEqual([]);
});

test('Gi Quick Actions exclusions preserve native inputs and interactive controls',async({page,request},info)=>{
 const {input,palette,turns}=await fixture(page,request,info);
 await input.press('End');await input.press('q');await expect(input).toHaveValue('untouched draftq');await expect(palette).toHaveCount(0);
 await page.getByTestId('hamburger').focus();await page.keyboard.press('q');await expect(palette).toHaveCount(0);
 await page.getByTestId('hamburger').click();await page.getByRole('menuitem',{name:'Show workspace',exact:true}).focus();await page.keyboard.press('q');await expect(palette).toHaveCount(0);
 await page.getByRole('menuitem',{name:'Show workspace',exact:true}).click();await page.locator('.workspace-sidebar').evaluate(el=>el.dispatchEvent(new KeyboardEvent('keydown',{key:'q',bubbles:true})));await expect(palette).toHaveCount(0);
 await page.getByTestId('hamburger').click();await page.getByRole('menuitem',{name:'Hide workspace',exact:true}).click();
 await page.getByRole('button',{name:/Manage sessions for/}).last().click();const search=page.getByRole('searchbox',{name:'Search sessions'});await search.fill('query');await expect(palette).toHaveCount(0);await search.press('Escape');
 // DOM-level guard fixtures cover selector classes absent from current Gi UI;
 // they earn no frozen original-004 credit for unimplemented Monaco/panels.
 for(const html of ['<input>','<textarea></textarea>','<select><option>x</option></select>','<div contenteditable="true"></div>','<a href="#">link</a>','<div role="button" tabindex="0">button</div>','<div class="monaco-editor" tabindex="0"></div>','<div class="terminal-pane" tabindex="0"></div>','<div class="post-reply" tabindex="0"></div>']){
  await page.evaluate(html=>{const wrap=document.createElement('div');wrap.id='exclusion-test';wrap.innerHTML=html;document.querySelector('.timeline').append(wrap);wrap.firstElementChild.dispatchEvent(new KeyboardEvent('keydown',{key:'m',bubbles:true}));},html);
  await expect(palette).toHaveCount(0);await page.evaluate(()=>document.getElementById('exclusion-test').remove());
 }
 expect(await turns()).toEqual([]);
});

test('Gi read-only workspace tab owns printable keys without opening Quick Actions',async({page,request},info)=>{
 const path=`quick-preview-${info.project.name}-${Date.now()}.md`;
 const written=await request.post('/api/tools/execute',{data:{tool:'write',input:{path,content:'# Native read-only preview\\n\\nText remains visible.'}}});
 expect(written.ok()).toBe(true);expect((await written.json()).error).toBeFalsy();
 const {main,input,palette,turns}=await fixture(page,request,info);
 await page.getByTestId('hamburger').click();await page.getByRole('menuitem',{name:'Show workspace',exact:true}).click();
 const tree=page.locator('.workspace-sidebar .workspace-tree-list');await expect(tree).toBeVisible();
 await tree.focus();await tree.press('q');await expect(palette).toHaveCount(0);
 await page.locator(`.workspace-row[data-path="${path}"] .workspace-label-text`).click();
 await page.getByRole('button',{name:'Open read-only tab',exact:true}).click();
 const preview=page.getByRole('region',{name:`Read-only preview: ${path}`});await expect(preview).toBeVisible();
 if(await page.locator('.workspace-toggle-tab.open').count())await page.locator('.workspace-toggle-tab.open').click();
 const tab=page.locator('.gi-readonly-tabs .tab-item.active');await expect(tab).toBeVisible();
 await tab.focus();await expect(tab).toBeFocused();await tab.press('q');await expect(palette).toHaveCount(0);
 await expect(preview.getByRole('heading',{name:'Native read-only preview'})).toBeVisible();
 await page.getByRole('button',{name:'Return to conversation',exact:true}).click();
 await expect(input).toHaveValue('untouched draft');expect(await turns()).toEqual([]);
 expect(await page.evaluate(()=>localStorage.getItem('gi_session_id'))).toBe(main.id);
});

test('Gi Quick Actions switches native sessions and gates unsupported actions',async({page,request},info)=>{
 const {main,child,input,palette,query,open,turns}=await fixture(page,request,info);
 await open('m');await query.fill('');
 await expect(page.locator('.timeline-quick-actions-item-workspace .timeline-quick-actions-item-title')).toHaveText(['Show workspace','Open explorer']);
 await expect(page.locator('.timeline-quick-actions-item-slash .timeline-quick-actions-item-title')).toHaveText(['/model','/compact','/mcp']);
 await page.screenshot({path:info.outputPath('quick-actions-groups.png')});
 await query.fill('Open explorer');await expect(page.locator('.timeline-quick-actions-item.active .timeline-quick-actions-item-title')).toHaveText('Open explorer');await page.waitForTimeout(150);await query.press('Enter');await expect(page.locator('.workspace-sidebar')).toBeVisible();await expect(palette).toHaveCount(0);
 // The supplied palette rebinds its capture listener in a passive effect after
 // dismissal. Wait for that effect before the next distinct user interaction.
 await page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
 await input.blur();await page.locator('.timeline').evaluate(el=>el.dispatchEvent(new KeyboardEvent('keydown',{key:'m',bubbles:true,cancelable:true})));await expect(palette).toBeVisible();await query.fill('Hide workspace');await expect(page.locator('.timeline-quick-actions-item.active .timeline-quick-actions-item-title')).toHaveText('Hide workspace');await page.waitForTimeout(150);await query.press('Enter');await expect(page.locator('.app-shell')).not.toHaveClass(/workspace-visible/);await expect(page.locator('.workspace-sidebar')).not.toHaveClass(/visible/);
 await open('m');await query.fill(child.title);await expect(palette.locator('.timeline-quick-actions-item-agent')).toHaveCount(1);await page.waitForTimeout(150);await query.press('Enter');
 await expect.poll(()=>page.evaluate(()=>localStorage.getItem('gi_session_id'))).toBe(child.id);await expect(input).toHaveValue('');await input.fill('child unsent');
 await open('m');await query.fill(main.title);await palette.locator('.timeline-quick-actions-item-agent').filter({has:page.locator('.timeline-quick-actions-item-title',{hasText:new RegExp('^@'+main.title+'$')})}).click();await expect.poll(()=>page.evaluate(()=>localStorage.getItem('gi_session_id'))).toBe(main.id);await expect(input).toHaveValue('untouched draft');
 expect(await turns()).toEqual([]);
});

test('Gi Quick Actions capability failure stays conservative and delayed responses cannot reopen another session',async({page,request},info)=>{
 const {main,child,input,palette,query,open,turns}=await fixture(page,request,info);
 let release,held=false,deny=false,inFlight=0,denied=0;const gate=new Promise(r=>release=r);
 // Keep one route registered across reload. Replacing routes while held
 // callbacks are draining can let a reload's requests bypass the new handler.
 await page.route('**/api/quick-actions',async route=>{
  if(deny){denied++;await route.abort('failed');return;}
  inFlight++;
  try{const response=await route.fetch();held=true;await gate;await route.fulfill({response});}finally{inFlight--;}
 });
 try{
  await page.getByRole('button',{name:/Manage sessions for/}).last().click();await page.locator(`[data-session-jid="gi:${child.id}"]`).getByRole('menuitem').click();await expect.poll(()=>held).toBe(true);
  await page.getByRole('button',{name:/Manage sessions for/}).last().click();await page.locator(`[data-session-jid="gi:${main.id}"]`).getByRole('menuitem').click();release();await expect(input).toHaveValue('untouched draft');await expect(palette).toHaveCount(0);
  await expect.poll(()=>inFlight).toBe(0);deny=true;
  await page.reload();await expect.poll(()=>denied).toBeGreaterThanOrEqual(2);await expect(input).toHaveValue('untouched draft');await open('m');await query.fill('');
  await expect(page.locator('.timeline-quick-actions-item-workspace')).toHaveCount(0);await expect(page.locator('.timeline-quick-actions-item-slash')).toHaveCount(0);await expect(page.locator('.timeline-quick-actions-item-agent')).not.toHaveCount(0);
  await query.press('Escape');expect(await turns()).toEqual([]);
 }finally{release()}
});

test('Gi Quick Actions linked agent tab selects the native destination without moving the original draft',async({page,request,context},info)=>{
 const {main,child,input,palette,query,open,turns}=await fixture(page,request,info);
 await open('m');await query.fill(child.title);await expect(palette.locator('.timeline-quick-actions-item-agent')).toHaveCount(1);await page.waitForTimeout(150);
 const newPage=context.waitForEvent('page');await query.press('Alt+Enter');const linked=await newPage;
 try{await expect(linked.getByRole('textbox',{name:inputName,exact:true})).toBeVisible();expect(new URL(linked.url()).searchParams.get('chat_jid')).toBe(`gi:${child.id}`);await expect.poll(()=>linked.evaluate(()=>localStorage.getItem('gi_session_id'))).toBe(child.id);
  await expect(input).toHaveValue('untouched draft');expect(await turns()).toEqual([]);
 }finally{await linked.close()}
});

async function sharedTypingFixture(page,request,info,id){
 const f=await fixture(page,request,info);
 const posted=await request.post(`/api/sessions/${f.main.id}/prompt`,{data:{prompt:`native key guard history ${id}`,model:'test-model'}});expect(posted.status()).toBe(202);const turn=(await posted.json()).turn_id;
 await expect.poll(async()=>(await f.turns()).find(t=>t.id===turn)?.status).toBe('completed');
 await page.reload();await expect(f.input).toHaveValue('untouched draft');
 await page.locator('.compose-box input[type=file]').setInputFiles({name:'guard-unsent.txt',mimeType:'text/plain',buffer:Buffer.from('guard native bytes')});
 const history=page.locator('.post-content').filter({hasText:`native key guard history ${id}`}).first();await expect(history).toBeVisible();await expect(history).toContainText(`native key guard history ${id}`);
 const state=async()=>({turns:await f.turns(),messages:await(await request.get(`/api/sessions/${f.main.id}/messages`)).json(),model:await(await request.get(`/api/sessions/${f.main.id}/model`)).json()});
 const before=await state();let mutations=0;page.on('request',r=>{if(!['GET','HEAD'].includes(r.method())&&new URL(r.url()).pathname.startsWith('/api/sessions'))mutations++;});
 const frames=()=>page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
 const open=async()=>{await history.click();await page.keyboard.press('m');await expect(f.palette).toHaveCount(1);await expect(f.query).toBeFocused();await expect(f.query).toHaveValue('m');};
 const close=async()=>{await f.query.press('Escape');await expect(f.palette).toHaveCount(0);await frames();};
 const unchanged=async(text='untouched draft')=>{await expect(f.input).toHaveValue(text);await expect(page.locator('.compose-input-main .compose-file-pill[title="guard-unsent.txt"]')).toHaveCount(1);expect(await page.evaluate(()=>localStorage.getItem('gi_session_id'))).toBe(f.main.id);expect(await state()).toEqual(before);expect(mutations).toBe(0);};
 // Prove the native palette is loaded and capable of opening before checking
 // that another surface or key prevents it. Never pass on an unready listener.
 await open();await close();await unchanged();
 return{...f,history,open,close,frames,unchanged};
}

test('Settings delivers target keys while native background popups remain suspended',async({page,request},info)=>{
 const f=await sharedTypingFixture(page,request,info,'Gi-only focus regression');
 const dialog=page.getByRole('dialog',{name:'Settings',exact:true});
 for(const background of ['model','session','palette','menu']){
  let popup,open;
  if(background==='model'){popup=page.getByRole('listbox',{name:'Models',exact:true});open=()=>page.getByRole('button',{name:'Open model picker',exact:true}).click();}
  if(background==='session'){popup=page.getByRole('menu',{name:'Sessions and agents',exact:true});open=()=>page.getByRole('button',{name:/Manage sessions for/}).last().click();}
  if(background==='palette'){popup=f.palette;open=f.open;}
  if(background==='menu'){popup=page.locator('.timeline-menu-dropdown');open=()=>page.getByTestId('hamburger').click();}
  await open();await expect(popup).toBeVisible();
  if(background==='model')await expect(popup.locator('.current-model')).toContainText('test/test-model');
  if(background!=='menu')await expect(popup.locator('.active')).toHaveCount(1);
  const active=()=>popup.locator('.active').evaluateAll(nodes=>nodes.map(n=>n.textContent));const before=await active();
  await page.keyboard.press('Control+,');await expect(dialog).toBeVisible();
  const input=dialog.getByLabel('Assistant display name',{exact:true});await input.click();await input.press('End');const original=await input.inputValue();
  await input.evaluate(el=>{window.__modalTargetKeys=[];el.addEventListener('keydown',e=>window.__modalTargetKeys.push({key:e.key,prevented:e.defaultPrevented}));});
  await input.press('q');await input.press('ArrowDown');await input.press('Home');await f.frames();await expect(input).toHaveValue(original+'q');
  expect(await page.evaluate(()=>window.__modalTargetKeys)).toEqual([{key:'q',prevented:false},{key:'ArrowDown',prevented:false},{key:'Home',prevented:false}]);expect(await active()).toEqual(before);
  await input.dispatchEvent('keydown',{key:'Escape',isComposing:true});await expect(dialog).toBeVisible();await expect(input).toBeFocused();expect(await page.evaluate(()=>window.__modalTargetKeys.at(-1))).toEqual({key:'Escape',prevented:false});
  const close=dialog.getByRole('button',{name:'Close settings',exact:true}),last=dialog.getByRole('button',{name:'Reload saved names',exact:true});await close.focus();await page.keyboard.press('Shift+Tab');await expect(last).toBeFocused();await page.keyboard.press('Tab');await expect(close).toBeFocused();expect(await active()).toEqual(before);
  await page.keyboard.press('Escape');await expect(dialog).toHaveCount(0);await expect(popup).toBeVisible();expect(await active()).toEqual(before);
  await page.keyboard.press('Escape');await expect(popup).toHaveCount(0);await f.frames();await f.unchanged();
 }
});

test.describe('Gi Quick Actions trusted touch dismissal',()=>{
 test.use({hasTouch:true});
 test('Gi palette touch dismissal consumes Send and restores Conversation without stale gesture capture',async({page,request},info)=>{
  const f=await sharedTypingFixture(page,request,info,'Gi-only focus regression'),conversation=page.getByRole('region',{name:'Conversation',exact:true});
  await conversation.focus();await conversation.press('m');await expect(f.query).toBeFocused();const send=page.getByRole('button',{name:'Send message',exact:true}),box=await send.boundingBox();
  await page.touchscreen.tap(box.x+box.width/2,box.y+box.height/2);await expect(f.palette).toHaveCount(0);await expect(conversation).toBeFocused();await f.unchanged();await f.input.tap();await expect(f.input).toBeFocused();
  await conversation.focus();await conversation.press('q');await expect(f.query).toBeFocused();await f.query.dispatchEvent('keydown',{key:'Escape',isComposing:true});await expect(f.palette).toBeVisible();await f.query.press('Escape');await expect(conversation).toBeFocused();await f.unchanged();
 });
});

test.describe('Gi Quick Actions close control touch',()=>{
 test.use({hasTouch:true});
 test('Gi Close button accepts trusted tap with actions still available',async({page,request},info)=>{
  const f=await sharedTypingFixture(page,request,info,'Gi-only focus regression'),conversation=page.getByRole('region',{name:'Conversation',exact:true});
  await conversation.focus();await conversation.press('m');await expect(f.query).toBeFocused();const close=f.palette.getByRole('button',{name:'Close quick actions',exact:true});await close.tap();await expect(f.palette).toHaveCount(0);await expect(conversation).toBeFocused();await f.unchanged();await f.input.tap();await expect(f.input).toBeFocused();
 });
});

test('Gi focused palette action button activates its own row rather than the search highlight',async({page,request},info)=>{
 const f=await sharedTypingFixture(page,request,info,'Gi-only focus regression'),conversation=page.getByRole('region',{name:'Conversation',exact:true});
 for(const key of ['Enter','Space']){
  await conversation.focus();await conversation.press('m');await expect(f.query).toBeFocused();await f.query.fill('');await expect(f.palette.locator('.timeline-quick-actions-item.active')).toHaveCount(1);await f.frames();
  const target=f.palette.getByRole('button').filter({has:page.locator('.timeline-quick-actions-item-title',{hasText:/^Open explorer$/})});await expect(target).toHaveCount(1);await expect(target).not.toHaveClass(/active/);await target.focus();
  let toggles=0;await page.locator('.app-shell').evaluate(el=>{window.__workspaceOpens=0;const o=new MutationObserver(records=>{for(const r of records)if(r.oldValue?.includes('workspace-collapsed')&&!el.classList.contains('workspace-collapsed'))window.__workspaceOpens++;});o.observe(el,{attributes:true,attributeFilter:['class'],attributeOldValue:true});window.__workspaceObserver=o;});
  await target.press(key);await expect(f.palette).toHaveCount(0);await expect(page.locator('.app-shell')).not.toHaveClass(/workspace-collapsed/);toggles=await page.evaluate(()=>{window.__workspaceObserver.disconnect();return window.__workspaceOpens;});expect(toggles).toBe(1);await page.locator('.workspace-toggle-tab.open').click();await f.unchanged();
 }
});
