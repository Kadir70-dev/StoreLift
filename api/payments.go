package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"storelift/api/internal/safepay"
	"strconv"
	"strings"
	"time"
)

// Payment state is never writable from an admin form. Only signed Stripe events can settle a deal.
var packagePrices = map[string]int{"audit": 49, "fix": 199, "premium": 499}
var packageNames = map[string]string{"audit": "StoreLift Focused SEO Audit", "fix": "StoreLift SEO & Conversion Fix Pack", "premium": "StoreLift Premium Optimization"}

func stripeConfigured() bool {
	return os.Getenv("STRIPE_SECRET_KEY") != "" && os.Getenv("STRIPE_WEBHOOK_SECRET") != "" && strings.HasPrefix(os.Getenv("SITE_URL"), "https://")
}

func (a *app) checkout(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		errOut(w, 503, "database unavailable")
		return
	}
	if !stripeConfigured() {
		errOut(w, 503, "secure checkout not configured; request saved, no payment collected")
		return
	}
	var input struct {
		LeadID string `json:"leadId"`
	}
	if body(r, &input) != nil || !uuidValid(input.LeadID) {
		errOut(w, 400, "valid leadId required")
		return
	}
	var email, site, pkg string
	if e := a.db.QueryRowContext(r.Context(), `SELECT email,website,preferred_package FROM leads WHERE id=$1`, input.LeadID).Scan(&email, &site, &pkg); e != nil {
		errOut(w, 404, "lead not found")
		return
	}
	amount, ok := packagePrices[pkg]
	if !ok {
		errOut(w, 422, "unsupported package")
		return
	}
	var previouslyPaid bool
	if e := a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM deals WHERE lead_id=$1 AND payment_status='paid')`, input.LeadID).Scan(&previouslyPaid); e != nil {
		errOut(w, 500, "checkout unavailable")
		return
	}
	if previouslyPaid {
		errOut(w, 409, "this request is already paid")
		return
	}
	// Retire stale pending links so a customer can retry after a session expires.
	_, _ = a.db.ExecContext(r.Context(), `UPDATE deals SET payment_status='unpaid',updated_at=now() WHERE lead_id=$1 AND payment_status='pending' AND checkout_expires_at < now()`, input.LeadID)
	// Reuse an active checkout session instead of generating duplicate payment links on retries.
	var existingURL string
	err := a.db.QueryRowContext(r.Context(), `SELECT stripe_checkout_url FROM deals WHERE lead_id=$1 AND payment_status='pending' AND checkout_expires_at>now() AND stripe_checkout_url IS NOT NULL ORDER BY created_at DESC LIMIT 1`, input.LeadID).Scan(&existingURL)
	if err == nil && safepay.ValidCheckoutURL(existingURL) {
		jsonOut(w, 200, map[string]string{"url": existingURL, "status": "pending"})
		return
	}
	dealID := makeID()
	_, err = a.db.ExecContext(r.Context(), `INSERT INTO deals(id,lead_id,package,amount_usd,payment_status,status) VALUES($1,$2,$3,$4,'pending','new')`, dealID, input.LeadID, pkg, amount)
	if err != nil {
		errOut(w, 500, "checkout could not start")
		return
	}
	session, err := createStripeSession(r.Context(), dealID, email, site, pkg, amount)
	if err != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE deals SET payment_status='unpaid' WHERE id=$1 AND stripe_session_id IS NULL`, dealID)
		errOut(w, 502, "payment provider temporarily unavailable")
		return
	}
	_, err = a.db.ExecContext(r.Context(), `UPDATE deals SET stripe_session_id=$1,stripe_checkout_url=$2,checkout_expires_at=to_timestamp($3),updated_at=now() WHERE id=$4`, session.ID, session.URL, session.ExpiresAt, dealID)
	if err != nil {
		errOut(w, 503, "could not save payment session; contact support before retrying")
		return
	}
	jsonOut(w, 200, map[string]string{"url": session.URL, "status": "pending"})
}

type stripeSession struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	ExpiresAt int64  `json:"expires_at"`
}

func createStripeSession(ctx context.Context, id, email, site, pkg string, amount int) (stripeSession, error) {
	var result stripeSession
	origin := strings.TrimRight(os.Getenv("SITE_URL"), "/")
	u, e := url.Parse(origin)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Host == "" {
		return result, errors.New("invalid SITE_URL")
	}
	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("client_reference_id", id)
	form.Set("customer_email", email)
	form.Set("line_items[0][quantity]", "1")
	form.Set("line_items[0][price_data][currency]", "usd")
	form.Set("line_items[0][price_data][unit_amount]", strconv.Itoa(amount*100))
	form.Set("line_items[0][price_data][product_data][name]", packageNames[pkg])
	form.Set("metadata[deal_id]", id)
	form.Set("payment_intent_data[metadata][deal_id]", id)
	form.Set("success_url", origin+"/checkout/success?session_id={CHECKOUT_SESSION_ID}")
	form.Set("cancel_url", origin+"/checkout/cancel")
	form.Set("expires_at", strconv.FormatInt(time.Now().Add(35*time.Minute).Unix(), 10))
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.stripe.com/v1/checkout/sessions", strings.NewReader(form.Encode()))
	if e != nil {
		return result, e
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("STRIPE_SECRET_KEY"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Idempotency-Key", "storelift-checkout-"+id)
	client := &http.Client{Timeout: 18 * time.Second}
	resp, e := client.Do(req)
	if e != nil {
		return result, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("stripe HTTP %d", resp.StatusCode)
	}
	if e = json.NewDecoder(io.LimitReader(resp.Body, 131072)).Decode(&result); e != nil {
		return result, e
	}
	if !strings.HasPrefix(result.ID, "cs_") || !safepay.ValidCheckoutURL(result.URL) || result.ExpiresAt <= time.Now().Unix() {
		return stripeSession{}, errors.New("invalid checkout session")
	}
	return result, nil
}

type stripeWebhookEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object struct {
			ID                string `json:"id"`
			ClientReferenceID string `json:"client_reference_id"`
			PaymentStatus     string `json:"payment_status"`
			AmountTotal       int    `json:"amount_total"`
			Currency          string `json:"currency"`
			PaymentIntent     string `json:"payment_intent"`
			Refunded          bool   `json:"refunded"`
			AmountRefunded    int    `json:"amount_refunded"`
			Amount            int    `json:"amount"`
		} `json:"object"`
	} `json:"data"`
}

func (a *app) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	if a.db == nil || os.Getenv("STRIPE_WEBHOOK_SECRET") == "" {
		errOut(w, 503, "payment webhook not configured")
		return
	}
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		errOut(w, 415, "JSON required")
		return
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if e != nil {
		errOut(w, 413, "payload too large")
		return
	}
	if !safepay.VerifyStripeSignature(raw, r.Header.Get("Stripe-Signature"), os.Getenv("STRIPE_WEBHOOK_SECRET"), time.Now()) {
		errOut(w, 400, "invalid signature")
		return
	}
	var event stripeWebhookEvent
	if json.Unmarshal(raw, &event) != nil || event.ID == "" || event.Type == "" {
		errOut(w, 400, "invalid event")
		return
	}
	switch event.Type {
	case "checkout.session.completed", "checkout.session.async_payment_succeeded", "checkout.session.async_payment_failed", "checkout.session.expired":
	case "charge.refunded":
		a.stripeRefund(w, r, event)
		return
	default:
		jsonOut(w, 200, map[string]bool{"ignored": true})
		return
	}
	obj := event.Data.Object
	if !uuidValid(obj.ClientReferenceID) || !strings.HasPrefix(obj.ID, "cs_") {
		errOut(w, 400, "invalid checkout reference")
		return
	}
	if event.Type == "checkout.session.completed" && obj.PaymentStatus != "paid" {
		jsonOut(w, 200, map[string]bool{"pending": true})
		return
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		errOut(w, 503, "database unavailable")
		return
	}
	defer tx.Rollback()
	var expectedAmount int
	var paymentState string
	e = tx.QueryRowContext(r.Context(), `SELECT amount_usd,payment_status FROM deals WHERE id=$1 AND stripe_session_id=$2 FOR UPDATE`, obj.ClientReferenceID, obj.ID).Scan(&expectedAmount, &paymentState)
	if e != nil {
		errOut(w, 409, "checkout session not recognized")
		return
	}
	if obj.AmountTotal != expectedAmount*100 || !strings.EqualFold(obj.Currency, "usd") {
		errOut(w, 409, "payment amount or currency mismatch")
		return
	}
	command, e := tx.ExecContext(r.Context(), `INSERT INTO payment_events(id,event_type) VALUES($1,$2) ON CONFLICT (id) DO NOTHING`, event.ID, event.Type)
	if e != nil {
		errOut(w, 503, "event store unavailable")
		return
	}
	n, _ := command.RowsAffected()
	if n == 0 {
		jsonOut(w, 200, map[string]bool{"duplicate": true})
		return
	}
	next := "pending"
	switch event.Type {
	case "checkout.session.completed", "checkout.session.async_payment_succeeded":
		next = "paid"
	case "checkout.session.async_payment_failed", "checkout.session.expired":
		next = "unpaid"
	}
	// A delayed/duplicate failed event must never revert a confirmed paid deal.
	if paymentState == "paid" && next != "paid" {
		next = "paid"
	}
	if paymentState == "refunded" {
		next = "refunded"
	}
	_, e = tx.ExecContext(r.Context(), `UPDATE deals SET payment_status=$1,stripe_payment_intent=$2,updated_at=now() WHERE id=$3`, next, obj.PaymentIntent, obj.ClientReferenceID)
	if e != nil {
		errOut(w, 503, "payment persistence failed")
		return
	}
	if e = tx.Commit(); e != nil {
		errOut(w, 503, "payment transaction not committed")
		return
	}
	jsonOut(w, 200, map[string]bool{"processed": true})
}

// Refund updates are also webhook-only. Partial refunds are tracked separately,
// and the purchase is only marked refunded when the entire charge is refunded.
func (a *app) stripeRefund(w http.ResponseWriter, r *http.Request, event stripeWebhookEvent) {
	o := event.Data.Object
	if o.PaymentIntent == "" || !strings.EqualFold(o.Currency, "usd") || o.Amount <= 0 || o.AmountRefunded < 0 || o.AmountRefunded > o.Amount {
		errOut(w, 400, "invalid refund details")
		return
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		errOut(w, 503, "database unavailable")
		return
	}
	defer tx.Rollback()
	var id string
	var total int
	e = tx.QueryRowContext(r.Context(), `SELECT id::text,amount_usd FROM deals WHERE stripe_payment_intent=$1 AND payment_status IN ('paid','refunded') FOR UPDATE`, o.PaymentIntent).Scan(&id, &total)
	if e != nil {
		errOut(w, 409, "refund not linked to a confirmed payment")
		return
	}
	if o.Amount != total*100 {
		errOut(w, 409, "refund amount mismatch")
		return
	}
	command, e := tx.ExecContext(r.Context(), `INSERT INTO payment_events(id,event_type) VALUES($1,$2) ON CONFLICT DO NOTHING`, event.ID, event.Type)
	if e != nil {
		errOut(w, 503, "payment event store unavailable")
		return
	}
	rows, _ := command.RowsAffected()
	if rows == 0 {
		jsonOut(w, 200, map[string]bool{"duplicate": true})
		return
	}
	status := "paid"
	if o.Refunded && o.AmountRefunded == o.Amount {
		status = "refunded"
	}
	_, e = tx.ExecContext(r.Context(), `UPDATE deals SET payment_status=$1,refunded_amount_cents=$2,updated_at=now() WHERE id=$3`, status, o.AmountRefunded, id)
	if e != nil {
		errOut(w, 503, "refund update unavailable")
		return
	}
	if e = tx.Commit(); e != nil {
		errOut(w, 503, "refund transaction failed")
		return
	}
	jsonOut(w, 200, map[string]bool{"processed": true})
}
