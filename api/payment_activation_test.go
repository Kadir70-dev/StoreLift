package main

import (
    "os"
    "testing"
)

func TestCheckoutFailsClosedWithoutLiveApproval(t *testing.T) {
    t.Setenv("SITE_URL", "https://storelift-staging.example")
    t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test_fixture")
    t.Setenv("STRIPE_SECRET_KEY", "")
    t.Setenv("STORELIFT_LIVE_PAYMENTS_APPROVED", "")
    if stripeConfigured() { t.Fatal("no Stripe key must disable checkout") }

    t.Setenv("STRIPE_SECRET_KEY", "sk_test_fixture")
    if !stripeConfigured() { t.Fatal("Stripe test keys must enable test-mode checkout") }

    t.Setenv("STRIPE_SECRET_KEY", "sk_live_fixture")
    if stripeConfigured() { t.Fatal("live key alone must never enable checkout") }

    t.Setenv("STORELIFT_LIVE_PAYMENTS_APPROVED", "true")
    if !stripeConfigured() { t.Fatal("owner-authorized live credentials should work") }

    t.Setenv("STRIPE_WEBHOOK_SECRET", "")
    if stripeConfigured() { t.Fatal("missing webhook secret must disable checkout") }

    t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test_fixture")
    t.Setenv("SITE_URL", "http://localhost:3000")
    if stripeConfigured() { t.Fatal("non-HTTPS origin cannot enable checkout") }

    _ = os.Getenv("STORELIFT_LIVE_PAYMENTS_APPROVED")
}
