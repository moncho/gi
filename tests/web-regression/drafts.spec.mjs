import { test, expect } from '@playwright/test';

const inputName = 'Message (Enter to send, Shift+Enter for newline)...';
const attachment = name => ({ name, mimeType: 'text/plain', buffer: Buffer.from(`contents of ${name}`) });
async function fixture(page, request, info) {
  const agent = `draft-${info.project.name}-${info.title.replace(/[^a-z0-9]/gi, '').slice(0,24)}`;
  const main = await (await request.post('/api/sessions', { data: { title: `@${agent}`, agent_id: agent } })).json();
  const fork = await (await request.post(`/api/sessions/${main.id}/fork`, { data: { title: `${agent}-child`, agent_id: `${agent}-child` } })).json();
  const child = fork.branch.chat_jid.slice(3);
  await page.addInitScript(id => { if (!localStorage.getItem('gi_session_id')) localStorage.setItem('gi_session_id', id); }, main.id);
  await page.goto('/');
  const input = page.getByRole('textbox', { name: inputName, exact: true });
  await expect(input).toBeVisible();
  const switchTo = async id => {
    await page.getByRole('button', { name: /Manage sessions for/ }).last().click();
    await page.locator(`[data-session-jid="gi:${id}"]`).getByRole('menuitem').click();
    await expect.poll(() => page.evaluate(() => localStorage.getItem('gi_session_id'))).toBe(id);
  };
  return { main, child, input, switchTo };
}
async function storedDraft(page, id) {
  return page.evaluate(async id => {
    const db = await new Promise((resolve,reject) => {
      const request = indexedDB.open('gi-session-drafts');
      request.onsuccess = () => resolve(request.result); request.onerror = () => reject(request.error);
    });
    return new Promise((resolve,reject) => {
      const tx = db.transaction('drafts','readonly'); const req = tx.objectStore('drafts').get(id);
      tx.oncomplete = () => { db.close(); resolve(req.result ? { text:req.result.draft.text, media:req.result.draft.media.map(f=>f.name), refs:req.result.draft.messageRefs, fileRefs:req.result.draft.fileRefs, pending:req.result.pending.length } : null); };
      tx.onerror = () => reject(tx.error);
    });
  }, id);
}

async function attachWorkspaceFile(page, request) {
  const response = await request.post('/api/tools/execute', {data:{tool:'write', input:{path:'draft-reference.txt',content:'Draft reference fixture'}}});
  expect(response.ok()).toBe(true);
  await page.getByTestId('hamburger').click();
  await page.getByRole('menuitem', {name:'Show workspace',exact:true}).click();
  await page.locator('.workspace-row[data-path="draft-reference.txt"]').click();
  await page.getByTestId('hamburger').click();
  await page.getByRole('menuitem', {name:'Hide workspace',exact:true}).click();
  await expect(page.locator('.compose-file-pill[title="draft-reference.txt"]')).toBeVisible();
}
async function history(request, session) {
  const response = await request.post(`/api/sessions/${session}/prompt`, { data: { prompt:`Reference ${session}`, model:'test-model' } });
  expect(response.status()).toBe(202);
  await expect.poll(async () => {
    const { messages } = await (await request.get(`/api/sessions/${session}/messages`)).json();
    return messages?.length || 0;
  }).toBe(2);
}

test('Gi drafts retain text, file bytes and message references across reload and session switches', async ({page,request},info) => {
  const { main,child,input,switchTo } = await fixture(page,request,info);
  await history(request,main.id); await page.reload();
  await page.locator('.post-time').first().click();
  await expect(page.locator('.compose-file-pill[title^="Message reference:"]')).toHaveCount(1);
  await attachWorkspaceFile(page,request);
  await input.fill('A durable draft\n第二行');
  await page.locator('.compose-box input[type=file]').setInputFiles(attachment('draft-a.txt'));
  await expect.poll(()=>storedDraft(page,main.id)).toMatchObject({ text:'A durable draft\n第二行', media:['draft-a.txt'] });
  await switchTo(child); await expect(input).toHaveValue('');
  await input.fill('B durable draft');
  await expect.poll(()=>storedDraft(page,child)).toMatchObject({ text:'B durable draft' });
  await page.reload(); await expect(input).toHaveValue('B durable draft');
  await switchTo(main.id);
  await expect(input).toHaveValue('A durable draft\n第二行');
  await expect(page.locator('.compose-file-pill[title="draft-reference.txt"]')).toBeVisible();
  await expect(page.locator('.compose-file-pill[title^="Message reference:"]')).toHaveCount(1);
  await expect(page.locator('.compose-file-pill').filter({hasText:'draft-a.txt'})).toBeVisible();
  // Real upload of the restored File proves bytes survive structured cloning.
  await input.press('Enter');
  await expect.poll(async () => {
    const {media} = await (await request.get(`/api/sessions/${main.id}/media`)).json(); return media?.length || 0;
  }).toBe(1);
  const {media} = await (await request.get(`/api/sessions/${main.id}/media`)).json();
  const bytes = await request.get(`/api/sessions/${main.id}/media/${media[0].id}`);
  expect(await bytes.text()).toBe('contents of draft-a.txt');
  await expect.poll(()=>storedDraft(page,main.id)).toMatchObject({text:'',media:[],pending:0});
  await page.reload(); await expect(input).toHaveValue('');
  await expect(page.locator('.compose-file-pill')).toHaveCount(0);
});

test('Gi failed send merges into a newer draft on an A-B-A revisit', async ({page,request},info) => {
  const {main,child,input,switchTo}=await fixture(page,request,info);
  let release,held=false; const gate=new Promise(resolve=>{release=resolve;});
  await page.route(`**/api/sessions/${main.id}/prompt`,async route=>{held=true;await gate;await route.abort('failed');});
  try {
    await input.fill('old A send');await input.press('Enter');await expect.poll(()=>held).toBe(true);
    await switchTo(child);await input.fill('B draft');await switchTo(main.id);
    await input.fill('new visit draft');release();
    await expect(input).toHaveValue('old A send\n\nnew visit draft');
    await input.fill('edited after recovery');
    await switchTo(child);await expect(input).toHaveValue('B draft');
    await switchTo(main.id);await expect(input).toHaveValue('edited after recovery');
    await expect.poll(()=>storedDraft(page,main.id)).toMatchObject({text:'edited after recovery',pending:0});
  } finally {release();}
});

test('Gi acknowledgement cleanup failure warns without restoring a delivered draft', async ({page,request},info) => {
  await page.addInitScript(()=>{
    const put=IDBObjectStore.prototype.put;
    let captured=false;
    IDBObjectStore.prototype.put=function(value,...args){
      if(this.name==='drafts') {
        if(value.pending?.length)captured=true;
        else if(captured)throw new DOMException('Cleanup quota failure','QuotaExceededError');
      }
      return put.call(this,value,...args);
    };
  });
  const {main,input}=await fixture(page,request,info);
  await input.fill('acknowledged once');await input.press('Enter');
  await expect(page.getByRole('alert').filter({hasText:'Send acknowledged, but draft cleanup failed'})).toBeVisible();
  await expect(input).toHaveValue('');
  const {turns}=await (await request.get(`/api/sessions/${main.id}/turns`)).json();
  expect(turns.filter(t=>t.prompt==='acknowledged once')).toHaveLength(1);
});

test('Gi reload recovers an unacknowledged send without resubmitting it', async ({page,request},info) => {
  const {main,input}=await fixture(page,request,info);
  let release,held=false;const gate=new Promise(resolve=>{release=resolve;});
  await page.route(`**/api/sessions/${main.id}/prompt`,async route=>{held=true;await gate;await route.abort('failed').catch(()=>{});});
  try {
    await input.fill('uncertain send');await input.press('Enter');await expect.poll(()=>held).toBe(true);
    await input.fill('new draft');await expect.poll(()=>storedDraft(page,main.id)).toMatchObject({text:'new draft',pending:1});
    await page.reload();
    await expect(input).toHaveValue('uncertain send\n\nnew draft');
    await expect(page.getByRole('alert')).toContainText('Delivery is unknown');
    release();await page.unroute(`**/api/sessions/${main.id}/prompt`);
    await page.reload();await expect(input).toHaveValue('uncertain send\n\nnew draft');
    const {turns}=await (await request.get(`/api/sessions/${main.id}/turns`)).json();expect(turns||[]).toHaveLength(0);
  } finally {release();}
});

test('Gi storage failure retains draft and prevents an unprotected send', async ({page,request},info) => {
  // Storage fault injection only; UI state and timeline remain native.
  await page.addInitScript(()=>{const original=IDBObjectStore.prototype.put;IDBObjectStore.prototype.put=function(...args){if(this.name==='drafts')throw new DOMException('Test quota exhausted','QuotaExceededError');return original.apply(this,args);};});
  const {main,input}=await fixture(page,request,info);
  await input.fill('keep on quota failure');await input.press('Enter');
  await expect(input).toHaveValue('keep on quota failure');
  await expect(page.getByRole('alert').filter({hasText:'Draft not saved'})).toBeVisible();
  const {turns}=await (await request.get(`/api/sessions/${main.id}/turns`)).json();expect(turns||[]).toHaveLength(0);
});

// Hold only a real accepted HTTP response; native persistence, SSE and provider
// execution continue. No invented post payload or replacement EventSource.
async function holdAcknowledgement(page, session) {
  let release, held = false, responseStatus;
  const gate = new Promise(resolve => { release = resolve; });
  const pattern = `**/api/sessions/${session}/prompt`;
  await page.route(pattern, async route => {
    const response = await route.fetch();
    responseStatus = response.status(); held = true;
    await gate; await route.fulfill({ response });
  });
  return { release, held: () => held, status: () => responseStatus,
    async close() { release(); await page.unrouteAll({ behavior: 'wait' }); } };
}
async function completedPrompt(request, session, prompt) {
  await expect.poll(async () => {
    const { turns } = await (await request.get(`/api/sessions/${session}/turns`)).json();
    return turns?.find(turn => turn.prompt === prompt)?.status;
  }).toBe('completed');
  // Wait for display-idle too. This does not establish active-claim release:
  // rapid history setup must use explicit queue intent to avoid steering.
  await expect.poll(async () => (await (await request.get(`/api/sessions/${session}/activity`)).json()).status).toBe('idle');
}
async function messages(request, session) {
  return (await (await request.get(`/api/sessions/${session}/messages`)).json()).messages || [];
}

test('Gi delayed acknowledgement refreshes the current search rather than the hidden timeline', async ({page,request},info) => {
  const {main,input} = await fixture(page,request,info);
  const ack = await holdAcknowledgement(page,main.id);
  try {
    await input.fill('searchable accepted text'); await input.press('Enter');
    await expect.poll(ack.held).toBe(true); await completedPrompt(request,main.id,'searchable accepted text');
    await input.fill('draft beneath search');
    await page.getByRole('button',{name:'Search',exact:true}).click();
    const search=page.getByRole('textbox',{name:'Search (Enter to run)...',exact:true});
    await search.fill('no-such-message'); await search.press('Enter');
    await expect(page.getByText('No matching messages.',{exact:true})).toBeVisible();
    let searches=0,pages=0;
    page.on('request',req=>{const path=new URL(req.url()).pathname;if(path.endsWith(`/${main.id}/search`))searches++;if(path.endsWith(`/${main.id}/messages`))pages++;});
    ack.release(); await expect.poll(()=>storedDraft(page,main.id)).toMatchObject({pending:0});
    await expect.poll(()=>searches).toBeGreaterThan(0); expect(pages).toBe(0);
    await expect(search).toHaveValue('no-such-message'); await expect(page.locator('.timeline .post')).toHaveCount(0);
    await search.press('Escape'); await expect(input).toHaveValue('draft beneath search');
    const stored=(await messages(request,main.id)).find(m=>m.role==='user');
    await expect(page.locator(`#post-${stored.id}`)).toContainText(stored.content);
  } finally { await ack.close(); }
});

test('Gi accepted origin response cannot refresh a newly selected chat', async ({page,request},info) => {
  const {main,child,input,switchTo} = await fixture(page,request,info);
  const ack = await holdAcknowledgement(page,main.id);
  try {
    await input.fill('accepted in origin'); await input.press('Enter');
    await expect.poll(ack.held).toBe(true); await completedPrompt(request,main.id,'accepted in origin');
    await switchTo(child); await input.fill('child draft remains');
    await expect.poll(()=>storedDraft(page,child)).toMatchObject({text:'child draft remains'});
    ack.release(); await expect.poll(()=>storedDraft(page,main.id)).toMatchObject({text:'',pending:0});
    await expect(input).toHaveValue('child draft remains'); await expect(page.locator('.timeline .post')).toHaveCount(0);
    expect(await messages(request,child)).toEqual([]);
    await switchTo(main.id); await expect(input).toHaveValue('');
    const stored=(await messages(request,main.id)).find(m=>m.role==='user');
    await expect(page.locator(`#post-${stored.id}`)).toContainText('accepted in origin');
    await expect(page.locator(`#post-${stored.id}`)).toHaveCount(1);
  } finally { await ack.close(); }
});

async function showWorkspace(page) {
  await page.getByTestId('hamburger').click();
  await page.getByRole('menuitem',{name:'Show workspace',exact:true}).click();
}
async function hideWorkspace(page) {
  await page.getByTestId('hamburger').click();
  await page.getByRole('menuitem',{name:'Hide workspace',exact:true}).click();
}
async function createReferenceFolder(request) {
  const response=await request.post('/api/tools/execute',{data:{tool:'write',input:{path:'reference-folder/child.txt',content:'Folder reference fixture'}}});
  expect(response.ok()).toBe(true);
}

test('Gi explicit folder references follow selection and remain durable and session-local',async({page,request},info)=>{
  const {main,child,input,switchTo}=await fixture(page,request,info);
  await createReferenceFolder(request);await page.reload();await showWorkspace(page);
  const reference=page.getByRole('button',{name:'Reference selected folder',exact:true});
  const folder=page.locator('.workspace-row[data-path="reference-folder"]');
  await folder.click();await reference.click();await expect(reference).toBeDisabled();
  await hideWorkspace(page);await input.fill('A folder draft');
  await expect.poll(()=>storedDraft(page,main.id)).toMatchObject({text:'A folder draft'});
  await page.reload();await expect(page.locator('.compose-file-pill[title="reference-folder"]')).toBeVisible();
  await showWorkspace(page);await folder.click();await expect(reference).toBeDisabled();
  // A file selection must remove the folder action, leaving existing file attach behaviour.
  await page.locator('.workspace-row[data-path="reference-folder/child.txt"]').click();
  await expect(reference).toBeHidden();await hideWorkspace(page);
  await switchTo(child);await expect(page.locator('.compose-file-pill')).toHaveCount(0);
  await input.fill('B draft');await showWorkspace(page);await folder.click();await expect(reference).toBeEnabled();
  await reference.click();await expect(reference).toBeDisabled();await hideWorkspace(page);
  await expect(page.locator('.compose-file-pill[title="reference-folder"]')).toHaveCount(1);
  // Removal re-enables the action, and repeated open/close does not install duplicates.
  await page.locator('.compose-file-pill[title="reference-folder"] button').click();
  await showWorkspace(page);await expect(reference).toHaveCount(1);await expect(reference).toBeEnabled();
  await hideWorkspace(page);await showWorkspace(page);await expect(reference).toHaveCount(1);await hideWorkspace(page);
  await switchTo(main.id);await expect(input).toHaveValue('A folder draft');
  await expect(page.locator('.compose-file-pill[title="reference-folder"]')).toHaveCount(1);
  await expect(page.locator('.compose-file-pill[title="reference-folder/child.txt"]')).toHaveCount(1);
  expect(await messages(request,main.id)).toEqual([]);expect(await messages(request,child)).toEqual([]);
});

test('Gi failed folder-reference send restores its origin without altering another session',async({page,request},info)=>{
  const {main,child,input,switchTo}=await fixture(page,request,info);
  await createReferenceFolder(request);await page.reload();await showWorkspace(page);
  await page.locator('.workspace-row[data-path="reference-folder"]').click();
  await page.getByRole('button',{name:'Reference selected folder',exact:true}).click();await hideWorkspace(page);
  await input.fill('captured folder draft');
  await expect.poll(()=>storedDraft(page,main.id)).toMatchObject({fileRefs:['reference-folder']});
  let release,held=false;const gate=new Promise(resolve=>{release=resolve});
  await page.route(`**/api/sessions/${main.id}/prompt`,async route=>{held=true;await gate;await route.abort('failed');});
  try {
    await input.press('Enter');await expect.poll(()=>held).toBe(true);await expect(page.locator('.compose-file-pill')).toHaveCount(0);
    await input.fill('newer origin typing');await switchTo(child);await input.fill('child stays untouched');release();
    await expect.poll(()=>storedDraft(page,main.id)).toMatchObject({text:'captured folder draft\n\nnewer origin typing',fileRefs:['reference-folder'],pending:0});
    await expect(input).toHaveValue('child stays untouched');await expect(page.locator('.compose-file-pill')).toHaveCount(0);
    await switchTo(main.id);await expect(page.locator('.compose-file-pill[title="reference-folder"]')).toHaveCount(1);
    await page.reload();await expect(input).toHaveValue('captured folder draft\n\nnewer origin typing');
    await expect(page.locator('.compose-file-pill[title="reference-folder"]')).toHaveCount(1);
    expect(await messages(request,main.id)).toEqual([]);expect(await messages(request,child)).toEqual([]);
  }finally{release();await page.unrouteAll({behavior:'wait'});}
});

for (const successfulPrefix of [0, 1]) {

}

test('Gi native upload transport retains bytes and emits real progress without browser routing',async({page,request},info)=>{
 await page.addInitScript(()=>{
  window.__uploadProgress=[];
  const open=XMLHttpRequest.prototype.open;
  XMLHttpRequest.prototype.open=function(method,url,...rest){
   if(String(url).endsWith('/media'))this.upload.addEventListener('progress',e=>window.__uploadProgress.push({loaded:e.loaded,total:e.total,computable:e.lengthComputable}));
   return open.call(this,method,url,...rest);
  };
 });
 const {main,input}=await fixture(page,request,info);
 const file={name:'unrouted.txt',mimeType:'text/plain',buffer:Buffer.alloc(512*1024,'p')};
 await input.fill('unrouted bytes');await page.locator('.compose-box input[type=file]').setInputFiles(file);await input.press('Enter');
 await expect.poll(async()=>((await(await request.get(`/api/sessions/${main.id}/media`)).json()).media??[]).length).toBe(1);
 const {media}=await(await request.get(`/api/sessions/${main.id}/media`)).json();
 expect(await(await request.get(`/api/sessions/${main.id}/media/${media[0].id}`)).body()).toEqual(file.buffer);
 await expect.poll(()=>page.evaluate(()=>window.__uploadProgress.some(p=>p.computable&&p.loaded>0&&p.total>512*1024))).toBe(true);
 await expect(page.locator('.gi-compose-transfer')).toBeHidden();
});

test('Gi overlapping upload and send operations complete independently and clear on native rejection',async({page,request},info)=>{
 const {main,input}=await fixture(page,request,info);
 let releaseUpload,releaseFirst,releaseSecond,uploadHeld=false,firstHeld=false,secondHeld=false;
 const uploadGate=new Promise(r=>releaseUpload=r),firstGate=new Promise(r=>releaseFirst=r),secondGate=new Promise(r=>releaseSecond=r);
 await page.route(`**/api/sessions/${main.id}/media`,async route=>{const response=await route.fetch({postData:route.request().postDataBuffer()});uploadHeld=true;await uploadGate;await route.fulfill({response});});
 await page.route(`**/api/sessions/${main.id}/prompt`,async route=>{
  const body=route.request().postDataJSON();
  if(body.prompt.startsWith('first attachment')){firstHeld=true;await firstGate;}else{secondHeld=true;await secondGate;}
  const response=await route.fetch();await route.fulfill({response});
 });
 try{
  await input.fill('first attachment');await page.locator('.compose-box input[type=file]').setInputFiles(attachment('concurrent.txt'));await input.press('Enter');await expect.poll(()=>uploadHeld).toBe(true);
  await input.fill('second plain');await input.press('Enter');await expect.poll(()=>secondHeld).toBe(true);
  await expect(page.locator('.gi-compose-upload')).toBeVisible();await expect(page.locator('.gi-compose-sending')).toHaveText('Sending message…');
  releaseUpload();await expect.poll(()=>firstHeld).toBe(true);await expect(page.locator('.gi-compose-upload')).toHaveCount(0);await expect(page.locator('.gi-compose-sending')).toHaveText('Sending 2 messages…');
  releaseFirst();await expect(page.locator('.gi-compose-sending')).toHaveText('Sending message…');await expect(page.locator('.compose-send-stack .send-btn')).toHaveAttribute('aria-busy','true');
  releaseSecond();await expect(page.locator('.gi-compose-transfer')).toBeHidden();
  await expect.poll(async()=>((await(await request.get(`/api/sessions/${main.id}/messages`)).json()).messages??[]).filter(m=>m.role==='user').length).toBe(2);
  await page.unroute(`**/api/sessions/${main.id}/media`);await page.unroute(`**/api/sessions/${main.id}/prompt`);
  // Corrupt only the HTTP header; the real Go multipart parser rejects it.
  let rejected=0;
  await page.route(`**/api/sessions/${main.id}/media`,async route=>{const response=await route.fetch({headers:{...route.request().headers(),'content-type':'multipart/form-data'}});rejected=response.status();await route.fulfill({response});});
  await input.fill('retry draft');await page.locator('.compose-box input[type=file]').setInputFiles(attachment('reject.txt'));await input.press('Enter');
  await expect.poll(()=>rejected).toBe(400);await expect(input).toHaveValue('retry draft');await expect(page.locator('.gi-compose-transfer')).toBeHidden();await expect(page.locator('.compose-send-stack .send-btn')).not.toHaveAttribute('aria-busy','true');
  await expect(page.getByRole('alert')).toContainText('no multipart boundary');
 }finally{releaseUpload();releaseFirst();releaseSecond()}
});

test('Gi Cancel uploads aborts captured batch before send, keeps exact draft/files and retries once',async({page,request},info)=>{
 const {main,child,input,switchTo}=await fixture(page,request,info);
 let release,held=false,uploadRequests=0,prompts=0;const gate=new Promise(r=>release=r);
 await page.addInitScript(()=>{window.__uploadAborts=0;const open=XMLHttpRequest.prototype.open;XMLHttpRequest.prototype.open=function(method,url,...args){if(String(url).endsWith('/media'))this.addEventListener('abort',()=>window.__uploadAborts++);return open.call(this,method,url,...args);};});
 await page.reload();await expect(input).toBeVisible();
 // Native response is already durable when cancelled; no fictitious rollback.
 await page.route(`**/api/sessions/${main.id}/media`,async route=>{uploadRequests++;const response=await route.fetch({postData:route.request().postDataBuffer()});expect(response.status()).toBe(201);held=true;await gate;try{await route.fulfill({response});}catch{}});
 page.on('request',r=>{if(r.method()==='POST'&&new URL(r.url()).pathname.endsWith('/prompt'))prompts++;});
 const files=[attachment('cancel-a.txt','exact α bytes'),attachment('cancel-b.txt','exact β bytes')];
 const exact=()=>page.evaluate(async id=>{const db=await new Promise((ok,no)=>{const r=indexedDB.open('gi-session-drafts');r.onsuccess=()=>ok(r.result);r.onerror=()=>no(r.error);});const value=await new Promise((ok,no)=>{const r=db.transaction('drafts').objectStore('drafts').get(id);r.onsuccess=()=>ok(r.result);r.onerror=()=>no(r.error);});db.close();return {text:value?.draft.text,files:value?.draft.media.map(f=>({name:f.name,bytes:Array.from(new Uint8Array(f.bytes))})),pending:value?.pending.length};},main.id);
 try{
  await input.fill('cancel draft 中文');await page.locator('.compose-box input[type=file]').setInputFiles(files);await expect.poll(exact).toMatchObject({text:'cancel draft 中文',files:files.map(f=>({name:f.name,bytes:[...f.buffer]})),pending:0});const before=await exact();
  await input.press('Enter');await expect.poll(()=>held).toBe(true);await expect(page.getByRole('button',{name:'Cancel uploads',exact:true})).toBeVisible();
  await switchTo(child);await input.fill('independent child');await expect(page.getByRole('button',{name:'Cancel uploads',exact:true})).toHaveCount(0);await switchTo(main.id);
  await page.getByRole('button',{name:'Cancel uploads',exact:true}).click();await expect.poll(()=>page.evaluate(()=>window.__uploadAborts)).toBe(1);await expect(input).toHaveValue('cancel draft 中文');await expect(page.getByRole('alert')).toContainText('Upload cancelled');await expect(page.locator('.gi-compose-transfer')).toBeHidden();await expect.poll(exact).toEqual(before);expect(prompts).toBe(0);expect(uploadRequests).toBe(1);
  release();await page.unroute(`**/api/sessions/${main.id}/media`);await page.reload();await expect(input).toHaveValue('cancel draft 中文');await expect(page.locator('.compose-box').getByText('cancel-a.txt',{exact:true})).toBeVisible();await expect(page.locator('.compose-box').getByText('cancel-b.txt',{exact:true})).toBeVisible();await expect.poll(exact).toEqual(before);
  await input.press('Enter');await expect.poll(async()=> (await messages(request,main.id)).filter(m=>m.role==='user').length).toBe(1);const user=(await messages(request,main.id)).find(m=>m.role==='user');expect(user.payload.media).toHaveLength(2);expect(new Set(user.payload.media.map(m=>m.media_id)).size).toBe(2);
  for(const [index,media]of user.payload.media.entries())expect(await(await request.get(`/api/sessions/${main.id}/media/${media.media_id}`)).body()).toEqual(files[index].buffer);
  expect((await(await request.get(`/api/sessions/${main.id}/media`)).json()).media).toHaveLength(2);expect(prompts).toBe(1);expect((await messages(request,child)).length).toBe(0);await switchTo(child);await expect(input).toHaveValue('independent child');await switchTo(main.id);await page.reload();expect((await messages(request,main.id)).filter(m=>m.role==='user')).toHaveLength(1);
  await page.screenshot({path:info.outputPath('cancelled-upload-retry.png')});
 }finally{release();}
});

test('Gi upload cancellation merges newer draft and leaves another session upload and a dispatched send alone',async({page,request},info)=>{
 const {main,child,input,switchTo}=await fixture(page,request,info);
 let releaseUploads,releaseSend;const uploadGate=new Promise(r=>releaseUploads=r),sendGate=new Promise(r=>releaseSend=r);const uploads=[];let sending=false;
 await page.route('**/api/sessions/*/media',async route=>{if(route.request().method()!=='POST')return route.continue();const response=await route.fetch({postData:route.request().postDataBuffer()});uploads.push(route.request().url());await uploadGate;try{await route.fulfill({response});}catch{}});
 await page.route(`**/api/sessions/${child}/prompt`,async route=>{sending=true;await sendGate;const response=await route.fetch();await route.fulfill({response});});
 try{
  await input.fill('origin captured');await page.locator('.compose-box input[type=file]').setInputFiles(attachment('origin.txt'));await input.press('Enter');await expect.poll(()=>uploads.length).toBe(1);
  await input.fill('newer origin');await page.locator('.compose-box input[type=file]').setInputFiles(attachment('newer.txt'));
  await switchTo(child);await input.fill('child captured');await page.locator('.compose-box input[type=file]').setInputFiles(attachment('child.txt'));await input.press('Enter');await expect.poll(()=>uploads.length).toBe(2);await input.fill('newer child');
  await switchTo(main.id);await page.getByRole('button',{name:'Cancel uploads',exact:true}).click();await expect(input).toHaveValue('origin captured\n\nnewer origin');await expect.poll(()=>storedDraft(page,main.id)).toMatchObject({media:['origin.txt','newer.txt'],pending:0});
  await switchTo(child);await expect(input).toHaveValue('newer child');await expect(page.getByRole('button',{name:'Cancel uploads',exact:true})).toBeVisible();releaseUploads();await expect.poll(()=>sending).toBe(true);await expect(page.getByRole('button',{name:'Cancel uploads',exact:true})).toHaveCount(0);await expect(page.locator('.gi-compose-sending')).toBeVisible();
  releaseSend();await expect.poll(async()=> (await messages(request,child)).filter(m=>m.role==='user').length).toBe(1);expect((await messages(request,main.id)).length).toBe(0);await expect(input).toHaveValue('newer child');await switchTo(main.id);await expect(input).toHaveValue('origin captured\n\nnewer origin');await page.reload();await expect(input).toHaveValue('origin captured\n\nnewer origin');
 }finally{releaseUploads();releaseSend();}
});

test('@ux-compose-007 Accepted response notifies refresh and displays the stored message', async ({page,request},info) => {

  const {main,input} = await fixture(page,request,info);
  const ack = await holdAcknowledgement(page,main.id);
  let reads = 0;
  page.on('request',req => { if(new URL(req.url()).pathname === `/api/sessions/${main.id}/messages`) reads++; });
  try {
    await input.fill('accepted native text'); await input.press('Enter');
    await expect.poll(ack.held).toBe(true); expect(ack.status()).toBe(202);
    await completedPrompt(request,main.id,'accepted native text');
    const stored = (await messages(request,main.id)).find(m => m.role === 'user');
    expect(stored.content).toBe('accepted native text');
    await expect(page.locator(`#post-${stored.id}`)).toContainText(stored.content);
    // SSE has already delivered the completed turn; HTTP acknowledgement still
    // owns a refresh independently (and never adds a synthetic duplicate).
    const before = reads; ack.release();
    await expect.poll(() => reads).toBeGreaterThan(before);
    await expect.poll(() => storedDraft(page,main.id)).toMatchObject({pending:0});
    await expect(page.locator(`#post-${stored.id}`)).toHaveCount(1);
    await page.reload(); await expect(page.locator(`#post-${stored.id}`)).toContainText(stored.content);
  } finally { await ack.close(); }
});

test('@ux-compose-009 Multiple native uploads retain filename, identifier and byte association', async ({page,request},info) => {

  const {main,input} = await fixture(page,request,info);
  const files = [attachment('first α.txt'), attachment('second β.txt'), attachment('third.txt')];
  const uploads = [];
  page.on('response', response => {
    if(response.request().method()==='POST' && response.url().endsWith(`/api/sessions/${main.id}/media`))
      uploads.push(response.json().then(data=>data.media));
  });
  const sent = page.waitForRequest(req => req.method()==='POST' && req.url().endsWith(`/api/sessions/${main.id}/prompt`));
  await input.fill('  attachment batch  ');
  await page.locator('.compose-box input[type=file]').setInputFiles(files);
  await input.press('Enter'); const body = (await sent).postDataJSON();
  expect(uploads).toHaveLength(files.length);
  const uploaded = await Promise.all(uploads);
  expect(new Set(uploaded.map(u=>u.id)).size).toBe(files.length);
  expect(body.media).toEqual(uploaded.map(u=>({media_id:u.id,session_id:main.id})));
  expect(body.prompt).toBe('attachment batch\n\nAttachments:\n'+uploaded.map((u,i)=>`- attachment:${u.id} (${files[i].name})`).join('\n'));
  await completedPrompt(request,main.id,body.prompt);
  const {media} = await (await request.get(`/api/sessions/${main.id}/media`)).json();
  for(let i=0;i<uploaded.length;i++) {
    const native = media.find(m=>m.id===uploaded[i].id); expect(native.filename).toBe(files[i].name);
    const bytes = await request.get(`/api/sessions/${main.id}/media/${uploaded[i].id}`);
    expect(await bytes.body()).toEqual(files[i].buffer);
  }
  const stored = (await messages(request,main.id)).find(m=>m.role==='user');
  expect(stored.content).toBe(body.prompt);
  await expect(page.locator(`#post-${stored.id}`)).toContainText('second β.txt');
  await expect.poll(()=>storedDraft(page,main.id)).toMatchObject({text:'',media:[],pending:0});
});

test('@ux-compose-010 Earlier acknowledgement never resets newer typing or attachments', async ({page,request},info) => {

  const {main,input} = await fixture(page,request,info);
  const ack = await holdAcknowledgement(page,main.id);
  try {
    await input.fill('earlier captured'); await input.press('Enter');
    await expect(input).toHaveValue(''); await expect.poll(ack.held).toBe(true);
    await completedPrompt(request,main.id,'earlier captured');
    await input.fill('new draft\n第二行');
    await page.locator('.compose-box input[type=file]').setInputFiles(attachment('new-unsent.txt'));
    await input.press('Home'); await input.press('ArrowRight');
    const cursor = await input.evaluate(el => [el.selectionStart,el.selectionEnd]);
    ack.release();
    await expect.poll(()=>storedDraft(page,main.id)).toMatchObject({text:'new draft\n第二行',media:['new-unsent.txt'],pending:0});
    await expect(input).toHaveValue('new draft\n第二行');
    expect(await input.evaluate(el => [el.selectionStart,el.selectionEnd])).toEqual(cursor);
    const stored = await messages(request,main.id);
    expect(stored.filter(m=>m.role==='user').map(m=>m.content)).toEqual(['earlier captured']);
    await page.reload(); await expect(input).toHaveValue('new draft\n第二行');
    await expect(page.locator('.compose-file-pill').filter({hasText:'new-unsent.txt'})).toBeVisible();
  } finally { await ack.close(); }
});

test('@ux-compose-011 Native posts reconcile once and respect current history-reading position', async ({page,request},info) => {
  test.setTimeout(90000);
  const {main,input} = await fixture(page,request,info);
  for(let i=0;i<16;i++) {
    const prompt = `Reader history ${i}\n` + Array.from({length:8},(_,n)=>`Historical line ${n}`).join('\n');
    // Explicit queue admission remains a distinct turn during the tiny gap
    // between completed status and release of the prior native active claim.
    expect((await request.post(`/api/sessions/${main.id}/prompt`,{data:{prompt,model:'test-model',intent:'queue'}})).status()).toBe(202);
    await completedPrompt(request,main.id,prompt);
  }
  // Queue admission can add a native status message. Assert the actual
  // persisted baseline rather than assuming exactly two rows per turn.
  const baseline=(await messages(request,main.id)).length;
  await page.reload(); const timeline = page.locator('.timeline');
  await expect(page.locator('.timeline .post')).toHaveCount(baseline);
  const ack = await holdAcknowledgement(page,main.id);
  try {
    await input.fill('send before reading history'); await input.press('Enter');
    await expect.poll(ack.held).toBe(true); await completedPrompt(request,main.id,'send before reading history');
    await expect(page.locator('.timeline .post')).toHaveCount(baseline+2);
    await input.fill('next unsent reader draft');
    await timeline.hover(); await page.mouse.wheel(0,-700);
    await expect.poll(()=>timeline.evaluate(el=>el.scrollTop)).toBeLessThan(-100);
    // WebKit wheel/entry animations must finish before measuring a reading anchor.
    await page.waitForTimeout(350);
    const anchor = await timeline.evaluate(root=>{
      const box=root.getBoundingClientRect();
      const node=[...root.querySelectorAll('.post[id]')].find(el=>{const r=el.getBoundingClientRect();return r.top>=box.top&&r.top<box.bottom;});
      return {id:node.id,top:node.getBoundingClientRect().top};
    });
    ack.release(); await expect.poll(()=>storedDraft(page,main.id)).toMatchObject({pending:0});
    const position = () => page.locator(`[id="${anchor.id}"]`).evaluate(el=>el.getBoundingClientRect().top);
    await expect.poll(async()=>Math.abs(await position()-anchor.top)).toBeLessThanOrEqual(1);
    // Real later arrivals follow the same policy, not a separate response-only path.
    expect((await request.post(`/api/sessions/${main.id}/prompt`,{data:{prompt:'later native arrival',model:'test-model'}})).status()).toBe(202);
    await completedPrompt(request,main.id,'later native arrival');
    await expect(page.locator('.timeline .post')).toHaveCount(baseline+4);
    await expect.poll(async()=>Math.abs(await position()-anchor.top)).toBeLessThanOrEqual(1);
    await expect(input).toHaveValue('next unsent reader draft');
    await timeline.hover(); await page.mouse.wheel(0,100000);
    await expect.poll(()=>timeline.evaluate(el=>Math.abs(el.scrollTop))).toBeLessThanOrEqual(1);
    await input.fill('near-bottom submission'); await input.press('Enter');
    await completedPrompt(request,main.id,'near-bottom submission');
    await expect(page.locator('.timeline .post')).toHaveCount(baseline+6);
    await expect.poll(()=>timeline.evaluate(el=>Math.abs(el.scrollTop))).toBeLessThanOrEqual(1);
    const native = await messages(request,main.id);
    const ids = await page.locator('.timeline .post').evaluateAll(nodes=>nodes.map(n=>n.id.slice(5)));
    expect(ids).toEqual(native.map(m=>m.id)); expect(new Set(ids).size).toBe(ids.length);
  } finally { await ack.close(); }
});

test('@ux-compose-008 Serialize text, file, folder and message references and send references alone',async({page,request},info)=>{

  const {main,input}=await fixture(page,request,info);
  await createReferenceFolder(request);await history(request,main.id);await page.reload();
  const message=page.locator('.timeline .post').first();const messageId=(await message.getAttribute('id')).slice(5);
  // References carry the message's numeric row ID (as Piclaw's do), not its text ID.
  const conversation=await(await request.get(`/api/sessions/${main.id}/messages?view=conversation&limit=50`)).json();
  const messageRef=conversation.messages.find(m=>m.id===messageId).display_row_id;expect(Number.isInteger(messageRef)).toBe(true);
  await message.locator('.post-time').click();
  await expect(page.locator(`.compose-file-pill[title="Message reference: ${messageRef}"]`)).toBeVisible();
  await attachWorkspaceFile(page,request);
  await showWorkspace(page);
  const folder=page.locator('.workspace-row[data-path="reference-folder"]');
  await folder.click();
  // Navigation alone does not attach directories or silently submit anything.
  await expect(page.locator('.compose-file-pill[title="reference-folder"]')).toHaveCount(0);
  const reference=page.getByRole('button',{name:'Reference selected folder',exact:true});
  await expect(reference).toBeEnabled();await expect(reference).toHaveAttribute('title','Reference folder: reference-folder');
  await reference.press('Enter');await expect(reference).toBeDisabled();
  await hideWorkspace(page);
  await expect(page.locator('.compose-file-pill[title="reference-folder"]')).toBeVisible();
  await input.fill('  multiline draft\n第二行  ');
  const sent=page.waitForRequest(req=>req.method()==='POST'&&req.url().endsWith(`/api/sessions/${main.id}/prompt`));
  await input.press('Enter');const body=(await sent).postDataJSON();
  const expected=`multiline draft\n第二行\n\nFiles:\n- draft-reference.txt\n- reference-folder\n\nReferenced messages:\n- message:${messageRef}`;
  expect(body.prompt).toBe(expected);expect(body.media).toEqual([]);
  await completedPrompt(request,main.id,expected);
  expect((await messages(request,main.id)).filter(m=>m.role==='user').map(m=>m.content)).toContain(expected);
  await expect.poll(()=>storedDraft(page,main.id)).toMatchObject({text:'',pending:0});
  await expect(page.locator('.compose-file-pill')).toHaveCount(0);
  // Select all three reference types again, with no text or media at all.
  await page.locator(`#post-${messageId} .post-time`).click();await attachWorkspaceFile(page,request);
  await showWorkspace(page);await folder.click();await reference.click();await hideWorkspace(page);
  await expect(input).toHaveValue('');
  const only=page.waitForRequest(req=>req.method()==='POST'&&req.url().endsWith(`/api/sessions/${main.id}/prompt`));
  await input.press('Enter');const referencesOnly=(await only).postDataJSON();
  expect(referencesOnly.prompt).toBe(expected.slice(expected.indexOf('Files:')));
  await completedPrompt(request,main.id,referencesOnly.prompt);
  await expect.poll(()=>storedDraft(page,main.id)).toMatchObject({text:'',pending:0});
  const stored=await messages(request,main.id);expect(stored.filter(m=>m.role==='user')).toHaveLength(3);
  expect(stored.filter(m=>m.role==='user').at(-1).content).toBe(referencesOnly.prompt);
});

for(const method of ['attach','drop','paste'])test(`${method==='attach'?'@shared-39':'Gi'} durable attachment retry via ${method} survives source removal without duplicate media`,async({page,request},info)=>{
 const {mkdtempSync,writeFileSync,unlinkSync,existsSync,rmSync}=await import('node:fs'),{tmpdir}=await import('node:os'),{join}=await import('node:path');
 const {main,child,input,switchTo}=await fixture(page,request,info);await switchTo(child);await input.fill('source-owner draft');await switchTo(main.id);
 const base64=await page.evaluate(()=>{const c=document.createElement('canvas');c.width=73;c.height=29;c.getContext('2d').fillRect(0,0,73,29);return c.toDataURL('image/png').split(',')[1]});
 const raw=Buffer.from(base64,'base64'),dir=mkdtempSync(join(tmpdir(),'gi-source-')),path=join(dir,'durable.png');writeFileSync(path,raw);
 let release,held=false,prompts=0;const gate=new Promise(r=>release=r);
 await page.route(`**/api/sessions/${main.id}/media`,async route=>{const response=await route.fetch({postData:route.request().postDataBuffer()});expect(response.status()).toBe(201);held=true;await gate;try{await route.fulfill({response});}catch{}});
 page.on('request',r=>{if(r.method()==='POST'&&new URL(r.url()).pathname.endsWith('/prompt'))prompts++;});
 try{
  await input.fill('durable retry caption');
  if(method==='attach')await page.locator('.compose-box input[type=file]').setInputFiles(path);
  else await page.locator(method==='drop'?'.compose-input-wrapper':'.compose-box textarea').evaluate((el,{method,bytes})=>{const data=new DataTransfer();data.items.add(new File([new Uint8Array(bytes)],'durable.png',{type:'image/png'}));el.dispatchEvent(method==='drop'?new DragEvent('drop',{bubbles:true,cancelable:true,dataTransfer:data}):new ClipboardEvent('paste',{bubbles:true,cancelable:true,clipboardData:data}));},{method,bytes:[...raw]});
  await expect(page.locator('.compose-box').getByText('durable.png',{exact:true})).toBeVisible();await input.press('Enter');await expect.poll(()=>held).toBe(true);await expect(page.getByRole('progressbar',{name:'Attachment upload progress'})).toBeVisible();await expect(page.locator('.gi-compose-upload')).toContainText('Uploading');
  const before=(await(await request.get(`/api/sessions/${main.id}/media`)).json()).media;expect(before).toHaveLength(1);const id=before[0].id;
  await page.getByRole('button',{name:'Cancel uploads',exact:true}).click();await expect(input).toHaveValue('durable retry caption');await expect(page.getByRole('alert')).toContainText('Upload cancelled');await expect(page.locator('.gi-compose-transfer')).toBeHidden();expect(prompts).toBe(0);expect(await messages(request,main.id)).toEqual([]);
  release();await page.unroute(`**/api/sessions/${main.id}/media`);unlinkSync(path);expect(existsSync(path)).toBe(false);await page.reload();await expect(input).toHaveValue('durable retry caption');await expect(page.locator('.compose-box').getByText('durable.png',{exact:true})).toBeVisible();
  await input.press('Enter');await expect.poll(async()=> (await messages(request,main.id)).filter(m=>m.role==='user').length).toBe(1);
  const user=(await messages(request,main.id)).find(m=>m.role==='user');expect(user.payload.media).toHaveLength(1);expect(user.payload.media[0]).toMatchObject({media_id:id,session_id:main.id});expect(user.content).toBe(`durable retry caption\n\nAttachments:\n- attachment:${id} (durable.png)`);
  expect((await(await request.get(`/api/sessions/${main.id}/media`)).json()).media).toHaveLength(1);expect(prompts).toBe(1);
  const image=page.locator('.media-preview img');await expect(image).toHaveCount(1);await expect.poll(()=>image.evaluate(el=>el.naturalWidth)).toBe(73);
  await page.reload();await expect(image).toHaveCount(1);await expect.poll(()=>image.evaluate(el=>el.naturalWidth)).toBe(73);expect(await(await request.get(`/api/media/${id}/raw`)).body()).toEqual(raw);expect(existsSync(path)).toBe(false);
  await switchTo(child);await expect(input).toHaveValue('source-owner draft');expect(await messages(request,child)).toEqual([]);expect((await(await request.get(`/api/sessions/${child}/media`)).json()).media).toEqual([]);
  if(method==='attach')await info.attach('durable-media-proof',{body:JSON.stringify({id,sourceRemoved:true,bytes:raw.length,session:main.id}),contentType:'application/json'});
 }finally{release();rmSync(dir,{recursive:true,force:true});}
});
