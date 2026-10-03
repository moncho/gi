import {test,expect} from '@playwright/test';

const inputName='Message (Enter to send, Shift+Enter for newline)...';
async function swipe(target){
  await target.evaluate(el=>{
    for(const [name,x] of [['touchstart',190],['touchmove',85],['touchend',85]]){
      const touch={identifier:1,target:el,clientX:x,clientY:150},event=new Event(name,{bubbles:true,cancelable:true});
      Object.defineProperty(event,'touches',{value:name==='touchend'?[]:[touch]});
      Object.defineProperty(event,'changedTouches',{value:[touch]});el.dispatchEvent(event);
    }
  });
}

test('@ux-mobile-002 Stored-image attachment preview excludes a real modal gesture',async({page,request},info)=>{
  const id=`swipe-image-${info.project.name}-${Date.now()}`;
  const create=async suffix=>{const response=await request.post('/api/sessions',{data:{agent_id:`${id}-${suffix}`,title:`${id}-${suffix}`}});expect(response.status()).toBe(201);return(await response.json()).id;};
  const main=await create('main');await create('other');
  await page.addInitScript(id=>{localStorage.setItem('gi_session_id',id);Object.defineProperty(navigator,'userAgent',{configurable:true,value:'iPhone Safari'});},main);
  await page.goto('/');const input=page.getByRole('textbox',{name:inputName,exact:true});await expect(input).toBeVisible();
  const png=Buffer.from(await page.evaluate(()=>{const canvas=document.createElement('canvas');canvas.width=16;canvas.height=16;return canvas.toDataURL('image/png').split(',')[1]}),'base64');
  await input.fill('stored image swipe fixture');await page.locator('.compose-box input[type=file]').setInputFiles({name:'swipe-image.png',mimeType:'image/png',buffer:png});
  const sent=page.waitForRequest(r=>r.method()==='POST'&&r.url().endsWith(`/api/sessions/${main}/prompt`));await input.press('Enter');await sent;
  await expect.poll(async()=>((await(await request.get(`/api/sessions/${main}/turns`)).json()).turns||[]).some(turn=>turn.status==='completed')).toBe(true);
  const stored=(await(await request.get(`/api/sessions/${main}/messages`)).json()).messages.find(message=>message.role==='user');
  const post=page.locator(`#post-${stored.id}`);const image=post.locator('.media-preview img');await expect(image).toBeVisible();
  const preview=post.getByRole('button',{name:'Preview',exact:true});await expect(preview).toBeVisible();await preview.click();
  const modal=page.locator('.attachment-preview-modal');await expect(modal).toBeVisible();
  await input.fill('unsent modal draft');await swipe(modal.locator('img').first());await page.waitForTimeout(200);
  expect(await page.evaluate(()=>localStorage.getItem('gi_session_id'))).toBe(main);await expect(modal).toBeVisible();await expect(input).toHaveValue('unsent modal draft');
  await modal.getByRole('button',{name:'Close',exact:true}).click();await expect(modal).toHaveCount(0);
  await swipe(page.locator('.timeline').first());await expect.poll(()=>page.evaluate(()=>localStorage.getItem('gi_session_id'))).not.toBe(main);
});

test('@ux-mobile-002 Mounted controls do not enter the iOS chat carousel',async({page,request},info)=>{
  test.skip(!process.env.GI_UX_CARD_REJECTION,'requires isolated card fixture');

  const main='card-rejection-main',selected=()=>page.evaluate(()=>localStorage.getItem('gi_session_id'));
  await page.addInitScript(id=>{localStorage.setItem('gi_session_id',id);Object.defineProperty(navigator,'userAgent',{configurable:true,value:'iPhone Safari'});},main);
  const path=`swipe-excluded-${info.project.name}-${Date.now()}.md`;
  const written=await request.post('/api/tools/execute',{data:{tool:'write',input:{path,content:'# Swipe preview\nRetain this read-only tab'}}});expect(written.ok()).toBe(true);expect((await written.json()).error).toBeFalsy();
  await page.goto('/');const input=page.getByRole('textbox',{name:inputName,exact:true});await expect(input).toBeVisible();
  const card=page.locator(`#post-${main}-post .adaptive-card-container`),cardInput=card.getByRole('textbox',{name:'Card answer',exact:true});await expect(cardInput).toBeVisible();
  await input.fill('unsent swipe draft');const session=main;
  const noSwitch=async(target)=>{await expect(target).toBeVisible();await swipe(target);await page.waitForTimeout(200);expect(await selected()).toBe(session);if(await input.count())await expect(input).toHaveValue('unsent swipe draft');};
  await noSwitch(input);
  await noSwitch(cardInput);
  await noSwitch(card.getByRole('button',{name:'Submit answer',exact:true}));
  await page.getByTestId('hamburger').click();await page.getByRole('menuitem',{name:'Show workspace',exact:true}).click();
  const explorer=page.locator('.workspace-sidebar');await noSwitch(explorer);
  await page.locator(`.workspace-row[data-path="${path}"] .workspace-label-text`).click();
  await page.getByRole('button',{name:'Open read-only tab',exact:true}).click();
  const preview=page.getByRole('region',{name:`Read-only preview: ${path}`});await noSwitch(preview);
  await page.getByRole('button',{name:'Return to conversation',exact:true}).click();await expect(input).toHaveValue('unsent swipe draft');
  const pickerTrigger=page.locator('.compose-model-hint-btn');await pickerTrigger.click();
  const picker=page.locator('.compose-model-popup');await noSwitch(picker);
  await page.keyboard.press('Escape');
  // A positive control: the same event sequence on the mounted timeline must switch.
  await swipe(page.locator('.timeline').first());await expect.poll(selected).not.toBe(session);
});
