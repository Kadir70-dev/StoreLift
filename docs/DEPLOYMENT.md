# StoreLift deployment runbook — no production deployment performed

## Infra
- **Web:** Vercel project with root `web/`, framework Next.js, Node 22, environment `API_INTERNAL_URL=https://YOUR-API-DOMAIN` (Vercel must be able to reach Go over TLS). `next.config.ts` forwards same-origin `/api/*` traffic to that trusted backend. Avoid arbitrary user-supplied API destination.
- **API:** Any provider supporting a long-running Go container (Render, Fly.io, VPS, etc.) with TLS at the ingress. Build `api/Dockerfile`. Set `PORT`, `WEB_ORIGIN=https://YOUR-WEB-DOMAIN`, `DATABASE_URL` (TLS/SSL enabled), `PAGESPEED_API_KEY`, `ADMIN_PASSWORD_HASH`, `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `RATE_LIMIT_SECRET` (32+ random characters), `SITE_URL=https://YOUR-WEB-DOMAIN` in the provider secret manager.
- **Database:** Managed PostgreSQL 16 with automatic encrypted backups, separate least-privilege application role, network allowlisting and connection limits. Apply `db/schema.sql` before starting the API (`psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f db/schema.sql`). In a nonproduction database run `db/smoke.sql`; it rolls back its test rows. Local compose credentials are **development only**.
- **Stripe:** Create test-mode secret key and webhook signing secret. Configure webhook URL `https://YOUR-WEB-DOMAIN/api/webhooks/stripe` for `checkout.session.completed`, `checkout.session.async_payment_succeeded`, `checkout.session.async_payment_failed`, `checkout.session.expired`, `charge.refunded`. Start with test mode and verify provider dashboards and event replays before live keys. The return page is *not* proof of payment.
- **Google PageSpeed:** Provision a restricted API key for the PageSpeed Insights API. Absence of key displays `not measured`, never a made-up performance score.

## Launch approval gates
1. All GitHub Action jobs green on an actual network-enabled runner; install dependencies and generate/review lockfiles and Go checksums. Test browser screenshots from Chromium and mobile emulation.
2. Repeat payments in Stripe test mode including success, declined, delayed, duplicate event, expired link, partial/full refund. Confirm net-revenue reporting and DB state with real PostgreSQL.
3. Configure **real** support/privacy contact, approved terms/refund policy, privacy retention and deletion workflow, tax calculation/liability and the legal identity of the seller. Have this reviewed appropriately for target jurisdictions.
4. Set `RATE_LIMIT_SECRET` to activate the PostgreSQL-backed multi-instance limiter; without it the fallback limiter is process-local. Add bot protection and audit queuing at the web ingress. Ensure the trusted ingress passes real client identity safely: `RemoteAddr` will otherwise be the proxy egress address and a shared quota may throttle unrelated users. Keep the backend reachable only through controlled ingress. Schedule `DELETE FROM rate_buckets WHERE window_start < now()-interval '48 hours'` periodically. Set monitoring/alerts on API, DB, Stripe events, PSI quota and crawler errors. Validate robots compliance on real merchant domains.
5. Run authorized merchant-store audits and manually verify HTML findings, JavaScript-rendered product details, screenshots and QA. Do not present model/fixture values as real customer results.
6. Store secrets only in encrypted secrets management. Never commit credentials, paste them into chats or put them in `NEXT_PUBLIC_*` variables. Configure HTTPS and production cookie policy.
7. Obtain explicit release authorization before changing production environments or enabling live checkout.

## Rollback
- Vercel: redeploy or promote last known-good web build.
- Go: deploy previous tested container digest. Avoid destructive automatic schema rollbacks; DB changes are additive.
- Payments: disable checkout by removing `STRIPE_SECRET_KEY` from Go runtime until incident resolved; do not remove Stripe webhook secret until event reconciliation is complete.

## Merchant access
- Free audit never needs Shopify admin permissions and does not edit pages.
- Only perform paid changes after agreeing on scope and requesting **Shopify collaborator access** via Shopify Dev Dashboard. Merchant must approve minimal permissions. Never ask for owner passwords.

## Current status
**Not deployed to production.** Provider accounts, domain, live secrets, real customer data and authorization are unavailable in this package. Deployment configuration exists; release and live verification remain gated.
