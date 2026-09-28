import {test,expect} from '@playwright/test';
import {journeyEnvironment} from './support/journey-environment.mjs';

test('observed run ending before Steer admission launches selected row once and keeps draft',async({page,request},info)=>{
 const env=await journeyEnvironment(info);let release;
 try{
  await page.goto(env.origin);const input=page.locator('.compose-box textarea');await expect(input).toBeVisible();await input.fill('ended race draft Ω');
  const id=await page.evaluate(()=>localStorage.getItem('gi_session_id'));
  expect((await request.post(`${env.origin}/__test/ended-steer/${id}/seed`)).status()).toBe(204);
  await page.reload();const row=page.locator('[data-queue-id="selected"]');const button=row.getByRole('button',{name:'Inject queued follow-up as steer',exact:true});await expect(button).toBeEnabled();
  const url=`/api/sessions/${id}/queue/selected/steer`;let held=false,calls=0,body;const gate=new Promise(r=>release=r);
  await page.route(`**${url}`,async route=>{calls++;body=route.request().postDataJSON();held=true;await gate;await route.continue()});
  const reply=page.waitForResponse(r=>r.url().endsWith(url));await button.click();await expect.poll(()=>held).toBe(true);expect(body).toEqual({active_turn_id:'observed'});await expect(button).toBeDisabled();
  expect((await request.post(`${env.origin}/__test/ended-steer/${id}/end`)).status()).toBe(204);release();expect((await reply).status()).toBe(200);
  await expect(row).toHaveCount(0);await expect(input).toHaveValue('ended race draft Ω');expect(calls).toBe(1);
  const turns=async()=>(await(await request.get(`${env.origin}/api/sessions/${id}/turns`)).json()).turns;
  await expect.poll(async()=>(await turns()).find(t=>t.id==='selected')?.status).toBe('completed');expect(await turns()).toHaveLength(2);
  expect((await request.post(`${env.origin}${url}`,{data:body})).status()).toBe(409);
  const messages=(await(await request.get(`${env.origin}/api/sessions/${id}/messages`)).json()).messages;
  expect(messages.filter(m=>m.role==='user'&&m.content==='ended steer selected instruction')).toHaveLength(1);
  expect(messages.filter(m=>m.role==='assistant').at(-1).content).toContain('ended steer selected instruction');
  await page.reload();await expect(input).toHaveValue('ended race draft Ω');await expect(row).toHaveCount(0);
 }finally{release?.();await env.close()}
});
