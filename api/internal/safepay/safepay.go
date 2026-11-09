package safepay

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func ValidCheckoutURL(s string) bool {
	u, e := url.Parse(s)
	return e == nil && u.Scheme == "https" && u.Hostname() == "checkout.stripe.com" && u.User == nil && u.Port() == ""
}

// Stripe signing input = timestamp + '.' + exact raw request body.
// Reject old/future events outside a 5-minute clock-skew tolerance.
func VerifyStripeSignature(payload []byte, header, secret string, now time.Time) bool {
	if secret == "" || len(payload) > 1<<20 {
		return false
	}
	var timestamp int64
	sigs := make([][]byte, 0, 2)
	for _, part := range strings.Split(header, ",") {
		pieces := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pieces) != 2 {
			continue
		}
		if pieces[0] == "t" {
			timestamp, _ = strconv.ParseInt(pieces[1], 10, 64)
		}
		if pieces[0] == "v1" {
			if sig, e := hex.DecodeString(pieces[1]); e == nil && len(sig) == 32 {
				sigs = append(sigs, sig)
			}
		}
	}
	if timestamp < 1 || len(sigs) == 0 {
		return false
	}
	distance := now.Unix() - timestamp
	if distance > 300 || distance < -300 {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10) + "."))
	mac.Write(payload)
	expected := mac.Sum(nil)
	for _, sig := range sigs {
		if subtle.ConstantTimeCompare(sig, expected) == 1 {
			return true
		}
	}
	return false
}
