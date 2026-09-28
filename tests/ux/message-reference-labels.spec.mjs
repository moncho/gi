import {test,expect} from '@playwright/test';
import {mkdirSync,writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
test('compact message references preserve queued content, return, removal and full canonical identity',async({page,request},info)=>{
 const token=`reference-${info.project.name}-${Date.now()}`,gates=resolve('test-results/ux-parity/queue-gates');mkdirSync(gates,{recursive:true});
 const {id}=await(await request.post('/api/sessions',{data:{title:token,agent_id:token}})).json();
 const submit=prompt=>request.post(`/api/sessions/${id}/prompt`,{data:{prompt,model:'test-model'}});
 await submit('reference history');let messages=[];
 await expect.poll(async()=>{messages=(await(await request.get(`/api/sessions/${id}/messages?view=conversation&limit=50`)).json()).messages;return messages.length}).toBe(2);
 const message=messages.find(m=>m.role==='assistant');
 await page.addInitScript(id=>localStorage.setItem('gi_session_id',id),id);await page.goto('/');
 const input=page.locator('.compose-box textarea');await expect(input).toBeVisible();
 try{
  await submit(`UX queue gate:${token}`);await expect(page.getByRole('button',{name:'Stop response',exact:true})).toBeVisible();
  const unknown='msg_0000000000000999999';
  const content=`queued reference\n\nReferenced messages:\n- message:${message.id}\n- message:${unknown}`;
  const queued=await request.post(`/api/sessions/${id}/prompt`,{data:{prompt:content,intent:'queue',model:'test-model'}});expect(queued.status()).toBe(202);
  const row=page.locator('.compose-queue-stack-item').filter({hasText:'queued reference'});
  await expect(row).toContainText(`msg:${message.display_row_id}`);await expect(row).toContainText('msg:msg_…999999');
  await expect(row.locator(`[title="Message reference: ${unknown}"]`)).toBeVisible();
  await page.reload();await expect(row).toContainText(`msg:${message.display_row_id}`);
  const snapshot=await(await request.get(`/api/sessions/${id}/queue`)).json();expect(snapshot.items.some(item=>item.prompt===content)).toBe(true);
  await row.getByRole('button',{name:/Return/}).click();await expect(input).toHaveValue('queued reference');await expect(row).toHaveCount(0);
  const pill=page.locator('.compose-box .compose-file-pill').filter({hasText:`msg:${message.display_row_id}`});await expect(pill).toHaveAttribute('title',`Message reference: ${message.id}`);
  await pill.getByRole('button').click();await expect(pill).toHaveCount(0);
  const fallback=page.locator(`.compose-box [title="Message reference: ${unknown}"]`);await expect(fallback).toBeVisible();
  expect((await(await request.get(`/api/sessions/${id}/queue`)).json()).items).toHaveLength(0);
 }finally{writeFileSync(resolve(gates,token),'release');await expect.poll(async()=>(await(await request.get(`/api/sessions/${id}/activity`)).json()).status).toBe('idle');}
});
