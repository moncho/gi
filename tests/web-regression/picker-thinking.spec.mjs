import {test,expect} from '@playwright/test';
import {journeyEnvironment} from '../ux/support/journey-environment.mjs';
test('composer thinking picker commits explicit choice, retains draft, rejects failure and captures next request',async({page,request},info)=>{
 const env=await journeyEnvironment(info);let unblock;
 try{
  await page.goto(env.origin);const input=page.locator('.compose-box textarea');await expect(input).toBeFocused();await input.fill('unsent thinking draft');
  const id=await page.evaluate(()=>localStorage.getItem('gi_session_id'));await request.patch(`${env.origin}/api/sessions/${id}/model`,{data:{model:'ux-local/reasoner'}});await page.reload();await expect(input).toHaveValue('unsent thinking draft');
  const trigger=page.getByRole('button',{name:'Open model picker',exact:true});await trigger.click();const select=page.getByRole('combobox',{name:'Thinking level',exact:true});await expect(select).toBeEnabled();await expect(select.locator('option')).toHaveText(['Provider default','low','high']);
  let calls=0;const gate=new Promise(r=>unblock=r);await page.route('**/api/sessions/*/model',async r=>{if(r.request().method()==='PATCH'){calls++;await gate;}await r.continue();});
  await select.focus();await select.selectOption('high');await expect(select).toBeDisabled();await expect(page.getByRole('listbox',{name:'Models'})).toBeVisible();expect(calls).toBe(1);unblock();await expect(select).toBeEnabled();await expect(select).toHaveValue('high');await expect(select).toBeFocused();await expect(input).toHaveValue('unsent thinking draft');
  await page.unroute('**/api/sessions/*/model');
  await page.route('**/api/sessions/*/model',r=>r.request().method()==='PATCH'?r.fulfill({status:503,json:{error:'fixture rejected'}}):r.continue());
  await select.focus();await select.selectOption('low');await expect(page.getByRole('alert')).toContainText('Thinking selection failed');await expect(select).toHaveValue('high');await expect(select).toBeEnabled();await expect(select).toBeFocused();
  await page.unroute('**/api/sessions/*/model');await select.selectOption('');await expect(select).toHaveValue('');await expect.poll(async()=>(await(await request.get(`${env.origin}/api/sessions/${id}/model`)).json()).thinking_level).toBe('');
  await select.selectOption('high');await expect.poll(async()=>(await(await request.get(`${env.origin}/api/sessions/${id}/model`)).json()).thinking_level).toBe('high');
  await page.keyboard.press('Escape');await expect(trigger).toBeFocused();await expect(input).toHaveValue('unsent thinking draft');expect((await(await request.get(`${env.origin}/api/sessions/${id}/turns`)).json()).turns||[]).toHaveLength(0);
  await page.reload();await trigger.click();await expect(select).toHaveValue('high');await page.keyboard.press('Escape');
  const admitted=page.waitForResponse(r=>r.request().method()==='POST'&&r.url().endsWith(`/api/sessions/${id}/prompt`));await input.press('Enter');const turn=(await(await admitted).json()).turn_id;
  await expect.poll(async()=>(await(await request.get(`${env.origin}/api/sessions/${id}/turns`)).json()).turns.find(t=>t.id===turn)?.status).toBe('completed');
  const messages=(await(await request.get(`${env.origin}/api/sessions/${id}/messages`)).json()).messages;expect(messages.filter(m=>m.role==='assistant').at(-1).content).toContain('Provider model reasoner thinking high:');
 }finally{unblock?.();await env.close();}
});

for(const owner of ['composer','Settings'])test(`thinking response respects newer ${owner} focus`,async({page},info)=>{
 const env=await journeyEnvironment(info);let release;
 try{
  await page.goto(env.origin);const input=page.locator('.compose-box textarea');await expect(input).toBeVisible();await input.fill('focus retained');await page.getByRole('button',{name:'Open model picker',exact:true}).click();
  const select=page.getByRole('combobox',{name:'Thinking level',exact:true});await expect(select).toBeEnabled();
  const gate=new Promise(r=>release=r);let held=false;
  await page.route('**/api/sessions/*/model',async r=>{if(r.request().method()==='PATCH'){held=true;await gate;}await r.continue();});
  await select.focus();await select.selectOption('high');await expect.poll(()=>held).toBe(true);
  if(owner==='Settings'){await page.keyboard.press('Control+,');await expect(page.getByRole('dialog')).toBeVisible();}else await input.focus();
  const focused=await page.evaluateHandle(()=>document.activeElement);release();await expect(select).toHaveValue('high');await expect(select).toBeEnabled();await expect.poll(()=>focused.evaluate(e=>document.activeElement===e)).toBe(true);await expect(input).toHaveValue('focus retained');
 }finally{release?.();await env.close();}
});

test('stale thinking response cannot replace another session state or its draft',async({page,request},info)=>{
 const env=await journeyEnvironment(info);let release;
 try{
  await page.goto(env.origin);const input=page.locator('.compose-box textarea');await expect(input).toBeVisible();const id=await page.evaluate(()=>localStorage.getItem('gi_session_id'));
  const other=await(await request.post(`${env.origin}/api/sessions`,{data:{title:'Other thinking session',agent_id:'other-thinking'}})).json();await input.fill('source thinking draft');
  await page.getByRole('button',{name:'Open model picker',exact:true}).click();const select=page.getByRole('combobox',{name:'Thinking level',exact:true});await expect(select).toBeEnabled();
  const gate=new Promise(r=>release=r);let held=false;
  await page.route(`**/api/sessions/${id}/model`,async r=>{if(r.request().method()==='PATCH'){const response=await r.fetch();held=true;await gate;return r.fulfill({response});}await r.continue();});
  await select.selectOption('high');await expect.poll(()=>held).toBe(true);if(page.viewportSize().width<=639)await page.getByRole('button',{name:'Close model picker',exact:true}).click();
  await page.getByRole('button',{name:/Manage sessions for/}).last().click();await page.locator(`[data-session-jid="gi:${other.id}"]`).getByRole('menuitem').click();await input.fill('target thinking draft');release();
  await page.getByRole('button',{name:'Open model picker',exact:true}).click();await expect(select).toBeEnabled();await expect(select).toHaveValue('');await expect(input).toHaveValue('target thinking draft');
  expect((await(await request.get(`${env.origin}/api/sessions/${id}/model`)).json()).thinking_level).toBe('high');expect((await(await request.get(`${env.origin}/api/sessions/${other.id}/model`)).json()).thinking_level).toBe('');
 }finally{release?.();await env.close();}
});

test('thinking picker stale token rejects without replay and refreshes authoritative level',async({page,request},info)=>{
 const env=await journeyEnvironment(info);
 try{
  await page.goto(env.origin);const input=page.locator('.compose-box textarea');await expect(input).toBeVisible();await input.fill('stale thinking draft');const id=await page.evaluate(()=>localStorage.getItem('gi_session_id'));
  await page.getByRole('button',{name:'Open model picker',exact:true}).click();const select=page.getByRole('combobox',{name:'Thinking level',exact:true});await expect(select).toBeEnabled();
  const snapshot=(await(await request.get(`${env.origin}/api/sessions/${id}/model`)).json());
  const external=await request.patch(`${env.origin}/api/sessions/${id}/model`,{data:{model:snapshot.current,thinking_level:'low',thinking_token:snapshot.thinking_token}});expect(external.ok()).toBe(true);
  // The mounted picker owns its last fetched token, not the API client's update.
  let attempts=0;page.on('request',r=>{if(r.method()==='PATCH'&&r.url().endsWith(`/api/sessions/${id}/model`))attempts++;});
  const response=page.waitForResponse(r=>r.request().method()==='PATCH'&&r.url().endsWith(`/api/sessions/${id}/model`));await select.selectOption('high');expect((await response).status()).toBe(409);
  await expect(page.getByRole('alert')).toContainText('Thinking selection failed');await expect(select).toHaveValue('low');await expect(select).toBeEnabled();expect(attempts).toBe(1);
  expect((await(await request.get(`${env.origin}/api/sessions/${id}/model`)).json()).thinking_level).toBe('low');await expect(input).toHaveValue('stale thinking draft');expect((await(await request.get(`${env.origin}/api/sessions/${id}/turns`)).json()).turns||[]).toHaveLength(0);
 }finally{await env.close();}
});
