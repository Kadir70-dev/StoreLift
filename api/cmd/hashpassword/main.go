package main

import (
	"bufio"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"golang.org/x/term"
	"os"
	"strings"
)

func main() {
	fmt.Fprint(os.Stderr, "Enter strong admin password (16+ characters): ")
	raw, e := term.ReadPassword(int(os.Stdin.Fd()))
	if e != nil {
		fmt.Fprintln(os.Stderr, "\nCould not read password from terminal:", e)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr)
	if len(raw) < 16 {
		fmt.Fprintln(os.Stderr, "Password must be 16+ characters")
		os.Exit(1)
	}
	salt := make([]byte, 24)
	if _, e = rand.Read(salt); e != nil {
		panic(e)
	}
	mac := hmac.New(sha256.New, raw)
	mac.Write(salt)
	mac.Write([]byte{0, 0, 0, 1})
	sum := mac.Sum(nil)
	out := make([]byte, len(sum))
	copy(out, sum)
	for n := 1; n < 310000; n++ {
		mac.Reset()
		mac.Write(sum)
		sum = mac.Sum(nil)
		for i := range out {
			out[i] ^= sum[i]
		}
	}
	fmt.Printf("ADMIN_PASSWORD_HASH=pbkdf2_sha256$310000$%s$%s\n", hex.EncodeToString(salt), hex.EncodeToString(out))
	for i := range raw {
		raw[i] = 0
	}
	_ = bufio.ErrInvalidUnreadByte
	_ = strings.Builder{}
}
