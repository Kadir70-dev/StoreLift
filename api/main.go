package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/net/html"
)

type Issue struct {
	Severity       string `json:"severity"`
	Title          string `json:"title"`
	Evidence       string `json:"evidence"`
	Recommendation string `json:"recommendation"`
}
type Page struct {
	URL              string   `json:"url"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	H1               []string `json:"h1"`
	Canonical        string   `json:"canonical"`
	Robots           string   `json:"robots"`
	Images           int      `json:"images"`
	MissingAlt       int      `json:"missingAlt"`
	HasProductSchema bool     `json:"hasProductSchema"`
	HasCartAction    bool     `json:"hasCartAction"`
	HasReturnLink    bool     `json:"hasReturnLink"`
	HasShippingLink  bool     `json:"hasShippingLink"`
	Issues           []Issue  `json:"issues"`
}
type Speed struct {
	Status string   `json:"status"`
	Score  *int     `json:"score,omitempty"`
	Error  string   `json:"error,omitempty"`
	FCP    *float64 `json:"fcpMs,omitempty"`
	LCP    *float64 `json:"lcpMs,omitempty"`
}
type Result struct {
	ID        string   `json:"id"`
	Website   string   `json:"website"`
	AuditedAt string   `json:"auditedAt"`
	Pages     []Page   `json:"pages"`
	Mobile    Speed    `json:"mobile"`
	Desktop   Speed    `json:"desktop"`
	Issues    []Issue  `json:"issues"`
	Score     int      `json:"score"`
	Caveats   []string `json:"caveats"`
}
type app struct {
	db                     *sql.DB
	client                 *http.Client
	key, origin, adminHash string
	mu                     sync.Mutex
	attempts               map[string][]time.Time
	trustedProxies         []netip.Prefix
}

var emailRE = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func main() {
	a := &app{key: os.Getenv("PAGESPEED_API_KEY"), origin: env("WEB_ORIGIN", "http://localhost:3000"), adminHash: os.Getenv("ADMIN_PASSWORD_HASH"), attempts: map[string][]time.Time{}}
	proxies, err := parseTrustedProxies(os.Getenv("TRUSTED_PROXY_CIDRS"))
	if err != nil { log.Fatalf("invalid trusted proxy configuration: %v", err) }
	a.trustedProxies = proxies
	// Remote requests are not automatically followed; every redirect target is independently validated.
	a.client = &http.Client{Timeout: 14 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }, Transport: &http.Transport{Proxy: nil, DialContext: safeDial, MaxIdleConns: 15, IdleConnTimeout: 25 * time.Second, TLSHandshakeTimeout: 6 * time.Second, ResponseHeaderTimeout: 11 * time.Second}}
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			log.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err = db.PingContext(ctx); err != nil {
			log.Fatalf("database unavailable: %v", err)
		}
		a.db = db
	} else {
		log.Print("DATABASE_URL not provided: audits run without persistence")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("POST /api/audits", a.audit)
	mux.HandleFunc("GET /api/audits/{id}", a.getAudit)
	mux.HandleFunc("POST /api/leads", a.lead)
	mux.HandleFunc("POST /api/checkout", a.checkout)
	mux.HandleFunc("POST /api/webhooks/stripe", a.stripeWebhook)
	mux.HandleFunc("POST /api/admin/login", a.login)
	mux.HandleFunc("POST /api/admin/logout", a.logout)
	mux.HandleFunc("GET /api/admin/leads", a.adminLeads)
	mux.HandleFunc("GET /api/admin/deals", a.deals)
	mux.HandleFunc("POST /api/admin/deals", a.newDeal)
	mux.HandleFunc("PATCH /api/admin/deals/{id}", a.updateDeal)
	mux.HandleFunc("POST /api/admin/comparisons", a.comparison)
	mux.HandleFunc("GET /api/admin/comparisons", a.listComparisons)
	mux.HandleFunc("GET /api/admin/audits", a.adminAudits)
	srv := &http.Server{Addr: listenAddress(), Handler: a.cors(a.limit(mux)), ReadHeaderTimeout: 6 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 75 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 16}
	log.Printf("StoreLift listening on %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
// Managed Go hosts commonly inject PORT=10000; Docker/local config may use :8080.
func listenAddress() string {
  raw := env("PORT", ":8080")
  if _, err := strconv.Atoi(raw); err == nil {
    return ":" + raw
  }
  return raw
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func (a *app) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			if origin != a.origin {
				http.Error(w, "origin not allowed", 403)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", a.origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,OPTIONS")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (a *app) limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := requestClientIP(r, a.trustedProxies)
		now := time.Now()
		maxPerMinute := 60
		if r.URL.Path == "/api/admin/login" {
			maxPerMinute = 6
		}
		if r.URL.Path == "/api/audits" {
			maxPerMinute = 8
		}
		if r.URL.Path == "/api/checkout" {
			maxPerMinute = 10
		}
		// The Stripe webhook uses a signed raw payload with idempotency checks.
		// Never throttle legitimate event delivery by provider IP.
		if r.URL.Path == "/api/webhooks/stripe" {
			next.ServeHTTP(w, r)
			return
		}
		if a.db != nil && len(os.Getenv("RATE_LIMIT_SECRET")) >= 32 {
			mac := hmac.New(sha256.New, []byte(os.Getenv("RATE_LIMIT_SECRET")))
			mac.Write([]byte(ip))
			fingerprint := hex.EncodeToString(mac.Sum(nil))
			var count int
			e := a.db.QueryRowContext(r.Context(), `INSERT INTO rate_buckets(ip_hash,route,window_start,hits) VALUES($1,$2,$3,1)
			ON CONFLICT (ip_hash,route,window_start) DO UPDATE SET hits=rate_buckets.hits+1 RETURNING hits`, fingerprint, r.URL.Path, now.UTC().Truncate(time.Minute)).Scan(&count)
			if e != nil {
				errOut(w, 503, "rate limiter unavailable")
				return
			}
			if count > maxPerMinute {
				errOut(w, 429, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		a.mu.Lock()
		past := a.attempts[ip]
		keep := past[:0]
		for _, t := range past {
			if now.Sub(t) < time.Minute {
				keep = append(keep, t)
			}
		}
		if len(keep) >= maxPerMinute {
			a.attempts[ip] = keep
			a.mu.Unlock()
			http.Error(w, "rate limit exceeded", 429)
			return
		}
		a.attempts[ip] = append(keep, now)
		if len(a.attempts) > 10000 {
			for k, v := range a.attempts {
				if len(v) == 0 || now.Sub(v[len(v)-1]) > time.Minute {
					delete(a.attempts, k)
				}
			}
		}
		a.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}
func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func errOut(w http.ResponseWriter, status int, msg string) {
	jsonOut(w, status, map[string]string{"error": msg})
}
func body(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 32768)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("unexpected trailing data")
	}
	return nil
}
func (a *app) health(w http.ResponseWriter, r *http.Request) {
	jsonOut(w, 200, map[string]any{"status": "ok", "db": a.db != nil, "pagespeedConfigured": a.key != "", "checkoutConfigured": stripeConfigured()})
}
func normalize(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) > 250 || len(raw) < 4 {
		return nil, errors.New("invalid URL length")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" || u.Hostname() == "" {
		return nil, errors.New("only public HTTPS domains without custom ports are supported")
	}
	if u.RawQuery != "" {
		u.RawQuery = ""
	}
	if u.Path == "" {
		u.Path = "/"
	}
	if e = validateHost(u.Hostname()); e != nil {
		return nil, e
	}
	return u, nil
}
func validateHost(host string) error {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if net.ParseIP(host) != nil {
		return errors.New("IP addresses are blocked")
	}
	if !strings.Contains(host, ".") || len(host) > 253 {
		return errors.New("public domain required")
	}
	for _, s := range []string{"localhost", "local", "internal", "test", "invalid", "example", "onion"} {
		if host == s || strings.HasSuffix(host, "."+s) {
			return errors.New("restricted hostname")
		}
	}
	if strings.ContainsAny(host, "_@%\\/#") {
		return errors.New("invalid hostname")
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) < 1 || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return errors.New("invalid hostname label")
		}
	}
	return nil
}
func safeDial(ctx context.Context, network, address string) (net.Conn, error) {
	h, p, e := net.SplitHostPort(address)
	if e != nil {
		return nil, e
	}
	if e = validateHost(h); e != nil {
		return nil, e
	}
	if p != "443" {
		return nil, errors.New("only HTTPS port 443")
	}
	ips, e := net.DefaultResolver.LookupNetIP(ctx, "ip", h)
	if e != nil {
		return nil, e
	}
	if len(ips) == 0 {
		return nil, errors.New("no DNS records")
	}
	for _, ip := range ips {
		if !publicIP(ip) {
			return nil, errors.New("domain resolves to restricted IP")
		}
	}
	dialer := net.Dialer{Timeout: 6 * time.Second}
	return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), p))
}
func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() {
		return false
	}
	for _, cidr := range []string{"100.64.0.0/10", "169.254.0.0/16", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4", "2001:db8::/32", "fc00::/7", "fe80::/10"} {
		p := netip.MustParsePrefix(cidr)
		if p.Contains(ip) {
			return false
		}
	}
	return true
}
func (a *app) fetch(ctx context.Context, raw string) ([]byte, string, error) {
	u, e := normalize(raw)
	if e != nil {
		return nil, "", e
	}
	for i := 0; i < 3; i++ {
		if e := a.checkRobots(ctx, u); e != nil {
			return nil, "", e
		}
		req, e := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if e != nil {
			return nil, "", e
		}
		req.Header.Set("User-Agent", "StoreLiftAuditBot/0.1 (+public-page-SEO-audit)")
		req.Header.Set("Accept", "text/html")
		resp, e := a.client.Do(req)
		if e != nil {
			return nil, "", e
		}
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			loc := resp.Header.Get("Location")
			resp.Body.Close()
			if loc == "" {
				return nil, "", errors.New("redirect without location")
			}
			next, e := u.Parse(loc)
			if e != nil {
				return nil, "", e
			}
			if strings.TrimPrefix(next.Hostname(), "www.") != strings.TrimPrefix(u.Hostname(), "www.") {
				return nil, "", errors.New("cross-domain redirects are not followed")
			}
			u, e = normalize(next.String())
			if e != nil {
				return nil, "", e
			}
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			return nil, "", fmt.Errorf("page returned HTTP %d", resp.StatusCode)
		}
		if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
			resp.Body.Close()
			return nil, "", errors.New("URL is not an HTML page")
		}
		limited := io.LimitReader(resp.Body, 2<<20+1)
		data, e := io.ReadAll(limited)
		resp.Body.Close()
		if e != nil {
			return nil, "", e
		}
		if len(data) > 2<<20 {
			return nil, "", errors.New("HTML exceeds 2MB")
		}
		return data, u.String(), nil
	}
	return nil, "", errors.New("too many redirects")
}
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return strings.TrimSpace(a.Val)
		}
	}
	return ""
}
func contents(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
			b.WriteString(" ")
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(strings.Join(strings.Fields(b.String()), " "))
}
func parsePage(raw string, doc []byte, product bool) Page {
	p := Page{URL: raw, H1: []string{}, Issues: []Issue{}}
	root, e := html.Parse(strings.NewReader(string(doc)))
	if e != nil {
		return p
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "title":
				if p.Title == "" {
					p.Title = contents(n)
				}
			case "h1":
				p.H1 = append(p.H1, contents(n))
			case "meta":
				name := strings.ToLower(attr(n, "name"))
				if name == "description" && p.Description == "" {
					p.Description = attr(n, "content")
				}
				if name == "robots" {
					p.Robots = attr(n, "content")
				}
			case "link":
				if strings.EqualFold(attr(n, "rel"), "canonical") {
					p.Canonical = attr(n, "href")
				}
			case "img":
				p.Images++
				if attr(n, "alt") == "" {
					p.MissingAlt++
				}
			case "script":
				if strings.EqualFold(attr(n, "type"), "application/ld+json") && strings.Contains(strings.ToLower(contents(n)), `"product"`) {
					p.HasProductSchema = true
				}
			case "form":
				if strings.Contains(strings.ToLower(attr(n, "action")), "/cart/add") {
					p.HasCartAction = true
				}
			case "button":
				if strings.Contains(strings.ToLower(contents(n)), "add to cart") {
					p.HasCartAction = true
				}
			case "a":
				href := strings.ToLower(attr(n, "href"))
				if strings.Contains(href, "return") || strings.Contains(href, "refund") {
					p.HasReturnLink = true
				}
				if strings.Contains(href, "shipping") || strings.Contains(href, "delivery") {
					p.HasShippingLink = true
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	add := func(sev, title, evidence, fix string) { p.Issues = append(p.Issues, Issue{sev, title, evidence, fix}) }
	if p.Title == "" {
		add("high", "Missing page title", "No <title> element found", "Add a unique keyword-relevant HTML title")
	}
	if l := len([]rune(p.Title)); l > 65 {
		add("medium", "Long page title", fmt.Sprintf("%d characters", l), "Write a concise title; preview truncation on SERPs")
	}
	if p.Description == "" {
		add("medium", "Missing meta description", "No meta description found", "Write a unique compelling description")
	}
	if len(p.H1) != 1 {
		add("medium", "Heading structure", fmt.Sprintf("Found %d H1 headings", len(p.H1)), "Use one descriptive primary H1")
	}
	if p.Canonical == "" {
		add("medium", "Missing canonical tag", "No canonical URL detected", "Add a correct canonical link")
	}
	if strings.Contains(strings.ToLower(p.Robots), "noindex") {
		add("high", "Page set to noindex", p.Robots, "Confirm this page should be excluded from search")
	}
	if p.MissingAlt > 0 {
		add("medium", "Missing image alternative text", fmt.Sprintf("%d of %d images missing alt", p.MissingAlt, p.Images), "Add descriptive alt text to informative images; decorative images may use empty alt")
	}
	if product && !p.HasProductSchema {
		add("medium", "Product structured data not detected", "No Product JSON-LD detected in static HTML", "Validate product structured data on the rendered page")
	}
	if product && !p.HasCartAction {
		add("low", "Purchase CTA requires rendered-page review", "No add-to-cart form or button found in static HTML", "Inspect the rendered mobile product page and confirm the purchase CTA is visible and usable")
	}
	if product && (!p.HasShippingLink || !p.HasReturnLink) {
		add("low", "Policy link visibility needs review", "Static page lacks obvious links to one or more shipping/returns policies", "Review visibility of shipping and returns information; links may render client-side")
	}
	return p
}
func (a *app) pageSpeed(ctx context.Context, raw, strategy string) Speed {
	if a.key == "" {
		return Speed{Status: "not_configured", Error: "Set PAGESPEED_API_KEY for measured scores"}
	}
	u, _ := url.Parse("https://pagespeedonline.googleapis.com/pagespeedonline/v5/runPagespeed")
	q := u.Query()
	q.Set("url", raw)
	q.Set("strategy", strategy)
	q.Set("key", a.key)
	q.Add("category", "performance")
	u.RawQuery = q.Encode()
	req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return Speed{Status: "unavailable", Error: "PageSpeed request unavailable"}
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return Speed{Status: "unavailable", Error: fmt.Sprintf("PageSpeed returned HTTP %d", resp.StatusCode)}
	}
	var d struct {
		LighthouseResult struct {
			Categories struct {
				Performance struct {
					Score *float64 `json:"score"`
				} `json:"performance"`
			} `json:"categories"`
			Audits map[string]struct {
				NumericValue *float64 `json:"numericValue"`
			} `json:"audits"`
		} `json:"lighthouseResult"`
	}
	if e = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&d); e != nil {
		return Speed{Status: "unavailable", Error: "PageSpeed response unreadable"}
	}
	s := Speed{Status: "available"}
	if d.LighthouseResult.Categories.Performance.Score != nil {
		v := int(*d.LighthouseResult.Categories.Performance.Score*100 + 0.5)
		s.Score = &v
	}
	if v := d.LighthouseResult.Audits["first-contentful-paint"].NumericValue; v != nil {
		s.FCP = v
	}
	if v := d.LighthouseResult.Audits["largest-contentful-paint"].NumericValue; v != nil {
		s.LCP = v
	}
	return s
}
func makeID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func (a *app) audit(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Website string `json:"website"`
	}
	if body(r, &input) != nil {
		errOut(w, 400, "invalid request")
		return
	}
	u, e := normalize(input.Website)
	if e != nil {
		errOut(w, 400, e.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 48*time.Second)
	defer cancel()
	doc, actual, e := a.fetch(ctx, u.String())
	if e != nil {
		errOut(w, 422, "Could not audit this public page: "+e.Error())
		return
	}
	home := parsePage(actual, doc, false)
	result := Result{ID: makeID(), Website: actual, AuditedAt: time.Now().UTC().Format(time.RFC3339), Pages: []Page{home}, Mobile: Speed{Status: "unavailable"}, Desktop: Speed{Status: "unavailable"}, Caveats: []string{"Audit covers publicly accessible static HTML; JavaScript-rendered SEO and merchant analytics require additional authorized review.", "Conversion friction requires human review; no sales or revenue data was inferred.", "Scores are indicative diagnostics, not a guarantee of traffic or revenue."}}
	// Product page discovery uses only anchors on the original domain, maximum 1 extra page.
	root, _ := html.Parse(strings.NewReader(string(doc)))
	baseURL, _ := url.Parse(actual)
	var productURL string
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if productURL != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			href := attr(n, "href")
			candidate, e := baseURL.Parse(href)
			if e == nil && candidate.Hostname() == baseURL.Hostname() && strings.Contains(candidate.Path, "/products/") {
				if _, e = normalize(candidate.String()); e == nil {
					productURL = candidate.String()
					return
				}
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			visit(ch)
		}
	}
	visit(root)
	if productURL != "" {
		if doc2, final2, e := a.fetch(ctx, productURL); e == nil {
			result.Pages = append(result.Pages, parsePage(final2, doc2, true))
		}
	}
	for _, p := range result.Pages {
		result.Issues = append(result.Issues, p.Issues...)
	}
	result.Score = 100
	for _, i := range result.Issues {
		if i.Severity == "high" {
			result.Score -= 15
		} else {
			result.Score -= 6
		}
	}
	if result.Score < 0 {
		result.Score = 0
	}
	result.Caveats = append(result.Caveats, "Diagnostic score reflects detected HTML checks only; it is not a Google Lighthouse or SEO ranking score.")
	if a.key != "" {
		ch := make(chan struct {
			strategy string
			s        Speed
		}, 2)
		for _, st := range []string{"mobile", "desktop"} {
			go func(v string) {
				sctx, done := context.WithTimeout(ctx, 32*time.Second)
				defer done()
				ch <- struct {
					strategy string
					s        Speed
				}{v, a.pageSpeed(sctx, actual, v)}
			}(st)
		}
		for i := 0; i < 2; i++ {
			select {
			case v := <-ch:
				if v.strategy == "mobile" {
					result.Mobile = v.s
				} else {
					result.Desktop = v.s
				}
			case <-ctx.Done():
				result.Mobile = Speed{Status: "unavailable", Error: "timeout"}
				result.Desktop = Speed{Status: "unavailable", Error: "timeout"}
				i = 2
			}
		}
	} else {
		result.Mobile = Speed{Status: "not_configured", Error: "PageSpeed API key not configured"}
		result.Desktop = result.Mobile
	}
	if a.db != nil {
		data, _ := json.Marshal(result)
		_, e = a.db.ExecContext(ctx, "INSERT INTO audits (id,website,result) VALUES ($1,$2,$3)", result.ID, result.Website, data)
		if e != nil {
			log.Printf("audit persistence error: %v", e)
		}
	}
	jsonOut(w, 200, result)
}
func (a *app) getAudit(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		errOut(w, 503, "database not configured")
		return
	}
	id := r.PathValue("id")
	if !uuidValid(id) {
		errOut(w, 400, "invalid ID")
		return
	}
	var raw []byte
	if err := a.db.QueryRowContext(r.Context(), "SELECT result FROM audits WHERE id=$1", id).Scan(&raw); err != nil {
		errOut(w, 404, "audit not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(raw)
}
func uuidValid(s string) bool {
	return regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(s)
}
func (a *app) lead(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Email   string `json:"email"`
		Website string `json:"website"`
		AuditID string `json:"auditId"`
		Package string `json:"package"`
		Consent bool   `json:"consent"`
	}
	if body(r, &v) != nil || len(v.Email) > 254 || !emailRE.MatchString(v.Email) || !v.Consent {
		errOut(w, 400, "valid email and consent required")
		return
	}
	if v.Package == "" {
		v.Package = "fix"
	}
	if v.Package != "fix" && v.Package != "audit" && v.Package != "premium" {
		errOut(w, 400, "invalid package")
		return
	}
	u, e := normalize(v.Website)
	if e != nil {
		errOut(w, 400, e.Error())
		return
	}
	if a.db == nil {
		errOut(w, 503, "database is not configured")
		return
	}
	id := makeID()
	_, e = a.db.ExecContext(r.Context(), "INSERT INTO leads(id,email,website,consent,preferred_package) VALUES($1,$2,$3,true,$4)", id, strings.ToLower(v.Email), u.String(), v.Package)
	if e != nil {
		errOut(w, 500, "could not save lead")
		return
	}
	if uuidValid(v.AuditID) {
		_, _ = a.db.ExecContext(r.Context(), "UPDATE audits SET lead_id=$1 WHERE id=$2 AND lead_id IS NULL", id, v.AuditID)
	}
	jsonOut(w, 201, map[string]string{"id": id, "status": "received"})
}
func (a *app) login(w http.ResponseWriter, r *http.Request) {
	if a.adminHash == "" {
		errOut(w, 503, "admin password hash not configured")
		return
	}
	var v struct {
		Password string `json:"password"`
	}
	if body(r, &v) != nil {
		errOut(w, 400, "invalid request")
		return
	}
	if !verifyPassword(v.Password, a.adminHash) {
		time.Sleep(350 * time.Millisecond)
		errOut(w, 401, "invalid credentials")
		return
	}
	token := make([]byte, 32)
	_, _ = rand.Read(token)
	session := base64.RawURLEncoding.EncodeToString(token) // Sessions stored in DB so logout and expiration are enforceable.
	if a.db == nil {
		errOut(w, 503, "database required for sessions")
		return
	}
	_, e := a.db.ExecContext(r.Context(), "INSERT INTO admin_sessions(token_hash,expires_at) VALUES(encode(sha256($1::bytea),'hex'), now()+interval '8 hours')", session)
	if e != nil {
		errOut(w, 500, "session setup missing; run migration")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "storelift_admin", Value: session, Path: "/api/admin", HttpOnly: true, Secure: strings.HasPrefix(a.origin, "https://"), SameSite: http.SameSiteStrictMode, MaxAge: 28800})
	jsonOut(w, 200, map[string]bool{"ok": true})
}
func (a *app) authorized(r *http.Request) bool {
	if a.db == nil {
		return false
	}
	c, e := r.Cookie("storelift_admin")
	if e != nil || len(c.Value) != 43 {
		return false
	}
	var ok bool
	e = a.db.QueryRowContext(r.Context(), "SELECT EXISTS (SELECT 1 FROM admin_sessions WHERE token_hash=encode(sha256($1::bytea),'hex') AND expires_at>now())", c.Value).Scan(&ok)
	return e == nil && ok
}
func (a *app) guard(w http.ResponseWriter, r *http.Request) bool {
	if !a.authorized(r) {
		errOut(w, 401, "admin login required")
		return false
	}
	return true
}
func (a *app) logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie("storelift_admin"); e == nil && a.db != nil {
		_, _ = a.db.ExecContext(r.Context(), "DELETE FROM admin_sessions WHERE token_hash=encode(sha256($1::bytea),'hex')", c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "storelift_admin", Value: "", Path: "/api/admin", HttpOnly: true, MaxAge: -1})
	jsonOut(w, 200, map[string]bool{"ok": true})
}

// Admin credential format: pbkdf2_sha256$310000$<hex salt>$<hex hash>.
// Only the derived hash is stored in server environment; see cmd/hashpassword.
func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false
	}
	iterations, e := strconv.Atoi(parts[1])
	if e != nil || iterations < 210000 || iterations > 1000000 {
		return false
	}
	salt, e := hex.DecodeString(parts[2])
	if e != nil || len(salt) < 16 {
		return false
	}
	want, e := hex.DecodeString(parts[3])
	if e != nil || len(want) != 32 {
		return false
	}
	got := deriveKey([]byte(password), salt, iterations, 32)
	return subtle.ConstantTimeCompare(got, want) == 1
}
func deriveKey(password, salt []byte, iterations, size int) []byte {
	mac := hmac.New(sha256.New, password)
	mac.Write(salt)
	mac.Write([]byte{0, 0, 0, 1})
	sum := mac.Sum(nil)
	out := make([]byte, len(sum))
	copy(out, sum)
	for n := 1; n < iterations; n++ {
		mac.Reset()
		mac.Write(sum)
		sum = mac.Sum(nil)
		for i := range out {
			out[i] ^= sum[i]
		}
	}
	return out[:size]
}
func (a *app) adminLeads(w http.ResponseWriter, r *http.Request) {
	if !a.guard(w, r) {
		return
	}
	rows, e := a.db.QueryContext(r.Context(), `SELECT l.id::text,l.email,l.website,l.created_at,l.preferred_package,COALESCE((SELECT a.id::text FROM audits a WHERE a.lead_id=l.id ORDER BY a.created_at DESC LIMIT 1),'') FROM leads l ORDER BY l.created_at DESC LIMIT 100`)
	if e != nil {
		errOut(w, 500, "could not load leads")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, email, site, audit, pkg string
		var created time.Time
		if rows.Scan(&id, &email, &site, &created, &pkg, &audit) == nil {
			items = append(items, map[string]any{"id": id, "email": email, "website": site, "createdAt": created, "auditId": audit, "preferredPackage": pkg})
		}
	}
	jsonOut(w, 200, items)
}
func (a *app) deals(w http.ResponseWriter, r *http.Request) {
	if !a.guard(w, r) {
		return
	}
	rows, e := a.db.QueryContext(r.Context(), `SELECT d.id::text,l.email,l.website,d.package,d.amount_usd,d.status,d.payment_status,d.refunded_amount_cents,d.notes,d.created_at FROM deals d JOIN leads l ON l.id=d.lead_id ORDER BY d.created_at DESC LIMIT 100`)
	if e != nil {
		errOut(w, 500, "could not load deals")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, email, website, pkg, status, payment, notes string
		var amt, refunded int
		var created time.Time
		if rows.Scan(&id, &email, &website, &pkg, &amt, &status, &payment, &refunded, &notes, &created) == nil {
			items = append(items, map[string]any{"id": id, "email": email, "website": website, "package": pkg, "amount": amt, "status": status, "paymentStatus": payment, "refundedAmountCents": refunded, "notes": notes, "createdAt": created})
		}
	}
	jsonOut(w, 200, items)
}
func (a *app) newDeal(w http.ResponseWriter, r *http.Request) {
	if !a.guard(w, r) {
		return
	}
	var v struct {
		LeadID  string `json:"leadId"`
		AuditID string `json:"auditId"`
		Package string `json:"package"`
	}
	if body(r, &v) != nil || !uuidValid(v.LeadID) {
		errOut(w, 400, "invalid lead")
		return
	}
	amounts := map[string]int{"audit": 49, "fix": 199, "premium": 499}
	amount, ok := amounts[v.Package]
	if !ok {
		errOut(w, 400, "invalid package")
		return
	}
	id := makeID()
	var e error
	if uuidValid(v.AuditID) {
		_, e = a.db.ExecContext(r.Context(), "INSERT INTO deals(id,lead_id,audit_id,package,amount_usd) VALUES($1,$2,$3,$4,$5)", id, v.LeadID, v.AuditID, v.Package, amount)
	} else {
		_, e = a.db.ExecContext(r.Context(), "INSERT INTO deals(id,lead_id,package,amount_usd) VALUES($1,$2,$3,$4)", id, v.LeadID, v.Package, amount)
	}
	if e != nil {
		errOut(w, 422, "lead not found")
		return
	}
	jsonOut(w, 201, map[string]string{"id": id, "status": "new"})
}
func (a *app) updateDeal(w http.ResponseWriter, r *http.Request) {
	if !a.guard(w, r) {
		return
	}
	id := r.PathValue("id")
	if !uuidValid(id) {
		errOut(w, 400, "invalid ID")
		return
	}
	var v struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if body(r, &v) != nil || len(v.Notes) > 2000 {
		errOut(w, 400, "invalid request")
		return
	}
	statuses := map[string]bool{"new": true, "contacted": true, "qualified": true, "proposal": true, "won": true, "lost": true}
	if !statuses[v.Status] {
		errOut(w, 400, "invalid status")
		return
	}
	_, e := a.db.ExecContext(r.Context(), "UPDATE deals SET status=$1,notes=$2,updated_at=now() WHERE id=$3", v.Status, v.Notes, id)
	if e != nil {
		errOut(w, 500, "update failed")
		return
	}
	jsonOut(w, 200, map[string]bool{"ok": true})
}
