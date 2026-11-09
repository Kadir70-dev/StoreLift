# StoreLift QA / blocker report — 9 November 2026

## What was actually executed in this artifact environment
- `gofmt` on the complete Go source tree: **passed** (syntax formatted/parsed).
- TypeScript/TSX `typescript.createSourceFile(...).parseDiagnostics` parser on frontend files: **passed** (syntax only; not typecheck).
- `GOPROXY=off GOSUMDB=off go test ./internal/safepay ./internal/robotspolicy -count=1`: **passed**. Tests cover Stripe HMAC signature authenticity, mutation/replay-window rejection, safe checkout origin rules and basic robots.txt interpretation.
- Static scans for no hard-coded production credentials and source archive packaging: **performed**; not a substitute for dedicated SAST.

## Automated checks delivered but not run locally
- `go test ./...` / `go vet ./...`: external Go modules cannot download; missing `go.sum`.
- Real PostgreSQL migration, transactional smoke test and admin session integration: no PostgreSQL service or container runtime available here.
- `npm run typecheck`, `npm run lint`, `npm run build`: network registry unavailable and web dependencies not installed.
- Playwright desktop and mobile E2E screenshots: test scripts authored, but Next.js cannot start without dependencies.
- Real PageSpeed requests: no API key or outbound network available.
- Stripe Checkout and signed provider-to-DB webhook lifecycle: implemented but **not yet verified** against a real Stripe test account.

## Features implemented as source
1. Stripe-hosted checkout with server-authoritative fixed prices ($49/$199/$499), pending session reuse, expiry handling, Stripe HMAC/5-min tolerance, event idempotency, currency/amount check, paid/unpaid/refunded reconciliation and read-only admin payment states.
2. Before/after comparison page with same-store/time-order validation in Go API, stored historic audit selection, real delta calculations and export to PDF (client-side jsPDF).
3. PostgreSQL additive schema, payment events log, deal checkout columns, indexes, repeatable test transaction script.
4. Playwright mocked-flow, unauthenticated admin, mobile overflow tests and screenshot artifacts; CI GitHub Actions workflow for an actual network-enabled runner.
5. Robots.txt-aware public HTTPS HTML crawler with DNS/private-IP checks, no cross-domain redirects other than www equivalence, request timeouts and tight scan limits, plus optional PostgreSQL-backed hashed-IP rate buckets.
6. Password-only admin auth with server-stored PBKDF2 hash and HttpOnly sessions; no manual "paid" editing in UI/API.
7. Separate Go Dockerfile, Vercel proxy setup, deployment runbook and explicit no-prod gate.

## Blocking production launch
- Provider credentials, domain, API and DB provisioning, merchant-approved policies and **explicit user authorization**.
- Full network-enabled Go/Next/Playwright/PostgreSQL CI pass, Stripe test-mode real E2E, trusted-proxy client-IP attribution, enabled DB-backed rate limiter and edge bot protection and operational monitoring.
- Review of real merchant pages/robots and privacy/retention/terms/refunds/tax policy.

**Bottom line:** The pending functionality is **coded and packaged**, but the application is **not independently certified production-ready**. No live payment or prod deployment took place; live blockers must be cleared before accepting real money.
