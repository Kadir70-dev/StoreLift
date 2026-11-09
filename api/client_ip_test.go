package main

import (
 "net/http"
 "strings"
 "testing"
)
func TestTrustedProxyParsing(t *testing.T) {
 ok,err:=parseTrustedProxies("127.0.0.1/32, 10.42.0.0/16, ::1/128")
 if err!=nil||len(ok)!=3 {t.Fatalf("unexpected proxies %v %v",ok,err)}
 for _,raw:=range []string{"0.0.0.0/0","::/0","invalid","10.1.1.1","127.0.0.1/32,"} {
  if _,err:=parseTrustedProxies(raw);err==nil {t.Errorf("should reject %q",raw)}
 }
 if p,err:=parseTrustedProxies("");err!=nil||len(p)!=0 {t.Fatal("empty config should disable XFF")}
}
func TestRequestClientIP(t *testing.T) {
 trusted,_:=parseTrustedProxies("127.0.0.1/32, 10.42.0.0/16, ::1/128")
 cases:=[]struct{name,peer,chain,want string}{
 {"direct ignores spoof","198.51.100.10:1234","1.2.3.4","198.51.100.10"},
 {"trusted proxy","127.0.0.1:443","203.0.113.5","203.0.113.5"},
 {"spoofed leftmost ignored","127.0.0.1:443","1.2.3.4, 203.0.113.5","203.0.113.5"},
 {"two trusted hops","127.0.0.1:443","192.0.2.15, 10.42.1.2","192.0.2.15"},
 {"malformed chain fails closed","127.0.0.1:443","garbage, 10.42.1.2","127.0.0.1"},
 {"no header","127.0.0.1:443","","127.0.0.1"},
 {"ipv6","[::1]:443","2001:db8::1","2001:db8::1"},
 {"untrusted ipv6","[2001:db8::2]:443","1.2.3.4","2001:db8::2"},
 {"invalid peer","broken","1.2.3.4","broken"},
 }
 for _,tc:=range cases {t.Run(tc.name,func(t *testing.T){
  r:=&http.Request{RemoteAddr:tc.peer,Header:http.Header{}}
  if tc.chain!="" {r.Header.Set("X-Forwarded-For",tc.chain)}
  if got:=requestClientIP(r,trusted);got!=tc.want {t.Fatalf("got %q want %q",got,tc.want)}
 })}
 r:=&http.Request{RemoteAddr:"127.0.0.1:3333",Header:http.Header{}}
 r.Header.Set("X-Forwarded-For",strings.Repeat("a",1025))
 if got:=requestClientIP(r,trusted);got!="127.0.0.1" {t.Fatalf("oversized: %s",got)}
 if got:=requestClientIP(r,nil);got!="127.0.0.1" {t.Fatalf("unconfigured proxy: %s",got)}
}
