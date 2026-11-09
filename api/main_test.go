package main

import (
	"fmt"
	"net/netip"
	"strings"
	"testing"
)

func TestURLSecurity(t *testing.T) {
	bad := []string{"http://shopify.com", "https://localhost", "https://127.0.0.1", "https://169.254.169.254", "https://foo.internal", "https://public.com:8443", "https://user:pass@shop.com", "https://shop.com@localhost"}
	for _, v := range bad {
		if _, e := normalize(v); e == nil {
			t.Errorf("accepted %s", v)
		}
	}
	if _, e := normalize("example-shop.com"); e != nil {
		t.Fatal(e)
	}
}
func TestIP(t *testing.T) {
	for _, v := range []string{"127.0.0.1", "10.1.1.1", "169.254.169.254", "100.64.0.1", "::1", "fc00::1", "2001:db8::1"} {
		if publicIP(netip.MustParseAddr(v)) {
			t.Errorf("accepted %s", v)
		}
	}
	if !publicIP(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("blocked public address")
	}
}
func TestParse(t *testing.T) {
	p := parsePage("https://shop.example/products/cap", []byte(`<html><head><title>Cap</title><meta name="description" content="Nice cap"><link rel="canonical" href="https://shop.example/products/cap"></head><body><h1>Cap</h1><img src="x" alt="cap"><script type="application/ld+json">{"@type":"Product"}</script><form action="/cart/add"><button type="submit">Add to Cart</button></form><a href="/shipping">Shipping</a><a href="/returns">Return policy</a></body></html>`), true)
	if len(p.Issues) != 0 {
		t.Fatalf("unexpected issues %#v", p.Issues)
	}
	p = parsePage("https://example.com", []byte(strings.Repeat("x", 10)), false)
	if len(p.Issues) == 0 {
		t.Fatal("missing issues")
	}
}

func TestPasswordDerivedHash(t *testing.T) {
	salt := []byte("independent-test-salt-12345")
	hash := deriveKey([]byte("test-admin-long-password-2026!"), salt, 210000, 32)
	encoded := "pbkdf2_sha256$210000$" + fmt.Sprintf("%x", salt) + "$" + fmt.Sprintf("%x", hash)
	if !verifyPassword("test-admin-long-password-2026!", encoded) {
		t.Fatal("valid password rejected")
	}
	if verifyPassword("incorrect-password", encoded) {
		t.Fatal("incorrect password accepted")
	}
}
