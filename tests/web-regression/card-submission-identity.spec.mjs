import {test,expect} from '@playwright/test';
test.skip(!process.env.GI_UX_CARD_IDENTITY,'Requires disposable native submission blocks; use make test-ux-card-identity');

test('Gi mounted card submission receipts require bounded identity (ux-extra-002)',async({page,request})=>{
 const id='card-identity-fixture';
 const before=(await(await request.get(`/api/sessions/${id}/messages`)).json()).messages;
 expect(before).toHaveLength(8);
 await page.addInitScript(id=>localStorage.setItem('gi_session_id',id),id);
 await page.goto('/');
 const input=page.locator('.compose-box textarea');await input.fill('unsent card identity draft');
 for(let i=0;i<8;i++){
  const post=page.locator(`#post-identity-message-${i}`);await expect(post).toBeVisible();
  const receipt=post.locator('.adaptive-card-submission-receipt');
  await expect(receipt).toHaveCount(i===0||i===7?1:0);
 }
 await expect(input).toHaveValue('unsent card identity draft');
 expect((await(await request.get(`/api/sessions/${id}/messages`)).json()).messages).toEqual(before);
 expect((await(await request.get(`/api/sessions/${id}/turns`)).json()).turns||[]).toEqual([]);
});
