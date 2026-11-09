# StoreLift — Shopify SEO & Conversion Optimization

A read-only Shopify storefront audit + qualified-lead capture + verified Stripe Checkout service pipeline for **$49 Audit / $199 Fix Pack / $499 Premium**. Next.js frontend, Go REST API, PostgreSQL, PageSpeed Insights, browser-generated PDFs and a password-only admin workspace.

**Status:** Pre-production source deliverable. No real purchases, deployment or merchant edits were performed. Full Next build, real database and Stripe-test-mode verification remain blocked in this environment; see [`docs/QA_AND_BLOCKERS.md`](docs/QA_AND_BLOCKERS.md).

## Features
- Responsive landing, sample (clearly marked **DEMO DATA**), pricing, process, privacy draft, free scan input.
- HTTPS-only SSRF-hardened audit of homepage + at most one same-store public product page, robots.txt checks, HTML SEO evidence (meta, H1, canonical, noindex, image alt, product JSON-LD), observable purchase CTA and returns/shipping links.
- Optional measured Google PageSpeed mobile/desktop performance; **not measured** if unavailable.
- Store diagnostics with evidence, severity, practical fixes and local branded PDF report.
- Consented leads stored in PostgreSQL, sales pipeline with admin session, password-only login, tracked workflow status and immutable-to-admin verified provider payment state.
- Stripe Checkout Sessions created from **server-owned** package prices, matching amount/currency webhook verification, replay resistance and refund tracking. Payment link `/pay?lead=<UUID>` is shareable; card data goes directly to Stripe.
- Admin before/after dashboard with chronological same-store audit validation, measured score deltas and PDF comparison.
- Go unit tests; browser smoke/mocked-purchase tests with screenshot generation; GitHub Actions QA workflow and deployment runbook.

## Requirements and local startup

1. Node.js 22+, Go 1.23+, PostgreSQL 16+, Docker (for local compose), network access to install dependencies.
2. Root: `docker compose up -d postgres` (localhost port 5433, local-only default credentials).
3. Create `api/.env` from `api/.env.example`; use `DATABASE_URL=postgres://storelift:storelift_local_only@127.0.0.1:5433/storelift?sslmode=disable`. Run `cd api && go run ./cmd/hashpassword` interactively to generate `ADMIN_PASSWORD_HASH` (16+ character password), then put **only hash** in a private server-side environment. Go does **not** auto-load `.env` — use a trusted secrets loader or `set -a; source .env; set +a` in a local terminal.
4. Apply SQL once and after updates: `psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f db/schema.sql` from project root. Fresh Docker DB automatically loads `db/schema.sql` on initial creation.
5. API: `cd api && go mod download && go test ./... && go run .` on port 8080.
6. Web: `cd web && cp .env.example .env.local && npm install && npm run dev` on port 3000.
7. QA: `./scripts/run-qa.sh` (requires installed dependencies and Playwright Chromium), plus DB smoke when `DATABASE_URL` and `psql` exist.

## Optional integrations
- `PAGESPEED_API_KEY` (Go server): mobile and desktop real Lighthouse performance readings.
- **Stripe test mode** (Go server): `STRIPE_SECRET_KEY=sk_test_...`, `STRIPE_WEBHOOK_SECRET=whsec_...`, `SITE_URL=https://your-preview-domain`. Configure Stripe webhook on `/api/webhooks/stripe`; see [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md). Web never receives these keys. Without them the user can submit a service request but **cannot pay**.
- Frontend needs only `API_INTERNAL_URL`, the HTTPS Go backend address reachable from the Next server. Do not use `NEXT_PUBLIC_` for secrets.

## Main routes
| Route | Purpose |
| --- | --- |
| `/` | Landing, pricing and FAQ |
| `/audit` | Free audit, real HTML findings, PDF, lead request |
| `/pay?lead=...` | Stripe-hosted checkout for a lead's server-defined package |
| `/checkout/success` and `/checkout/cancel` | Non-authoritative payment returns (webhooks are authoritative) |
| `/admin` | Password-only sales and webhook-paid statuses |
| `/admin/comparisons` | Same-store chronological before/after comparisons and PDF |
| `/privacy` | Draft data use policy; requires launch review |

## Security and business limitations
- No Shopify admin credentials or merchant changes. Paid work requires approved scoped collaborator access, never an owner password.
- Only public pages inspected; no private merchant analytics, Google ranking guarantees, or fabricated sales. Diagnostic score is a custom HTML checklist, not a Lighthouse or revenue score.
- HTTP access limited to public HTTPS domains and robots.txt policies, product page count capped, response size and timeouts enforced. PostgreSQL-backed cross-instance rate limits are available when `RATE_LIMIT_SECRET` is set; otherwise the limiter is in memory. Configure trusted proxy IP attribution and bot protection before production.
- Payment fields cannot be set by admin UI or admin PATCH. Stripe webhooks are signature verified against original payload, amount/currency and stored Checkout Session before state change. Stripe test-mode E2E and legal/tax/refund policies remain a hard release gate.
- `.env` and secrets are never included in the source archive. Example audit in `examples/` is synthetic and must never be cited as customer proof.

## Operations and release
See [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) for Vercel frontend + Go container + managed PostgreSQL + Stripe webhook setup. **Do not launch or enable live payments without the owner's explicit approval.**
