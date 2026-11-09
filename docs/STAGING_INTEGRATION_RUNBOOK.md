# StoreLift staging E2E and launch gates (2026-11-09)

## Verified resources and status

- GitHub: `Kadir70-dev/StoreLift`
- Main at beginning of this work: `a4baf3022cba57e3155f9ab9b850fcf63b78a786`; GitHub Actions passed.
- Preview Vercel project: `storelift-preview`, project ID `prj_jSGxdiA0QGTVCIZKNZM2rTzKgHT1`.
- Preview deployment: `dpl_FBjQim176XSeGH5QUddUZsLMnmyQ`, `READY`, target `preview`, source `staging/preview`.
- URL: https://storelift-preview-1r5ysoytz-kadir70-devs-projects.vercel.app
- Vercel Authentication (SSO) protects preview URLs.
- **Preview frontend does not yet have a persistent Go backend or database; audit, leads and Stripe are not live there.**
- No production deployment or live payments authorized.

## New CI tests (non-mocked full stack)

`.github/workflows/qa.yml` has a `real-api-e2e` job:

1. Start PostgreSQL 16 service with throwaway CI-only credentials.
2. Apply production schema to that throwaway database.
3. Generate ephemeral PBKDF2-SHA256 admin hash without printing secrets.
4. Start the **actual** Go HTTP server, with Stripe checkout disabled.
5. Start the actual Next.js web server in Playwright.
6. Execute `web/tests/real-api.spec.ts` with **zero /api route interception**. Covers persistence of lead through browser API, password-only admin auth, authorization/revocation, checkout fail-closed, SSRF and consent rejection.
7. Execute `scripts/stripe-webhook-ci.py` against the running Go API + PostgreSQL, using a **synthetic** `whsec_ci_` signing secret and no Stripe network access: signed completion, duplicate, wrong amount, invalid signature, full refund, delayed failure ordering. Test records are limited to throwaway CI database.
8. Save browser QA artifacts.

These are implementation/integration checks, **not** a real Stripe Checkout purchase or live PageSpeed measurement.

## Production safety

Go checkout requires both `sk_test_`/approved `sk_live_` key and `whsec_` webhook secret and HTTPS `SITE_URL`. A live key is ignored unless the operator explicitly sets `STORELIFT_LIVE_PAYMENTS_APPROVED=true` after approval. The webhook handler also rejects an unapproved live-key configuration. The approval flag defaults to false and should not be set in staging.

Admin credentials belong server-side only. Never commit actual Stripe keys, API keys, admin hashes or database passwords.

## Steps still requiring connected infrastructure or business owner

1. Provision **persistent** staging Go HTTPS hosting and managed PostgreSQL (with backups/TLS). Run `db/schema.sql`, configure only staging secrets. Configure Vercel preview `API_INTERNAL_URL` to that HTTPS backend, then redeploy preview.
2. Set the restricted `PAGESPEED_API_KEY` in the Go service and verify one permissioned public Shopify store. Respect robots.txt and rate limits. Record real measurements, never fictional speed/sales values.
3. Configure a real **Stripe test-mode** account: `STRIPE_SECRET_KEY=sk_test_...`, `STRIPE_WEBHOOK_SECRET=whsec_...`, `SITE_URL=https://<actual-preview-url>`. Carry out successful, failed, cancelled, duplicate, delayed and refunded test flows, and reconcile DB events. Synthetic CI tests are not a substitute.
4. Set `RATE_LIMIT_SECRET` (32+ chars) to activate database-backed rate limits. Configure exact trusted proxy IP ranges; never `0.0.0.0/0` or `::/0`. Verify client-IP attribution from two networks in staging. Add bot/WAF rules.
5. Merchant must supply and approve actual seller legal identity, support email, jurisdiction, privacy retention periods, Terms, Refund policy and taxes. Current pages are drafts, not effective contracts.
6. Owner must explicitly authorize production deployment and, separately, live Stripe payments.

## Recommended operator commands (in a newly provisioned backend environment)

```sh
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f db/schema.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f db/smoke.sql
cd api && go test ./... && go vet ./...
# Start the Go API with server-side environment vars in your managed provider.
```

Never turn on real checkout until all gates above have been verified. See `docs/DEPLOYMENT.md` and GitHub issue #3.
