import { test, expect } from '@playwright/test';

test.describe('Gi trusted touch menu dismissal',()=>{
 test.use({hasTouch:true});
 test('Gi touch outside the menu dismisses once without leaking a send or consuming the next gesture',async({page,request},info)=>{
  const token=`menu-touch-${info.project.name}-${Date.now()}`,created=await request.post('/api/sessions',{data:{agent_id:token,title:token}});expect(created.status()).toBe(201);const main=await created.json();
  await page.addInitScript(id=>localStorage.setItem('gi_session_id',id),main.id);await page.goto('/');
  const input=page.getByRole('textbox',{name:'Message (Enter to send, Shift+Enter for newline)...',exact:true}),trigger=page.getByTestId('hamburger'),menu=page.locator('.timeline-menu-dropdown'),send=page.getByRole('button',{name:'Send message',exact:true});
  await input.fill('trusted touch stays unsent');let submissions=0;page.on('request',r=>{if(r.method()==='POST'&&new URL(r.url()).pathname.endsWith('/prompt'))submissions++;});
  await page.locator('.compose-box').evaluate(el=>{window.__shellTouches=0;el.addEventListener('click',()=>window.__shellTouches++);});
  const tap=async target=>{const b=await target.boundingBox();await page.touchscreen.tap(b.x+b.width/2,b.y+b.height/2);};
  await tap(trigger);await expect(menu).toHaveCount(1);await tap(send);await expect(menu).toHaveCount(0);await expect(trigger).toBeFocused();expect(submissions).toBe(0);expect(await page.evaluate(()=>window.__shellTouches)).toBe(0);await expect(input).toHaveValue('trusted touch stays unsent');
  await tap(input);await expect(input).toBeFocused();expect(await page.evaluate(()=>window.__shellTouches)).toBe(1);
  // A cancelled pointer sequence must not close/arm a later unrelated action.
  // This is a synthetic cancel-path check, distinct from the trusted tap above.
  await tap(trigger);await expect(menu).toHaveCount(1);await send.dispatchEvent('pointerdown',{pointerId:9,pointerType:'touch',isPrimary:true});await send.dispatchEvent('pointercancel',{pointerId:9,pointerType:'touch',isPrimary:true});await expect(menu).toHaveCount(1);
  await tap(send);await expect(menu).toHaveCount(0);expect(submissions).toBe(0);await tap(input);expect(await page.evaluate(()=>window.__shellTouches)).toBe(2);
  expect((await(await request.get(`/api/sessions/${main.id}/turns`)).json()).turns||[]).toEqual([]);
 });
});
