import {test,expect} from '@playwright/test';
import {journeyEnvironment} from '../ux/support/journey-environment.mjs';

test('Gi mounted composer does not intercept the Classic BTW command',async({page},info)=>{
 const env=await journeyEnvironment(info);let attempts=0;
 try{
  const created=page.waitForResponse(r=>new URL(r.url()).pathname==='/api/sessions'&&r.request().method()==='POST');
  await page.goto(env.origin);const response=await created;expect(response.status()).toBe(201);
  const id=(await response.json()).id;
  await page.route(`**/api/sessions/${id}/prompt`,route=>{
   attempts++;expect(route.request().postDataJSON().prompt).toBe('/btw fixture side question');
   return route.fulfill({status:503,contentType:'application/json',body:'{"error":"BTW admission intercepted by fixture"}'});
  });
  const input=page.locator('.compose-box textarea');await input.fill('/btw fixture side question');await input.press('Enter');
  await expect(page.getByRole('alert')).toContainText('BTW admission intercepted by fixture');
  expect(attempts).toBe(1);await expect(page.locator('.btw-panel')).toHaveCount(0);
  await expect(input).toHaveValue('/btw fixture side question');
  expect((await(await page.request.get(`${env.origin}/api/sessions/${id}/turns`)).json()).turns||[]).toEqual([]);
 }finally{await page.unrouteAll({behavior:'wait'});await page.close();await env.close();}
});
