CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE TABLE IF NOT EXISTS leads (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), email TEXT NOT NULL, website TEXT NOT NULL,
 consent BOOLEAN NOT NULL DEFAULT false, preferred_package TEXT NOT NULL DEFAULT 'fix' CHECK (preferred_package IN ('audit','fix','premium')), created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS audits (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), website TEXT NOT NULL,
 result JSONB NOT NULL, lead_id UUID REFERENCES leads(id), created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS audits_created_idx ON audits(created_at DESC);
CREATE TABLE IF NOT EXISTS deals (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), lead_id UUID REFERENCES leads(id),
 audit_id UUID REFERENCES audits(id), package TEXT NOT NULL CHECK (package IN ('audit','fix','premium')),
 amount_usd INT NOT NULL CHECK (amount_usd IN (49,199,499)),
 status TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new','contacted','qualified','proposal','won','lost')),
 payment_status TEXT NOT NULL DEFAULT 'unpaid' CHECK(payment_status IN ('unpaid','pending','paid','refunded')),
 notes TEXT NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS audit_comparisons (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), deal_id UUID NOT NULL REFERENCES deals(id),
 before_audit_id UUID NOT NULL REFERENCES audits(id), after_audit_id UUID NOT NULL REFERENCES audits(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS admin_sessions (
 token_hash TEXT PRIMARY KEY, expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS admin_sessions_expiry_idx ON admin_sessions(expires_at);

-- Hosted checkout / signed webhook reconciliation. All changes below are additive.
ALTER TABLE deals ADD COLUMN IF NOT EXISTS stripe_session_id TEXT UNIQUE;
ALTER TABLE deals ADD COLUMN IF NOT EXISTS stripe_checkout_url TEXT;
ALTER TABLE deals ADD COLUMN IF NOT EXISTS checkout_expires_at TIMESTAMPTZ;
ALTER TABLE deals ADD COLUMN IF NOT EXISTS stripe_payment_intent TEXT;
CREATE TABLE IF NOT EXISTS payment_events (
 id TEXT PRIMARY KEY,
 event_type TEXT NOT NULL,
 processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS deals_stripe_payment_idx ON deals(stripe_payment_intent);
CREATE INDEX IF NOT EXISTS deals_open_checkout_idx ON deals(lead_id, checkout_expires_at DESC) WHERE payment_status='pending';
CREATE INDEX IF NOT EXISTS audits_website_created_idx ON audits(website,created_at DESC);

ALTER TABLE deals ADD COLUMN IF NOT EXISTS refunded_amount_cents INT NOT NULL DEFAULT 0 CHECK(refunded_amount_cents >= 0);
CREATE UNIQUE INDEX IF NOT EXISTS deals_single_active_stripe_checkout ON deals(lead_id) WHERE payment_status='pending' AND stripe_session_id IS NOT NULL;

-- Cluster-safe, IP-hashed per-route minute buckets; prune rows older than 48h via scheduled SQL.
CREATE TABLE IF NOT EXISTS rate_buckets (
 ip_hash TEXT NOT NULL,route TEXT NOT NULL,window_start TIMESTAMPTZ NOT NULL,hits INT NOT NULL DEFAULT 0,
 PRIMARY KEY(ip_hash,route,window_start)
);
CREATE INDEX IF NOT EXISTS rate_buckets_prune_idx ON rate_buckets(window_start);
