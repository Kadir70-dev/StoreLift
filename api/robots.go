package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"storelift/api/internal/robotspolicy"
)

// This basic RFC 9309-compatible subset supports user-agent groups, allow/disallow,
// wildcards and end anchors. Unreadable robots policies fail closed.
// The parser deliberately implements a subset; review live merchant policies before release.
func robotsAllowed(content, path string) bool { return robotspolicy.Allowed(content, path) }
func (a *app) checkRobots(ctx context.Context, u *url.URL) error {
	target := *u
	target.Path = "/robots.txt"
	target.RawPath = ""
	target.RawQuery = ""
	target.Fragment = ""
	req, e := http.NewRequestWithContext(ctx, "GET", target.String(), nil)
	if e != nil {
		return e
	}
	req.Header.Set("User-Agent", "StoreLiftAuditBot/1.0 (+public-storefront-audit)")
	resp, e := a.client.Do(req)
	if e != nil {
		return errors.New("could not verify robots.txt")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 || resp.StatusCode == 410 {
		return nil
	}
	if resp.StatusCode != 200 {
		return errors.New("robots.txt does not permit a reliable audit")
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, 128*1024+1))
	if e != nil || len(raw) > 128*1024 {
		return errors.New("robots.txt unreadable or too large")
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if !robotsAllowed(string(raw), path) {
		return errors.New("the page is disallowed by robots.txt")
	}
	return nil
}
