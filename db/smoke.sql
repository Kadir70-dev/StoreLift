-- Run against a throwaway or testing PostgreSQL instance. All inserts roll back.
\set ON_ERROR_STOP on
BEGIN;
DO $$
DECLARE l UUID:=gen_random_uuid(); a UUID:=gen_random_uuid(); d UUID:=gen_random_uuid(); got INT;
BEGIN
 INSERT INTO leads(id,email,website,consent,preferred_package) VALUES(l,'smoke@example.invalid','https://example.com/',true,'fix');
 INSERT INTO audits(id,website,result,lead_id) VALUES(a,'https://example.com/','{"id":"test","score":75,"issues":[]}'::jsonb,l);
 INSERT INTO deals(id,lead_id,audit_id,package,amount_usd,status,payment_status) VALUES(d,l,a,'fix',199,'new','pending');
 UPDATE deals SET stripe_session_id='cs_test_smoke',stripe_checkout_url='https://checkout.stripe.com/c/pay/cs_test_smoke',checkout_expires_at=now()+interval '1 hour' WHERE id=d;
 INSERT INTO payment_events(id,event_type) VALUES('evt_smoke_test','checkout.session.completed');
 UPDATE deals SET payment_status='paid' WHERE id=d;
 SELECT count(*) INTO got FROM deals WHERE id=d AND payment_status='paid' AND amount_usd=199 AND stripe_session_id='cs_test_smoke';
 IF got<>1 THEN RAISE EXCEPTION 'deal persistence failed'; END IF;
 SELECT count(*) INTO got FROM audits WHERE id=a AND result->>'id'='test';
 IF got<>1 THEN RAISE EXCEPTION 'audit persistence failed'; END IF;
END $$;
ROLLBACK;
\echo 'StoreLift PostgreSQL transactional smoke assertions passed (rolled back).'
