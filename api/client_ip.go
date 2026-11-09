package main

import (
 "fmt"
 "net"
 "net/http"
 "net/netip"
 "strings"
)

func parseTrustedProxies(raw string) ([]netip.Prefix, error) {
 raw = strings.TrimSpace(raw)
 if raw == "" { return nil, nil }
 var prefixes []netip.Prefix
 for _, part := range strings.Split(raw, ",") {
  prefix, err := netip.ParsePrefix(strings.TrimSpace(part))
  if err != nil { return nil, fmt.Errorf("invalid TRUSTED_PROXY_CIDRS entry: %w", err) }
  prefix = prefix.Masked()
  if prefix.Addr().Is4() && prefix.Bits() < 8 || prefix.Addr().Is6() && prefix.Bits() < 32 {
   return nil, fmt.Errorf("overly broad trusted proxy range %s", prefix)
  }
  prefixes = append(prefixes, prefix)
 }
 return prefixes, nil
}
func isTrustedProxy(addr netip.Addr, trusted []netip.Prefix) bool {
 addr = addr.Unmap()
 for _, p := range trusted { if p.Contains(addr) { return true } }
 return false
}
// requestClientIP walks the X-Forwarded-For chain from the trusted TCP peer
// toward the visitor, stopping at the first untrusted hop. Spoofed leftmost
// addresses therefore cannot override that hop. Malformed chains fail closed.
func requestClientIP(r *http.Request, trusted []netip.Prefix) string {
 host, _, err := net.SplitHostPort(r.RemoteAddr)
 if err != nil { return r.RemoteAddr }
 peer, err := netip.ParseAddr(host)
 if err != nil || peer.Zone() != "" { return r.RemoteAddr }
 peer = peer.Unmap()
 if !isTrustedProxy(peer, trusted) { return peer.String() }
 chain := strings.Join(r.Header.Values("X-Forwarded-For"), ",")
 if chain == "" || len(chain) > 1024 { return peer.String() }
 hops := strings.Split(chain, ",")
 if len(hops) > 20 { return peer.String() }
 addresses := make([]netip.Addr, 0, len(hops))
 for _, item := range hops {
  ip, err := netip.ParseAddr(strings.TrimSpace(item))
  if err != nil || ip.Zone() != "" { return peer.String() }
  addresses = append(addresses, ip.Unmap())
 }
 current := peer
 for i := len(addresses)-1; i >= 0; i-- {
  if !isTrustedProxy(current, trusted) { break }
  current = addresses[i]
 }
 return current.String()
}
