package safepay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestStripeSignature(t *testing.T) {
	now := time.Unix(1794150000, 0)
	payload := []byte(`{"id":"evt_test","type":"checkout.session.completed"}`)
	secret := "whsec_test_do_not_use"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.", now.Unix())))
	mac.Write(payload)
	header := fmt.Sprintf("t=%d,v1=%s", now.Unix(), hex.EncodeToString(mac.Sum(nil)))
	if !VerifyStripeSignature(payload, header, secret, now) {
		t.Fatal("signed payload rejected")
	}
	if VerifyStripeSignature([]byte("{}"), header, secret, now) {
		t.Fatal("modified body accepted")
	}
	if VerifyStripeSignature(payload, header, secret, now.Add(6*time.Minute)) {
		t.Fatal("stale signature accepted")
	}
	if VerifyStripeSignature(payload, strings.Replace(header, "v1=", "v0=", 1), secret, now) {
		t.Fatal("wrong signature version accepted")
	}
}
func TestCheckoutURL(t *testing.T) {
	for _, s := range []string{"http://checkout.stripe.com/a", "https://checkout.stripe.com.evil.example/a", "https://user@checkout.stripe.com/a", "https://other.com/a", "https://checkout.stripe.com:8443/a"} {
		if ValidCheckoutURL(s) {
			t.Fatalf("unsafe checkout URL %s", s)
		}
	}
	if !ValidCheckoutURL("https://checkout.stripe.com/c/pay/cs_test_123") {
		t.Fatal("valid checkout URL rejected")
	}
}
