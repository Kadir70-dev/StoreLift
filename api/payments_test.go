package main

import (
	"testing"
)

func TestFixedPackagePrices(t *testing.T) {
	if packagePrices["audit"] != 49 || packagePrices["fix"] != 199 || packagePrices["premium"] != 499 {
		t.Fatal("pricing drift")
	}
}
