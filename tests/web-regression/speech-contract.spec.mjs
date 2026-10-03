import {test,expect} from '@playwright/test';

import {speechRuntime,fixture} from '../ux/support/speech-fixture.mjs';

test.skip(!process.env.GI_UX_SPEECH,'Requires isolated native empty-assistant seed; use make test-ux-speech-contract');

async function emptyGate(context,request){
 const messages=(await(await request.get('/api/sessions/speech-empty-fixture/messages')).json()).messages;
 expect(messages).toHaveLength(2);for(const m of messages){expect(m.role).toBe('assistant');expect(m.content.trim()).toBe('');}
 const page=await context.newPage();try{
  await speechRuntime(page);await page.addInitScript(()=>localStorage.setItem('gi_session_id','speech-empty-fixture'));await page.goto('/');
  for(const m of messages){const post=page.locator(`[id="post-${m.id}"]`);await expect(post).toBeVisible();await expect(post.locator('.post-speak-btn')).toHaveCount(0);}
  expect((await(await request.get('/api/sessions/speech-empty-fixture/messages')).json()).messages).toEqual(messages);
 }finally{await page.close();}
}
async function capability(page,request,context,info){
 await emptyGate(context,request);await speechRuntime(page,false);const f=await fixture(page,request,info);
 for(const m of f.agents)await expect(f.post(m.id)).toBeVisible();await expect(page.locator('.post-speak-btn')).toHaveCount(0);
 await page.evaluate(()=>sessionStorage.setItem('speech-fixture-enabled','true'));await page.reload();
 // Scope to the two fixture assistant posts. A speakable queued-prompt system
 // notice can also be projected as an agent response during native admission.
 for(const m of f.agents)await expect(f.post(m.id).getByRole('button',{name:'Read aloud',exact:true})).toHaveCount(1);
 for(const m of f.messages.filter(m=>m.role==='user'))await expect(f.post(m.id).locator('.post-speak-btn')).toHaveCount(0);
 await expect(f.input).toHaveValue('unsent speech draft β');await expect(page.locator('.compose-box')).toContainText('speech-ref.txt');
 return f;
}

test('@ux-timeline-027 assistant speech controls require native text and supported browser APIs',async({page,request,context},info)=>{
 await capability(page,request,context,info);
});
