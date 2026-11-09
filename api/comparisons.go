package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type AuditListing struct {
	ID        string    `json:"id"`
	Website   string    `json:"website"`
	CreatedAt time.Time `json:"createdAt"`
	Result    Result    `json:"result"`
}
type ComparisonView struct {
	ID           string    `json:"id"`
	DealID       string    `json:"dealId"`
	Store        string    `json:"store"`
	CreatedAt    time.Time `json:"createdAt"`
	Before       Result    `json:"before"`
	After        Result    `json:"after"`
	Delta        int       `json:"diagnosticDelta"`
	IssueDelta   int       `json:"issueDelta"`
	MobileDelta  *int      `json:"mobileDelta,omitempty"`
	DesktopDelta *int      `json:"desktopDelta,omitempty"`
}

func scoreDelta(a, b Speed) *int {
	if a.Status != "available" || b.Status != "available" || a.Score == nil || b.Score == nil {
		return nil
	}
	d := *b.Score - *a.Score
	return &d
}
func sameStore(a, b string) bool {
	x, e := url.Parse(a)
	if e != nil {
		return false
	}
	y, e := url.Parse(b)
	if e != nil {
		return false
	}
	return strings.EqualFold(strings.TrimPrefix(x.Hostname(), "www."), strings.TrimPrefix(y.Hostname(), "www."))
}
func parseAudit(raw []byte) (Result, error) { var v Result; e := json.Unmarshal(raw, &v); return v, e }
func (a *app) adminAudits(w http.ResponseWriter, r *http.Request) {
	if !a.guard(w, r) {
		return
	}
	rows, e := a.db.QueryContext(r.Context(), `SELECT id::text,website,created_at,result FROM audits ORDER BY created_at DESC LIMIT 200`)
	if e != nil {
		errOut(w, 503, "audit history unavailable")
		return
	}
	defer rows.Close()
	out := []AuditListing{}
	for rows.Next() {
		var v AuditListing
		var raw []byte
		if rows.Scan(&v.ID, &v.Website, &v.CreatedAt, &raw) != nil {
			continue
		}
		v.Result, _ = parseAudit(raw)
		out = append(out, v)
	}
	if e = rows.Err(); e != nil {
		errOut(w, 503, "audit history unavailable")
		return
	}
	jsonOut(w, 200, out)
}
func (a *app) comparison(w http.ResponseWriter, r *http.Request) {
	if !a.guard(w, r) {
		return
	}
	var input struct {
		DealID   string `json:"dealId"`
		BeforeID string `json:"beforeId"`
		AfterID  string `json:"afterId"`
	}
	if body(r, &input) != nil || !uuidValid(input.DealID) || !uuidValid(input.BeforeID) || !uuidValid(input.AfterID) || input.BeforeID == input.AfterID {
		errOut(w, 400, "invalid comparison IDs")
		return
	}
	var store string
	var beforeRaw, afterRaw []byte
	var beforeAt, afterAt time.Time
	e := a.db.QueryRowContext(r.Context(), `SELECT l.website, b.result, c.result, b.created_at, c.created_at
 FROM deals d JOIN leads l ON l.id=d.lead_id
 JOIN audits b ON b.id=$2 JOIN audits c ON c.id=$3
 WHERE d.id=$1`, input.DealID, input.BeforeID, input.AfterID).Scan(&store, &beforeRaw, &afterRaw, &beforeAt, &afterAt)
	if e != nil {
		errOut(w, 422, "deal or audit not found")
		return
	}
	before, be := parseAudit(beforeRaw)
	after, ae := parseAudit(afterRaw)
	if be != nil || ae != nil || !sameStore(store, before.Website) || !sameStore(store, after.Website) || !beforeAt.Before(afterAt) {
		errOut(w, 422, "audits must cover the same store and the after scan must be newer")
		return
	}
	id := makeID()
	_, e = a.db.ExecContext(r.Context(), `INSERT INTO audit_comparisons(id,deal_id,before_audit_id,after_audit_id) VALUES($1,$2,$3,$4)`, id, input.DealID, input.BeforeID, input.AfterID)
	if e != nil {
		errOut(w, 422, "could not create comparison")
		return
	}
	jsonOut(w, 201, map[string]string{"id": id})
}
func (a *app) listComparisons(w http.ResponseWriter, r *http.Request) {
	if !a.guard(w, r) {
		return
	}
	rows, e := a.db.QueryContext(r.Context(), `SELECT c.id::text,c.deal_id::text,l.website,c.created_at,b.result,af.result
 FROM audit_comparisons c JOIN deals d ON d.id=c.deal_id JOIN leads l ON l.id=d.lead_id
 JOIN audits b ON b.id=c.before_audit_id JOIN audits af ON af.id=c.after_audit_id
 ORDER BY c.created_at DESC LIMIT 100`)
	if e != nil {
		errOut(w, 503, "comparisons unavailable")
		return
	}
	defer rows.Close()
	views := []ComparisonView{}
	for rows.Next() {
		var v ComparisonView
		var br, ar []byte
		if rows.Scan(&v.ID, &v.DealID, &v.Store, &v.CreatedAt, &br, &ar) != nil {
			continue
		}
		v.Before, _ = parseAudit(br)
		v.After, _ = parseAudit(ar)
		v.Delta = v.After.Score - v.Before.Score
		v.IssueDelta = len(v.After.Issues) - len(v.Before.Issues)
		v.MobileDelta = scoreDelta(v.Before.Mobile, v.After.Mobile)
		v.DesktopDelta = scoreDelta(v.Before.Desktop, v.After.Desktop)
		views = append(views, v)
	}
	if e = rows.Err(); e != nil && e != sql.ErrNoRows {
		errOut(w, 503, "comparisons unavailable")
		return
	}
	jsonOut(w, 200, views)
}
