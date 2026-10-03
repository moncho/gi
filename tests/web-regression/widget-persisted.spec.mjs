import {test,expect} from '@playwright/test';
test.skip(!process.env.GI_UX_WIDGETS,'Requires disposable native widget messages; use make test-ux-widget-persisted');

test('Gi mounted persisted widget subset (ux-extra-004): empty versus populated HTML/SVG artifacts',async({page,request})=>{
 const id='widget-fixture';
 const before=(await(await request.get(`/api/sessions/${id}/messages`)).json()).messages;
 expect(before).toHaveLength(4);
 const expected=[false,true,false,true];
 for(let i=0;i<expected.length;i++){
  expect(before[i].payload.content_blocks[0].artifact.kind).toBe(i<2?'html':'svg');
  expect(Boolean(before[i].payload.content_blocks[0].artifact[i<2?'html':'svg'])).toBe(expected[i]);
 }
 await page.addInitScript(id=>localStorage.setItem('gi_session_id',id),id);
 await page.goto('/');
 const draft=page.locator('.compose-box textarea');await draft.fill('unsent persisted widget draft');
 for(let i=0;i<expected.length;i++){
  const post=page.locator(`#post-widget-message-${i}`);await expect(post).toBeVisible();
  const button=post.getByRole('button',{name:'Open widget'});await expect(button).toBeVisible();
  if(!expected[i]){await expect(button).toBeDisabled();continue;}
  await expect(button).toBeEnabled();await button.click();
  const pane=page.locator('.floating-widget-pane');await expect(pane).toBeVisible();
  const frame=pane.locator('iframe.floating-widget-frame');await expect(frame).toBeVisible();
  await expect(frame).toHaveAttribute('srcdoc',new RegExp(i===1?'Stored HTML proof':'Stored SVG proof'));
  await pane.getByRole('button',{name:'Close widget'}).click();await expect(pane).toBeHidden();
 }
 await expect(draft).toHaveValue('unsent persisted widget draft');
 expect((await(await request.get(`/api/sessions/${id}/messages`)).json()).messages).toEqual(before);
 expect((await(await request.get(`/api/sessions/${id}/turns`)).json()).turns||[]).toEqual([]);
});
