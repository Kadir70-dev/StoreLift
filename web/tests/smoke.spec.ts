import {test,expect} from '@playwright/test';

test('landing page clearly labels example data and offers a store audit', async ({page})=>{
 await page.goto('/');
 await expect(page.getByRole('heading',{level:1})).toContainText('Your store could');
 await expect(page.getByText('DEMO DATA')).toBeVisible();
 await expect(page.getByRole('button',{name:/Audit my store/i}).first()).toBeVisible();
 await expect(page.getByText('$199').first()).toBeVisible();
});

test('audit page does not fabricate scores before a URL is submitted',async ({page})=>{
 await page.goto('/audit');
 await expect(page.getByRole('heading',{level:1})).toContainText('Your store.');
 await expect(page.getByPlaceholder('yourstore.com')).toBeVisible();
 await expect(page.getByText('Your audit results.')).toHaveCount(0);
});

test('admin login is password only',async ({page})=>{
 await page.goto('/admin');
 await expect(page.getByRole('heading',{name:'Admin workspace'})).toBeVisible();
 await expect(page.getByLabel('Admin password')).toBeVisible();
 await expect(page.getByLabel(/email|username/i)).toHaveCount(0);
});

test('mobile navigation and pricing fit viewport',async ({page})=>{
 await page.setViewportSize({width:390,height:844});
 await page.goto('/');
 const bodyOverflow=await page.evaluate(()=>document.documentElement.scrollWidth>window.innerWidth);
 expect(bodyOverflow).toBe(false);
 await expect(page.getByRole('link',{name:/Free store audit/i}).first()).toBeVisible();
});
