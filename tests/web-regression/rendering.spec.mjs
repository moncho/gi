import {test,expect} from '@playwright/test';

import {attachGiDeviation} from '../ux/support/gi-deviations.mjs';
const inputName='Message (Enter to send, Shift+Enter for newline)...';

async function fixture(page,request,info,markdown){
 const token=`render-${info.project.name}-${Date.now()}`;
 const session=await(await request.post('/api/sessions',{data:{agent_id:token,title:token}})).json();
 const submitted=await request.post(`/api/sessions/${session.id}/prompt`,{data:{prompt:'Render fixture:\n\n'+markdown,model:'test-model'}});expect(submitted.status()).toBe(202);
 const {turn_id}=await submitted.json();
 await expect.poll(async()=>((await(await request.get(`/api/sessions/${session.id}/turns`)).json()).turns.find(t=>t.id===turn_id)?.status)).toBe('completed');
 const messages=(await(await request.get(`/api/sessions/${session.id}/messages`)).json()).messages;
 const stored=messages.find(m=>m.role==='assistant');expect(stored.content).toContain(markdown);
 await page.addInitScript(id=>localStorage.setItem('gi_session_id',id),session.id);await page.goto('/');
 const post=page.locator(`#post-${stored.id}`);await expect(post).toBeVisible();
 const input=page.getByRole('textbox',{name:inputName,exact:true});await input.fill('rendering draft retained');
 return {post,input,session,stored};
}
// Observe what the browser's real copy event receives, without replacing the
// clipboard API, execCommand, copy handlers or their success/failure results.
async function observeClipboard(page){
 await page.addInitScript(()=>{
  window.__nativeCopies=[];
  document.addEventListener('copy',event=>{
   const data=event.clipboardData;
   window.__nativeCopies.push({trusted:event.isTrusted,text:data?.getData('text/plain'),html:data?.getData('text/html')});
  });
 });
}

test('Gi fenced SVG uses Piclaw isolated preview and preserves exact source (bounded original-029 checks)',async({page,request},info)=>{
 await attachGiDeviation(info,'@gi-ux-003');await observeClipboard(page);
 const svg='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 120 60"><title>Safe vector</title><text x="2" y="12">SVG &amp; text</text></svg>\n';
 const {post,input}=await fixture(page,request,info,'Before SVG\n\n```svg\n'+svg+'```\n\nAfter SVG');
 const preview=post.locator('.model-svg-block'),image=preview.locator('img.model-svg-image');
 await expect(preview).toHaveCount(1);await expect(image).toBeVisible();
 await expect(image).toHaveAttribute('src',/^data:image\/svg\+xml;base64,/);
 await expect(image).toHaveAttribute('alt','Safe vector');
 await expect(post.locator('.post-content > svg')).toHaveCount(0);
 await expect(post.locator('.post-content')).toContainText('Before SVG');
 await expect(post.locator('.post-content')).toContainText('After SVG');
 const decoded=await image.evaluate(el=>new TextDecoder().decode(Uint8Array.from(atob(el.src.split(',')[1]),ch=>ch.charCodeAt(0))));
 expect(decoded).toContain('SVG &amp; text');expect(decoded).not.toContain('onload=');
 const code=preview.locator('.model-svg-source pre code');await expect(code).toHaveText(svg);
 expect(await preview.locator('.model-svg-source').evaluate(el=>el.hasAttribute('open'))).toBe(false);
 await preview.locator('.model-svg-source summary').click();
 const block=preview.locator('.post-code-block');await block.getByRole('button',{name:'Copy code',exact:true}).click();
 await expect.poll(()=>page.evaluate(()=>window.__nativeCopies.at(-1))).toMatchObject({trusted:true,text:svg});
 const surface=preview.getByRole('combobox',{name:'SVG background'});
 const initialSrc=await image.getAttribute('src');
 const layout=await preview.evaluate(el=>({width:el.getBoundingClientRect().width,body:document.body.scrollWidth,viewport:innerWidth}));
 expect(layout.width).toBeLessThanOrEqual(layout.viewport);expect(layout.body).toBeLessThanOrEqual(layout.viewport);
 await page.setViewportSize({width:Math.max(320,Math.floor(info.project.use.viewport.width*0.8)),height:info.project.use.viewport.height});
 await expect(image).toBeVisible();
 expect(await preview.evaluate(el=>document.body.scrollWidth<=innerWidth&&el.getBoundingClientRect().width<=innerWidth)).toBe(true);
 await surface.selectOption('dark');await expect(preview).toHaveAttribute('data-svg-surface','dark');
 await expect(image).toHaveAttribute('src',/^data:image\/svg\+xml;base64,/);
 await expect.poll(()=>image.getAttribute('src')).not.toBe(initialSrc);
 await surface.selectOption('light');await expect(preview).toHaveAttribute('data-svg-surface','light');
 await expect.poll(()=>image.getAttribute('src')).not.toBe(initialSrc);
 await expect(input).toHaveValue('rendering draft retained');
});

test('Gi SVG sanitizer rejects unsafe and malformed fences without loading resources',async({page,request},info)=>{
 await attachGiDeviation(info,'@gi-ux-003');await observeClipboard(page);
 const sources=[
  '<svg xmlns="http://www.w3.org/2000/svg" onload="window.svgAttack=true"><script>window.svgAttack=true</script><image href="https://example.invalid/svg-tracker"/></svg>\n',
  '<svg xmlns="http://www.w3.org/2000/svg"><foreignObject><img src="https://example.invalid/svg-tracker"/></foreignObject></svg>\n',
  '<svg xmlns="http://www.w3.org/2000/svg"><text>unescaped & text</text></svg>\n',
  '<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><a xlink:href="https://example.invalid/svg-tracker"><text>external</text></a></svg>\n',
  '<svg xmlns="http://www.w3.org/2000/svg"><g xmlns="http://www.w3.org/1999/xhtml"><div>mixed namespace</div></g></svg>\n',
  '<!DOCTYPE svg [<!ENTITY bad SYSTEM "https://example.invalid/svg-tracker">]><svg xmlns="http://www.w3.org/2000/svg"><text>&bad;</text></svg>\n',
 ];
 const requests=[];page.on('request',r=>{if(r.url().includes('svg-tracker'))requests.push(r.url());});
 const {post,input}=await fixture(page,request,info,sources.map(s=>'```svg\n'+s+'```').join('\n\n'));
 await expect(post.locator('.model-svg-block')).toHaveCount(0);
 const codes=post.locator('pre code.language-svg');await expect(codes).toHaveCount(sources.length);
 for(let n=0;n<sources.length;n++)await expect(codes.nth(n)).toHaveText(sources[n]);
 await expect(post.locator('.post-content img.model-svg-image, .post-content > svg')).toHaveCount(0);
 expect(requests).toEqual([]);expect(await page.evaluate(()=>!!window.svgAttack)).toBe(false);
 await post.locator('.post-code-block').first().getByRole('button',{name:'Copy code',exact:true}).click();
 await expect.poll(()=>page.evaluate(()=>window.__nativeCopies.at(-1))).toMatchObject({trusted:true,text:sources[0]});
 await expect(input).toHaveValue('rendering draft retained');
});

test('Gi strips unsafe SVG attributes from isolated preview without external fetches',async({page,request},info)=>{
 const source='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20"><rect style="fill:url(https://example.invalid/svg-tracker)" onclick="window.svgAttack=true" width="10" height="10" fill="red"/></svg>\n';
 const requests=[];page.on('request',r=>{if(r.url().includes('svg-tracker'))requests.push(r.url());});
 const {post}=await fixture(page,request,info,'```svg\n'+source+'```');
 const image=post.locator('.model-svg-block img.model-svg-image');await expect(image).toBeVisible();
 const decoded=await image.evaluate(el=>new TextDecoder().decode(Uint8Array.from(atob(el.src.split(',')[1]),ch=>ch.charCodeAt(0))));
 expect(decoded).toContain('<rect');expect(decoded).not.toContain('style=');expect(decoded).not.toContain('onclick=');
 expect(decoded).not.toContain('svg-tracker');expect(requests).toEqual([]);
 expect(await page.evaluate(()=>!!window.svgAttack)).toBe(false);
 await expect(post.locator('.model-svg-source pre code')).toHaveText(source);
});

test('Gi leaves oversized SVG as source without a preview (renderer-only fixture)',async({page,request},info)=>{
 // Inject only the browser's message response: the test-model provider rejects
 // a 256 KiB prompt before it reaches the renderer. Keep the stored turn small.
 const oversized='<svg xmlns="http://www.w3.org/2000/svg"><text>'+'x'.repeat(256*1024)+'</text></svg>\n';
 await page.route('**/api/sessions/*/messages*',async route=>{
  const response=await route.fetch(),body=await response.json();
  for(const message of body.messages||[])if(message.role==='assistant')message.content='```svg\n'+oversized+'```';
  await route.fulfill({response,json:body});
 });
 const {post,input}=await fixture(page,request,info,'```svg\n<svg><title>small provider prompt</title></svg>\n```');
 await expect(post.locator('.model-svg-block')).toHaveCount(0);
 await expect(post.locator('pre code.language-svg')).toContainText('x'.repeat(100));
 await expect(post.locator('img.model-svg-image')).toHaveCount(0);
 await expect(input).toHaveValue('rendering draft retained');
});

test('Gi wide Markdown tables stay inside the timeline and preserve the draft after reload',async({page,request},info)=>{
 const headers=Array.from({length:12},(_,i)=>`Column ${i+1}`);
 const markdown='| '+headers.join(' | ')+' |\n| '+headers.map(()=> '---').join(' | ')+' |\n| '+headers.map((_,i)=>`payload_${i}_${'abcdef'.repeat(8)}`).join(' | ')+' |';
 const {post,input,stored,session}=await fixture(page,request,info,markdown);
 const table=post.locator('.post-content table');await expect(table).toBeVisible();
 const measurements=async()=>table.evaluate(el=>({display:getComputedStyle(el).display,layout:getComputedStyle(el).tableLayout,body:document.body.scrollWidth,viewport:innerWidth,content:el.closest('.post-content').getBoundingClientRect().width,table:el.getBoundingClientRect().width}));
 expect((await measurements()).body).toBeLessThanOrEqual((await measurements()).viewport);
 expect((await measurements()).display).toBe('table');expect((await measurements()).layout).toBe('auto');
 const overflow=await table.evaluate(el=>{const content=el.closest('.post-content');return {client:content.clientWidth,scroll:content.scrollWidth,overflow:getComputedStyle(content).overflowX}});
 expect(overflow.scroll).toBeGreaterThan(overflow.client);expect(overflow.overflow).toBe('auto');
 // Native wheel input can reach the final column; it must not be clipped by
 // the supplied post-body overflow boundary.
 await table.locator('td').first().hover();await page.mouse.wheel(10000,0);
 await expect.poll(()=>table.evaluate(el=>el.closest('.post-content').scrollLeft)).toBeGreaterThan(0);
 await expect.poll(()=>table.locator('td').last().evaluate(el=>{const cell=el.getBoundingClientRect(),content=el.closest('.post-content').getBoundingClientRect();return cell.right<=content.right+1&&cell.left<content.right})).toBe(true);
 expect(await page.evaluate(()=>localStorage.getItem('gi_session_id'))).toBe(session.id);
 // Native edge gestures also belong to this table, not session navigation.
 await table.locator('td').last().hover();await page.mouse.wheel(10000,0);await page.waitForTimeout(500);await page.mouse.wheel(10000,0);await page.waitForTimeout(500);
 expect(await page.evaluate(()=>localStorage.getItem('gi_session_id'))).toBe(session.id);
 await expect(table.locator('td')).toHaveCount(12);await expect(input).toHaveValue('rendering draft retained');
 await page.reload();await expect(page.locator(`#post-${stored.id} table td`)).toHaveCount(12);await expect(input).toHaveValue('rendering draft retained');
});

test('@ux-timeline-023 Markdown table spans the content area with automatic table layout',async({page,request},info)=>{

 const markdown='| Column | Description |\n| --- | --- |\n| Alpha | Native stored Markdown |\n| Beta | 第二行 |';
 const {post,input}=await fixture(page,request,info,markdown);
 const table=post.locator('.post-content table');await expect(table).toBeVisible();
 const layout=await table.evaluate(el=>{const css=getComputedStyle(el);return {display:css.display,layout:css.tableLayout,width:el.getBoundingClientRect().width,parent:el.parentElement.getBoundingClientRect().width}});
 expect(layout.display).toBe('table');expect(layout.layout).toBe('auto');expect(Math.abs(layout.width-layout.parent)).toBeLessThanOrEqual(1);
 await expect(table.locator('tbody tr')).toHaveCount(2);await expect(table).toContainText('第二行');await expect(input).toHaveValue('rendering draft retained');
});

test('@ux-timeline-024 Code-copy control copies native code text from the top-right of its block',async({page,request},info)=>{
 await observeClipboard(page);
 const code='const answer = "<tag> & 中文🙂";\nconsole.log(answer);\n';
 const {post,input}=await fixture(page,request,info,'```javascript\n'+code+'```');
 const block=post.locator('.post-code-block'),button=block.getByRole('button',{name:'Copy code',exact:true});
 await expect(button).toBeVisible();await expect(block.locator('pre code')).toHaveText(code);
 const position=await button.evaluate(el=>{const a=el.getBoundingClientRect(),b=el.closest('.post-code-block').getBoundingClientRect();return {top:a.top-b.top,right:b.right-a.right}});
 expect(position.top).toBeGreaterThanOrEqual(0);expect(position.top).toBeLessThanOrEqual(12);expect(position.right).toBeGreaterThanOrEqual(0);expect(position.right).toBeLessThanOrEqual(12);
 await button.click();
 await expect.poll(()=>page.evaluate(()=>window.__nativeCopies.at(-1))).toMatchObject({trusted:true,text:code});
 expect((await page.evaluate(()=>window.__nativeCopies.at(-1))).text).not.toContain('<span');
 await expect(input).toHaveValue('rendering draft retained');
});
