#!/usr/bin/env python3
"""CI-only signed Stripe webhook -> real Go API -> real PostgreSQL integration.

This NEVER connects to Stripe. Every event/credential here is a synthetic fixture.
It is a regression test, not evidence of a Stripe test-mode checkout.
"""
import hashlib
import hmac
import json
import os
import subprocess
import time
import urllib.error
import urllib.request

API = os.environ.get("STORELIFT_CI_API_URL", "http://127.0.0.1:8080")
SECRET = os.environ["STRIPE_WEBHOOK_SECRET"]
DB = os.environ["DATABASE_URL"]
if not SECRET.startswith("whsec_ci_") or not API.startswith("http://127.0.0.1:"):
    raise SystemExit("CI-only fixtures required; refusing any remote provider/API URL")

LEAD = "00000000-0000-4000-8000-000000000011"
DEAL = "00000000-0000-4000-8000-000000000012"
SESSION = "cs_test_storelift_ci_integration"
PI = "pi_storelift_ci_integration"

def sql(query):
    result = subprocess.run(
        ["psql", DB, "-X", "-A", "-t", "-v", "ON_ERROR_STOP=1", "-c", query],
        capture_output=True, text=True, check=True
    )
    return result.stdout.strip()

def post(event, corrupt=False):
    body = json.dumps(event, separators=(",", ":")).encode()
    timestamp = str(int(time.time()))
    digest = hmac.new(
        SECRET.encode(), timestamp.encode() + b"." + body, hashlib.sha256
    ).hexdigest()
    if corrupt:
        digest = "0" * len(digest)
    req = urllib.request.Request(
        API + "/api/webhooks/stripe", data=body,
        headers={"Content-Type": "application/json",
                 "Stripe-Signature": f"t={timestamp},v1={digest}"},
        method="POST"
    )
    try:
        with urllib.request.urlopen(req, timeout=10) as response:
            return response.status, json.loads(response.read())
    except urllib.error.HTTPError as err:
        return err.code, json.loads(err.read())

def session_event(event_id, event_type, amount=19900):
    return {
        "id": event_id, "type": event_type,
        "data": {"object": {
            "id": SESSION, "client_reference_id": DEAL,
            "payment_status": "paid", "amount_total": amount,
            "currency": "usd", "payment_intent": PI
        }}
    }

sql(f"""
INSERT INTO leads(id,email,website,consent,preferred_package)
 VALUES ('{LEAD}','stripe-fixture@example.test',
 'https://sample-store.myshopify.com/',true,'fix');
INSERT INTO deals(id,lead_id,package,amount_usd,status,payment_status,
 stripe_session_id,checkout_expires_at)
 VALUES ('{DEAL}','{LEAD}','fix',199,'new','pending',
 '{SESSION}',now()+interval '1 hour');
""")

good = session_event("evt_ci_paid_001", "checkout.session.completed")
status, data = post(good)
assert status == 200 and data.get("processed"), (status, data)
assert sql(f"SELECT payment_status FROM deals WHERE id='{DEAL}'") == "paid"
assert sql(f"SELECT stripe_payment_intent FROM deals WHERE id='{DEAL}'") == PI
print("PASS: signed payment completed -> PostgreSQL paid")

status, data = post(good)
assert status == 200 and data.get("duplicate"), (status, data)
assert sql("SELECT count(*) FROM payment_events WHERE id='evt_ci_paid_001'") == "1"
print("PASS: duplicate signed event is idempotent")

status, data = post(session_event("evt_ci_wrong_amount", "checkout.session.completed", 4900))
assert status == 409, (status, data)
assert sql(f"SELECT payment_status FROM deals WHERE id='{DEAL}'") == "paid"
print("PASS: wrong amount rejected")

status, data = post(session_event("evt_ci_bad_signature", "checkout.session.completed"), corrupt=True)
assert status == 400, (status, data)
assert sql("SELECT count(*) FROM payment_events WHERE id='evt_ci_bad_signature'") == "0"
print("PASS: invalid signature rejected")

refund = {"id":"evt_ci_refund_001","type":"charge.refunded","data":{"object":{
  "payment_intent":PI,"currency":"usd","amount":19900,
  "amount_refunded":19900,"refunded":True
}}}
status, data = post(refund)
assert status == 200 and data.get("processed"), (status,data)
assert sql(f"SELECT payment_status FROM deals WHERE id='{DEAL}'") == "refunded"
assert sql(f"SELECT refunded_amount_cents FROM deals WHERE id='{DEAL}'") == "19900"
print("PASS: signed refund -> PostgreSQL refunded")

status, data = post(session_event("evt_ci_late_failed", "checkout.session.async_payment_failed"))
assert status == 200 and data.get("processed"), (status,data)
assert sql(f"SELECT payment_status FROM deals WHERE id='{DEAL}'") == "refunded"
print("PASS: delayed failure does not reverse refunded status")
print("Synthetic Stripe/Go/Postgres integration passed. No Stripe network calls made.")
