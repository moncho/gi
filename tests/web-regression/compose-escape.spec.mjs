import {test,expect} from '@playwright/test';
import {journeyEnvironment} from '../ux/support/journey-environment.mjs';

async function setup(page,info){
 const env=await journeyEnvironment(info);await page.goto(env.origin);const input=page.locator('.compose-box textarea');await expect(input).toBeVisible();return {...env,input};
}

test('plain Escape blurs composer without sending or losing draft and media',async({page},info)=>{
 const h=await setup(page,info);try{
  let sends=0;page.on('request',r=>{if(r.method()==='POST'&&r.url().endsWith('/prompt'))sends++});
  await h.input.fill('Escape draft Ω');await page.locator('.compose-box input[type=file]').setInputFiles({name:'escape.txt',mimeType:'text/plain',buffer:Buffer.from('escape retained file')});
  const pill=page.locator('.compose-file-pill[title="escape.txt"]');await expect(pill).toBeVisible();await h.input.focus();await h.input.press('Escape');await expect(h.input).not.toBeFocused();await expect(h.input).toHaveValue('Escape draft Ω');await expect(pill).toBeVisible();expect(sends).toBe(0);
  await h.input.focus();await h.input.press('End');await h.input.press('!');await expect(h.input).toHaveValue('Escape draft Ω!');await h.input.press('Escape');await expect(h.input).not.toBeFocused();
  // Persistence is asynchronous. Immediate reload before commit is a separate
  // retained failure; this checks restoration of the committed Escape draft.
  await expect.poll(()=>page.evaluate(async()=>{const db=await new Promise((resolve,reject)=>{const r=indexedDB.open('gi-session-drafts',2);r.onsuccess=()=>resolve(r.result);r.onerror=()=>reject(r.error)});try{return await new Promise((resolve,reject)=>{const r=db.transaction('drafts').objectStore('drafts').get(localStorage.getItem('gi_session_id'));r.onsuccess=()=>resolve(r.result?.draft.text);r.onerror=()=>reject(r.error)})}finally{db.close()}})).toBe('Escape draft Ω!');
  await page.reload();await expect(h.input).toHaveValue('Escape draft Ω!');await expect(pill).toBeVisible();expect(sends).toBe(0);
 }finally{await h.close()}
});

test('autocomplete owns first Escape; plain second Escape blurs; IME and consumed keys retain focus',async({page},info)=>{
 const h=await setup(page,info);try{
  await h.input.fill('/mo');const popup=page.locator('.slash-autocomplete');await expect(popup).toBeVisible();await h.input.press('Escape');await expect(popup).toHaveCount(0);await expect(h.input).toBeFocused();await expect(h.input).toHaveValue('/mo');
  await h.input.press('Escape');await expect(h.input).not.toBeFocused();await h.input.focus();await h.input.fill('guarded Escape');
  await h.input.evaluate(e=>e.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true,cancelable:true,isComposing:true})));await expect(h.input).toBeFocused();
  await h.input.evaluate(e=>{const event=new KeyboardEvent('keydown',{key:'Escape',bubbles:true,cancelable:true});event.preventDefault();e.dispatchEvent(event)});await expect(h.input).toBeFocused();await expect(h.input).toHaveValue('guarded Escape');
  const trigger=page.getByRole('button',{name:'Open model picker',exact:true});await trigger.click();const search=page.getByRole('combobox',{name:'Search models',exact:true});await expect(search).toBeFocused();await search.press('Escape');await expect(trigger).toBeFocused();await expect(h.input).toHaveValue('guarded Escape');
 }finally{await h.close()}
});
