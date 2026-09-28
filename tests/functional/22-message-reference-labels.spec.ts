import {test,expect,chromium,webkit} from '@playwright/test';
import {BASE_URL} from './helpers';
for(const [name,type]of Object.entries({chromium,webkit}))test(`${name} short reference labels preserve canonical draft and submitted IDs`,async()=>{
 const browser=await type.launch({headless:true}),page=await browser.newPage(),api=page.request;
 try{
  const {id}=await(await api.post(BASE_URL+'/api/sessions',{data:{title:'Reference label '+name}})).json();
  await api.post(BASE_URL+`/api/sessions/${id}/prompt`,{data:{prompt:'reference history',model:'test-model'}});
  let messages:any[]=[];await expect.poll(async()=>{messages=(await(await api.get(BASE_URL+`/api/sessions/${id}/messages?view=conversation&limit=50`)).json()).messages;return messages.length}).toBe(2);
  const message=messages.find(m=>m.role==='assistant');expect(message.display_row_id).toBeGreaterThan(0);expect(message.id.length).toBeGreaterThan(14);
  await page.addInitScript(id=>localStorage.setItem('gi_session_id',id),id);await page.goto(BASE_URL);
  await page.locator(`#post-${message.id} .post-time`).click();
  const pill=page.locator('.compose-box .compose-file-pill').filter({hasText:`msg:${message.display_row_id}`});
  await expect(pill).toBeVisible();await expect(pill).toHaveAttribute('title',`Message reference: ${message.id}`);
  const input=page.locator('.compose-box textarea');await input.fill('referenced draft');await page.reload();await expect(input).toHaveValue('referenced draft');await expect(pill).toBeVisible();
  const sent=page.waitForRequest(r=>r.method()==='POST'&&new URL(r.url()).pathname===`/api/sessions/${id}/prompt`);
  await input.press('Enter');const body=(await sent).postDataJSON();expect(body.prompt).toContain(`- message:${message.id}`);expect(body.prompt).not.toContain(`- message:${message.display_row_id}\n`);
  await expect(pill).toHaveCount(0);await expect(input).toHaveValue('');
 }finally{await browser.close();}
});
