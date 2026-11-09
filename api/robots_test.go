package main

import "testing"

func TestRobotsRules(t *testing.T) {
	robots := `User-agent: *
Disallow: /admin
Disallow: /products/private*
Allow: /products/private-sample
User-agent: StoreLiftAuditBot
Disallow: /checkout
Allow: /products/`
	for _, c := range []struct {
		path string
		want bool
	}{{"/checkout", false}, {"/products/abc", true}, {"/admin", true}} {
		if got := robotsAllowed(robots, c.path); got != c.want {
			t.Fatalf("for %q got %v", c.path, got)
		}
	}
	generic := `User-agent: *
Disallow: /private*
Allow: /private-public$
`
	for _, c := range []struct {
		path string
		want bool
	}{{"/private-a", false}, {"/private-public", true}, {"/private-public/child", false}, {"/other", true}} {
		if got := robotsAllowed(generic, c.path); got != c.want {
			t.Errorf("for %q got %v", c.path, got)
		}
	}
}
