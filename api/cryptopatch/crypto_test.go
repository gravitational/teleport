package cryptopatch

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"
)

func TestKeygenTimes(t *testing.T) {
	const count = 100

	t0 := time.Now()
	for range count {
		_, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Errorf("rsa.GenerateKey failed, error: %v", err)
		}
	}
	t.Logf("generating %d RSA keys took %v", count, time.Since(t0))

	t0 = time.Now()
	for range count {
		_, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Errorf("ecdsa.GenerateKey failed, error: %v", err)
		}
	}
	t.Logf("generating %d ECDSA keys took %v", count, time.Since(t0))

	t0 = time.Now()
	for range count {
		_, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Errorf("ed25519.GenerateKey failed, error: %v", err)
		}
	}
	t.Logf("generating %d ed25519 keys took %v", count, time.Since(t0))

	t.Error("failing test to print output")
}
