package main

import "testing"

func TestManagedHostPort(t *testing.T) {
    t.Setenv("PORT", "10000")
    if got := listenAddress(); got != ":10000" {
        t.Fatalf("numeric managed hosting port: %q", got)
    }
    t.Setenv("PORT", ":8080")
    if got := listenAddress(); got != ":8080" {
        t.Fatalf("local Go address: %q", got)
    }
}
