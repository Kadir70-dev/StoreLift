package main

import "testing"

func TestSameStore(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"https://www.example.com/", "https://example.com/products/a", true},
		{"https://example.com", "https://evil-example.com", false},
		{"https://a.example.com", "https://example.com", false},
		{"not a URL", "https://example.com", false},
	}
	for _, c := range cases {
		if got := sameStore(c.a, c.b); got != c.want {
			t.Errorf("sameStore(%q,%q)=%v want %v", c.a, c.b, got, c.want)
		}
	}
}
func TestDeltaRequiresMeasuredScores(t *testing.T) {
	x := 10
	y := 45
	v := scoreDelta(Speed{Status: "available", Score: &x}, Speed{Status: "available", Score: &y})
	if v == nil || *v != 35 {
		t.Fatal("incorrect real measured delta")
	}
	if scoreDelta(Speed{Status: "not_configured"}, Speed{Status: "available", Score: &y}) != nil {
		t.Fatal("invented delta")
	}
}
