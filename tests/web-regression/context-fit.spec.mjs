import {test,expect} from '@playwright/test';
import {mkdirSync,writeFileSync} from 'node:fs';
import {resolve} from 'node:path';

const inputName='Message (Enter to send, Shift+Enter for newline)...';
const file={name:'draft.txt',mimeType:'text/plain',buffer:Buffer.from('keep these bytes')};
async function fixture(page,request,info){
 const token=`context-fit-${info.project.name}-${Date.now()}`;
 const main=await(await request.post('/api/sessions',{data:{agent_id:token,title:token}})).json();
 const child=(await(await request.post(`/api/sessions/${main.id}/fork`,{data:{agent_id:token+'-child',title:token+'-child'}})).json()).branch.chat_jid.slice(3);
 await page.addInitScript(id=>{if(!localStorage.getItem('gi_session_id'))localStorage.setItem('gi_session_id',id);},main.id);
 await page.goto('/');const input=page.getByRole('textbox',{name:inputName,exact:true});await expect(input).toBeVisible();
 const state=async (id=main.id)=>(await(await request.get(`/api/sessions/${id}/model`)).json());
 expect((await state()).context_usage.tokens).toBeNull();
 const response=page.waitForResponse(r=>r.url().endsWith(`/api/sessions/${main.id}/prompt`)&&r.request().method()==='POST');
 await input.fill('Measure a real local-provider request');await input.press('Enter');const turn=await(await response).json();
 await expect.poll(async()=> (await state()).context_usage.tokens).toBe(100);
 await expect(page.locator('.compose-context-pie')).toHaveAttribute('aria-label','Context: 100 / 32K tokens (0%)\nCompact context',{timeout:15000});
 // Measurement provenance is exposed by the real model API, never seeded in DB.
 expect((await state()).context_usage.measurement).toMatchObject({turn_id:turn.turn_id,model:'ux-local/gate',iteration:1});
 const modelButton=page.getByRole('button',{name:'Open model picker',exact:true});const menu=page.getByRole('listbox',{name:'Models',exact:true});
 const option=name=>menu.getByRole('option').filter({hasText:name});
 const switchTo=async id=>{await page.getByRole('button',{name:/Manage sessions for/}).last().click();await page.locator(`[data-session-jid="gi:${id}"]`).getByRole('option').click();};
 return{main,child,input,state,turn,modelButton,menu,option,switchTo};
}

test('@gi-settings-006 Settings model fit uses measured native context and keeps unknowns selectable',async({page,request},info)=>{
 const{input,state,child,switchTo}=await fixture(page,request,info);
 await input.fill('settings measured draft');
 await page.keyboard.press('Control+,');
 const dialog=page.getByRole('dialog',{name:'Settings',exact:true});
 await dialog.getByRole('button',{name:'Models',exact:true}).click();
 const choice=dialog.getByLabel('Session model',{exact:true});await expect(choice).toBeVisible();
 await dialog.getByLabel('Filter models',{exact:true}).fill('ux-local/small');
 await choice.selectOption('ux-local/small');
 await expect(dialog.getByRole('button',{name:'Apply model'})).toBeDisabled();
 await expect(dialog.getByRole('status')).toContainText('cannot fit the measured context');
 expect((await state()).current).toBe('ux-local/gate');
 await dialog.getByLabel('Filter models',{exact:true}).fill('ux-local/equal');
 await choice.selectOption('ux-local/equal');await dialog.getByRole('button',{name:'Apply model'}).click();
 await expect(dialog.getByTestId('settings-current-model')).toHaveText('ux-local/equal');
 await page.keyboard.press('Escape');await expect(input).toHaveValue('settings measured draft');
 await expect(page.locator('.compose-context-pie')).toHaveAttribute('aria-label','Context: 100 / 100 tokens (100%)\nCompact context');
 await switchTo(child);expect((await state(child)).context_usage.tokens).toBeNull();
 await page.keyboard.press('Control+,');await dialog.getByRole('button',{name:'Models',exact:true}).click();
 await dialog.getByLabel('Filter models',{exact:true}).fill('ux-local/small');
 await choice.selectOption('ux-local/small');await expect(dialog.getByRole('button',{name:'Apply model'})).toBeEnabled();
 await dialog.getByRole('button',{name:'Apply model'}).click();await expect(dialog.getByTestId('settings-current-model')).toHaveText('ux-local/small');
});

if(process.env.GI_UX_SETTINGS_CATALOGUE){
 test('@gi-settings-004 Settings caps a real native catalogue and filters beyond the first page',async({page,request},info)=>{
  const{input,state}=await fixture(page,request,info);
  expect((await state()).model_options.length).toBeGreaterThan(50);
  await input.fill('bounded catalogue draft');await page.keyboard.press('Control+,');
  const dialog=page.getByRole('dialog',{name:'Settings',exact:true});await dialog.getByRole('button',{name:'Models',exact:true}).click();
  const select=dialog.getByLabel('Session model',{exact:true});await expect(select).toBeVisible();
  await expect(select.locator('option:not([disabled])')).toHaveCount(50);await expect(dialog.getByText(/Refine the filter/)).toBeVisible();
  await dialog.getByLabel('Filter models',{exact:true}).fill('settings-59');await expect(select.locator('option:not([disabled])')).toHaveCount(1);
  await expect(dialog.getByRole('button',{name:'Apply model'})).toBeDisabled();
  await select.selectOption('ux-local/settings-59');await dialog.getByRole('button',{name:'Apply model'}).click();
  await expect(dialog.getByTestId('settings-current-model')).toHaveText('ux-local/settings-59');
  await page.keyboard.press('Escape');await expect(input).toHaveValue('bounded catalogue draft');
 });
}
