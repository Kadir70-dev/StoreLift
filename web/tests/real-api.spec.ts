import {expect, test} from '@playwright/test';

// Unlike integration.spec.ts, this suite does not intercept /api/* requests.
// It exercises Next.js -> actual Go HTTP server -> PostgreSQL on a CI runner.
// Run only in a private staging/CI environment with ephemeral credentials.
test.skip(process.env.REAL_API_E2E !== '1', 'Requires a real local Go API and PostgreSQL');

test('real lead persistence, password-only admin session and checkout safety', async ({page}) => {
  const health = await page.request.get('/api/health');
  expect(health.status()).toBe(200);
  const h = await health.json();
  expect(h.status).toBe('ok');
  expect(h.db).toBe(true);
  expect(h.checkoutConfigured).toBe(false);

  const email = `ci-storelift-${Date.now()}@example.test`;
  const website = 'https://sample-store.myshopify.com/';
  const result = await page.request.post('/api/leads', {data: {
    website, email, consent: true, package: 'fix'
  }});
  expect(result.status()).toBe(201);
  const lead = await result.json();
  expect(lead.id).toMatch(/^[0-9a-f-]{36}$/);

  // Real checkout must NOT collect money without provider credentials.
  const checkout = await page.request.post('/api/checkout', {data: {leadId: lead.id}});
  expect(checkout.status()).toBe(503);
  expect((await checkout.json()).error).toContain('not configured');

  // Authorization is enforced by the Go API, not only by UI hiding.
  const unauthenticated = await page.request.get('/api/admin/leads');
  expect(unauthenticated.status()).toBe(401);

  await page.goto('/admin');
  await expect(page.getByLabel('Admin password')).toBeVisible();
  await expect(page.getByLabel(/username|email/i)).toHaveCount(0);
  await page.getByLabel('Admin password').fill(process.env.CI_ADMIN_PASSWORD || '');
  await page.getByRole('button', {name: 'Unlock dashboard'}).click();

  await expect(page.getByRole('heading', {name: 'Sales workspace'})).toBeVisible();
  await expect(page.getByText(email)).toBeVisible();
  const authorized = await page.request.get('/api/admin/leads');
  expect(authorized.status()).toBe(200);
  const leads = await authorized.json();
  expect(leads.some((item: {id: string}) => item.id === lead.id)).toBe(true);

  await page.getByRole('button', {name: /Log out/}).click();
  await expect(page.getByLabel('Admin password')).toBeVisible();
  const revoked = await page.request.get('/api/admin/leads');
  expect(revoked.status()).toBe(401);
});

test('real API rejects private-network audit targets and consent-free leads', async ({page}) => {
  const blocked = await page.request.post('/api/audits', {data: {website: 'https://127.0.0.1/'}});
  expect(blocked.status()).toBe(400);
  const noConsent = await page.request.post('/api/leads', {data: {
    email: 'not-consented@example.test',
    website: 'https://sample-store.myshopify.com/',
    consent: false
  }});
  expect(noConsent.status()).toBe(400);
});
