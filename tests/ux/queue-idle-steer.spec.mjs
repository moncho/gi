import {test,expect} from '@playwright/test';
import {mkdirSync,writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
test('idle queue Steer starts only the selected held turn and preserves draft, metadata and paused siblings',async({page,request},info)=>{
 const token=`idle-steer-${info.project.name}-${Date.now()}`,gates=resolve('test-results/ux-parity/queue-gates');mkdirSync(gates,{recursive:true});
 const {id}=await(await request.post('/api/sessions',{data:{title:token,agent_id:token}})).json();
 const active=await(await request.post(`/api/sessions/${id}/prompt`,{data:{prompt:`UX queue gate:${token}`,model:'test-model'}})).json();
 const queue=async prompt=>{const r=await request.post(`/api/sessions/${id}/prompt`,{data:{prompt,intent:'queue',model:'test-model'}});expect(r.status()).toBe(202);return(await r.json()).turn_id;};
 const first=await queue('paused sibling first'),selected=await queue('selected idle instruction'),last=await queue('paused sibling last');
 await page.addInitScript(id=>localStorage.setItem('gi_session_id',id),id);await page.goto('/');const input=page.locator('.compose-box textarea');await input.fill('unsent draft survives');
 const activity=async()=>await(await request.get(`/api/sessions/${id}/activity`)).json();
 try{
  await page.getByRole('button',{name:'Stop response',exact:true}).click();await expect.poll(async()=>(await activity()).status).toBe('idle');
  await page.reload();await expect(input).toHaveValue('unsent draft survives');
  const row=page.locator(`[data-queue-id="${selected}"]`),steer=row.locator('.compose-queue-stack-steer-btn');await expect(steer).toBeEnabled();
  // Rejection must not consume the queued row or draft; retry is explicit.
  await page.route(`**/queue/${selected}/steer`,r=>r.fulfill({status:503,json:{error:'fixture unavailable'}}),{times:1});
  await steer.click();await expect(page.getByRole('alert').filter({hasText:'Queue action failed'})).toBeVisible();await expect(row).toBeVisible();await expect(input).toHaveValue('unsent draft survives');
  const sent=page.waitForRequest(r=>r.method()==='POST'&&r.url().endsWith(`/queue/${selected}/steer`));await steer.click();expect((await sent).postDataJSON()).toEqual({active_turn_id:''});
  await expect(row).toHaveCount(0);await expect.poll(async()=>((await(await request.get(`/api/sessions/${id}/turns`)).json()).turns.find(t=>t.id===selected)).status).toBe('completed');
  await expect.poll(async()=>(await activity()).status).toBe('idle');expect((await activity()).queue_hold_turn_id).toBe(active.turn_id);
  const turns=(await(await request.get(`/api/sessions/${id}/turns`)).json()).turns;expect(turns).toHaveLength(4);for(const other of [first,last])expect(turns.find(t=>t.id===other).status).toBe('queued');
  const messages=(await(await request.get(`/api/sessions/${id}/messages`)).json()).messages;expect(messages.filter(m=>m.role==='user'&&m.content==='selected idle instruction')).toHaveLength(1);
  expect((await request.post(`/api/sessions/${id}/queue/${selected}/steer`,{data:{active_turn_id:''}})).status()).toBe(409);
  await page.reload();await expect(input).toHaveValue('unsent draft survives');await expect(page.locator('.compose-queue-stack-item')).toHaveCount(2);
 }finally{writeFileSync(resolve(gates,token),'release');}
});
