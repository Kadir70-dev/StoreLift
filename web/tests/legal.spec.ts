import {test,expect} from '@playwright/test';

test('policy pages are accessible, visibly marked as drafts, and in footer',async ({page})=>{
 await page.goto('/');
 await expect(page.getByRole('link',{name:'Privacy'})).toHaveAttribute('href','/privacy');
 await expect(page.getByRole('link',{name:'Terms'})).toHaveAttribute('href','/terms');
 await expect(page.getByRole('link',{name:'Refunds'})).toHaveAttribute('href','/refund');
 for (const path of ['/terms','/refund']) {
  await page.goto(path);
  await expect(page.getByRole('heading',{level:1})).toBeVisible();
  await expect(page.getByText(/pending approval|draft only/i).first()).toBeVisible();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
 }
 await page.goto('/privacy');
 await expect(page.getByRole('heading',{level:1})).toBeVisible();
});
