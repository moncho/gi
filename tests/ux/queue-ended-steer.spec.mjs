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

// Fixture-owned cancelling claim models the window before worker cleanup.
// The real HTTP handler must reject without consuming the row, then accept
// the same selected ID once release makes the observed run terminal.
test('Steer during cancelling claim keeps selected row for retry after release',async({page,request},info)=>{
 const env=await journeyEnvironment(info);
 try{
  await page.goto(env.origin);const input=page.locator('.compose-box textarea');await expect(input).toBeVisible();await input.fill('keep draft during cleanup Ω');
  const id=await page.evaluate(()=>localStorage.getItem('gi_session_id'));
  expect((await request.post(`${env.origin}/__test/ended-steer/${id}/seed?media=1`)).status()).toBe(204);
  const turns=async()=>(await(await request.get(`${env.origin}/api/sessions/${id}/turns`)).json()).turns;
  const queuedMedia=(await turns()).find(t=>t.id==='selected').metadata.media;
  expect(queuedMedia).toHaveLength(1);expect(queuedMedia[0].filename).toBe('selected.txt');
  const attachmentURL=`${env.origin}/api/media/${queuedMedia[0].media_id}/raw`;
  const storedBytes=async()=>{const res=await request.get(attachmentURL);expect(res.status()).toBe(200);expect(res.headers()['content-type']).toContain('text/plain');return (await res.body()).toString('utf8')};
  expect(await storedBytes()).toBe('selected media bytes');
  await page.reload();const row=page.locator('[data-queue-id="selected"]');const button=row.getByRole('button',{name:'Inject queued follow-up as steer',exact:true});await expect(button).toBeEnabled();
  expect((await request.post(`${env.origin}/__test/ended-steer/${id}/cancel`)).status()).toBe(204);
  const url=`${env.origin}/api/sessions/${id}/queue/selected/steer`,body={active_turn_id:'observed'};
  const conflict=page.waitForResponse(r=>r.url()===url&&r.request().method()==='POST');await button.click();expect((await conflict).status()).toBe(409);
  expect((await turns()).find(t=>t.id==='selected').status).toBe('queued');
  expect((await turns()).find(t=>t.id==='selected').metadata.media).toEqual(queuedMedia);
  expect(await storedBytes()).toBe('selected media bytes');
  await expect(row).toHaveCount(1);await expect(button).toBeDisabled();await expect(input).toHaveValue('keep draft during cleanup Ω');
  await expect(page.getByRole('alert')).toContainText('Queue action failed');
  expect((await request.post(`${env.origin}/__test/ended-steer/${id}/release-cancelled`)).status()).toBe(204);
  // This fixture changes SQLite without emitting a completion event. The UI
  // should regain the retry affordance on its 10s selected-state safety poll.
  await expect(button).toBeEnabled({timeout:16000});
  const retried=page.waitForResponse(r=>r.url()===url&&r.request().method()==='POST');await button.click();expect((await retried).status()).toBe(200);
  await expect.poll(async()=>(await turns()).find(t=>t.id==='selected')?.status).toBe('completed');
  await expect(row).toHaveCount(0);expect((await request.post(url,{data:body})).status()).toBe(409);
  const messages=(await(await request.get(`${env.origin}/api/sessions/${id}/messages`)).json()).messages;
  const delivered=messages.filter(m=>m.role==='user'&&m.content==='ended steer selected instruction');
  expect(delivered).toHaveLength(1);expect(delivered[0].payload.media).toEqual(queuedMedia);
  expect(await storedBytes()).toBe('selected media bytes');
  await page.reload();await expect(input).toHaveValue('keep draft during cleanup Ω');await expect(row).toHaveCount(0);
 }finally{await env.close()}
});
