import {test,expect} from '@playwright/test';

test('retry snapshot is active, visible and not hidden by the last completed tool',async({page,request})=>{
 const response=await request.post('/api/sessions',{data:{agent_id:'retry-fixture',title:'Retry snapshot'}});expect(response.ok()).toBe(true);const {id}=await response.json();await page.addInitScript(id=>localStorage.setItem('gi_session_id',id),id);
 const title='Provider request timed out — retrying (attempt 1/3, 2s delay)';
 let active=true;
 await page.route(`**/api/sessions/${id}/activity`,route=>route.fulfill({json:active?{status:'running',phase:'retry_wait',turn_id:'retry-fixture',tool:{tool_call_id:'previous-tool',name:'shell',status:'completed',duration_ms:0},retry:{title,attempt:1,max_attempts:3,phase:'retry_wait',retry_at:new Date(Date.now()+2000).toISOString(),failure_category:'timeout'}}:{status:'idle',phase:'completed',turn_id:'retry-fixture'}}));
 await page.goto('/');await expect(page.getByText(title,{exact:false})).toBeVisible();await expect(page.locator('.gi-tool-activity')).toHaveCount(0);
 const input=page.locator('.compose-box textarea');await input.fill('new draft during retry');await page.reload();await expect(page.getByText(title,{exact:false})).toBeVisible();await expect(input).toHaveValue('new draft during retry');
 active=false;await page.reload();await expect(page.getByText(title,{exact:false})).toHaveCount(0);await expect(input).toHaveValue('new draft during retry');
});
