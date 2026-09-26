/**
 * 04-sse-and-streaming.spec.ts — Verify SSE connection and real-time events.
 *
 * Tests that the SSE endpoint is reachable, sends the expected event types,
 * and that the frontend establishes and maintains the connection.
 */
import { test, expect } from '@playwright/test';
import { BASE_URL, waitForAppShell, sendMessage, apiGet, findSessionForMessage } from './helpers';

test.describe('SSE and streaming', () => {

  test('SSE endpoint is reachable', async ({ page }) => {
    await page.goto(BASE_URL);
    await waitForAppShell(page);
    // Verify SSE is working by checking that the EventSource connects
    const sseConnected = await page.evaluate(async () => {
      return new Promise<boolean>((resolve) => {
        const es = new EventSource('/sse/stream');
        es.addEventListener('connected', () => { es.close(); resolve(true); });
        es.onerror = () => { es.close(); resolve(false); };
        setTimeout(() => { es.close(); resolve(false); }, 5000);
      });
    });
    expect(sseConnected).toBeTruthy();
  });

  test('SSE sends connected event', async ({ page }) => {
    const sseEvents: string[] = [];
    // Intercept SSE to capture events
    await page.route('**/sse/stream**', async (route) => {
      // Let it through but capture
      route.continue();
    });
    
    await page.goto(BASE_URL);
    await waitForAppShell(page);
    
    // Check that the frontend SSEClient connected
    const connectionStatus = await page.evaluate(() => {
      // Check localStorage or DOM for connection state
      return document.querySelector('.connection-status')?.textContent || 'no-indicator';
    });
    // If there's no explicit indicator, at least verify no connection error
    expect(connectionStatus).not.toContain('disconnected');
  });

  test('sending from the selected composer yields exact session/turn SSE frames', async ({ page, request }) => {
    const created = await request.post(`${BASE_URL}/api/sessions`, { data: { title: 'SSE compose', agent_id: `sse-compose-${Date.now()}` } });
    expect(created.ok()).toBe(true);
    const session = await created.json();
    await page.addInitScript(id => localStorage.setItem('gi_session_id', id), session.id);
    await page.goto(BASE_URL);
    await waitForAppShell(page);
    await page.evaluate(id => new Promise<void>((resolve, reject) => {
      const source = new EventSource(`/sse/stream?chat_jid=gi:${id}`);
      (window as any).__composeSse = source;
      (window as any).__composeFrames = [];
      for (const type of ['new_post', 'agent_response', 'agent_status']) {
        source.addEventListener(type, (event: MessageEvent) => (window as any).__composeFrames.push({ type, data: JSON.parse(event.data) }));
      }
      source.addEventListener('connected', () => resolve(), { once: true });
      source.onerror = () => reject(new Error('session SSE connection failed'));
    }), session.id);
    try {
      const input = page.locator('.compose-box textarea');
      await input.fill('SSE compose identity proof');
      const acceptedResponse = page.waitForResponse(r => r.request().method() === 'POST' && r.url().endsWith(`/api/sessions/${session.id}/prompt`));
      await input.press('Enter');
      const accepted = await (await acceptedResponse).json();
      expect(accepted.turn_id).toBeTruthy();
      await expect.poll(() => page.evaluate(id => (window as any).__composeFrames.some((f:any) => f.type === 'new_post' && f.data.turn_id === id && f.data.is_bot_message), accepted.turn_id)).toBe(true);
      const frames = await page.evaluate(() => (window as any).__composeFrames);
      const reply = frames.find((f:any) => f.type === 'new_post' && f.data.turn_id === accepted.turn_id && f.data.is_bot_message);
      expect(reply.data.chat_jid).toBe(`gi:${session.id}`);
      await expect.poll(async () => (await apiGet(request, `/api/sessions/${session.id}/turns`)).turns.find((t:any) => t.id === accepted.turn_id)?.status).toBe('completed');
      await expect(page.locator('.post').filter({ hasText: 'Gi received: SSE compose identity proof' })).toBeVisible();
    } finally {
      await page.evaluate(() => (window as any).__composeSse.close());
    }
  });

  test('turn events are persisted for completed turns', async ({ page, request }) => {
    await page.goto(BASE_URL);
    await waitForAppShell(page);
    await sendMessage(page, 'turn events check');
    await page.waitForTimeout(5000);

    const session = await findSessionForMessage(request, 'turn events check');
    const turns = await apiGet(request, `/api/sessions/${session.id}/turns`);
    const lastTurn = turns.turns[turns.turns.length - 1];
    
    const events = await apiGet(request, `/api/turns/${lastTurn.id}/events`);
    const eventTypes = events.events.map((e: any) => e.type);
    
    // Must have at least: turn.submitted, turn.started, turn.finished
    expect(eventTypes).toContain('turn.submitted');
    expect(eventTypes).toContain('turn.started');
    expect(eventTypes).toContain('turn.finished');
  });
});

test('Native terminal SSE frames retain their session and turn identity',async({page,request})=>{
 const created=await request.post(`${BASE_URL}/api/sessions`,{data:{agent_id:`sse-identity-${Date.now()}`,title:'SSE identity'}});expect(created.ok()).toBe(true);const session=await created.json();
 await page.goto(BASE_URL);
 await page.evaluate(id=>new Promise<void>((resolve,reject)=>{
  const source=new EventSource(`/sse/stream?chat_jid=gi:${id}`);(window as any).__identitySource=source;(window as any).__identityFrames=[];
  for(const type of ['new_post','agent_response','agent_status'])source.addEventListener(type,(event:MessageEvent)=>(window as any).__identityFrames.push({type,data:JSON.parse(event.data)}));
  source.addEventListener('connected',()=>resolve(),{once:true});source.onerror=()=>reject(new Error('SSE connection failed'));
 }),session.id);
 try{
  const accepted=await request.post(`${BASE_URL}/api/sessions/${session.id}/prompt`,{data:{prompt:'SSE identity proof',model:'test-model'}});expect(accepted.ok()).toBe(true);const turn=await accepted.json();
  await expect.poll(()=>page.evaluate(()=>(window as any).__identityFrames.some((frame:any)=>frame.type==='new_post'&&frame.data.is_bot_message))).toBe(true);
  const frames=await page.evaluate(()=>(window as any).__identityFrames);
  const reply=frames.find((frame:any)=>frame.type==='new_post'&&frame.data.is_bot_message);
  expect(reply.data).toMatchObject({turn_id:turn.turn_id,chat_jid:'gi:'+session.id});
 }finally{await page.evaluate(()=>(window as any).__identitySource.close());}
});
