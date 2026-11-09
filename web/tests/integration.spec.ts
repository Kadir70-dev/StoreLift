import {test,expect} from '@playwright/test';
import {readFileSync} from 'fs';
import {resolve} from 'path';
const fixture=JSON.parse(readFileSync(resolve(process.cwd(),'../examples/sample-audit.json'),'utf8'));

test('store audit → evidence → lead → checkout (mock payment; no real charge)',async ({page},testInfo)=>{
 let leadPayload:{email?:string;consent?:boolean}={};
 await page.route('**/api/health',route=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({status:'ok',checkoutConfigured:true})}));
 await page.route('**/api/audits',route=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(fixture)}));
 await page.route('**/api/leads',async route=>{leadPayload=route.request().postDataJSON();await route.fulfill({status:201,contentType:'application/json',body:JSON.stringify({id:'c73ee5d1-9bb3-4c2a-8224-bc771fe097e0',status:'received'})})});
 await page.route('**/api/checkout',async route=>route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:'Mock checkout is disabled; no charge made'})}));
 await page.goto('/');
 await page.getByRole('textbox',{name:'Shopify store URL'}).fill('sample-store.example');
 await page.getByRole('button',{name:/Audit my store/i}).first().click();
 await expect(page.getByRole('heading',{name:/Your audit results/i})).toBeVisible();
 await expect(page.getByText('Missing meta description').first()).toBeVisible();
 await expect(page.getByText('Not measured')).toHaveCount(2);
 await page.getByLabel('Your work email').fill('merchant@example.com');
 await page.locator('input[type="checkbox"]').check();
 await page.getByRole('button',{name:/Request my \$199 package/i}).click();
 await expect(page.getByText('Request received')).toBeVisible();
 expect(leadPayload.email).toBe('merchant@example.com');
 expect(leadPayload.consent).toBe(true);
 await page.screenshot({path:testInfo.outputPath('audit-result.png'),fullPage:true});
 await page.getByRole('button',{name:/Pay \$199 via Stripe/i}).click();
 await expect(page.getByRole('alert')).toContainText('Mock checkout is disabled');
});

test('admin does not show email/username and denies unauthorized data',async ({page},testInfo)=>{
 await page.route('**/api/admin/leads',route=>route.fulfill({status:401,contentType:'application/json',body:JSON.stringify({error:'admin login required'})}));
 await page.route('**/api/admin/deals',route=>route.fulfill({status:401,contentType:'application/json',body:JSON.stringify({error:'admin login required'})}));
 await page.goto('/admin');
 await expect(page.getByLabel('Admin password')).toBeVisible();
 await expect(page.getByLabel(/email|username/i)).toHaveCount(0);
 await page.screenshot({path:testInfo.outputPath('admin-login.png'),fullPage:true});
});

test('mobile screens do not overflow',async ({page},testInfo)=>{
 await page.setViewportSize({width:390,height:844});
 await page.goto('/');
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
 await page.screenshot({path:testInfo.outputPath('mobile-home.png'),fullPage:true});
 await page.goto('/pay?lead=c73ee5d1-9bb3-4c2a-8224-bc771fe097e0');
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
 await expect(page.getByRole('button',{name:/Continue to payment/i})).toBeVisible();
});
