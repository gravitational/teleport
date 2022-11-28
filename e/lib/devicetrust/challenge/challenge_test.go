package challenge_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"testing"

	"github.com/gravitational/teleport/e/lib/devicetrust/challenge"
)

func TestVerify(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048 /* bits */)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	tests := []struct {
		name    string
		privKey crypto.PrivateKey
		pubKey  crypto.PublicKey
	}{
		{
			name:    "ecdsa key",
			privKey: ecKey,
			pubKey:  ecKey.Public(),
		},
		{
			name:    "rsa key",
			privKey: rsaKey,
			pubKey:  rsaKey.Public(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chal, err := challenge.New()
			if err != nil {
				t.Fatalf("New failed: %v", err)
			}

			h := sha256.Sum256(chal)

			var sig []byte
			switch k := test.privKey.(type) {
			case *ecdsa.PrivateKey:
				sig, err = ecdsa.SignASN1(rand.Reader, k, h[:])
			case *rsa.PrivateKey:
				sig, err = rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, h[:])
			}
			if err != nil {
				t.Fatalf("SignASN1 or SignPKCS1v15 failed: %v", err)
			}

			// Verify correct challenge signature.
			if err := challenge.Verify(chal, sig, test.pubKey, crypto.SHA256); err != nil {
				t.Errorf("Verify returned err=%v, want nil", err)
			}

			// Verify bad challenge signature.
			sig = []byte("invalid sig")
			if err := challenge.Verify(chal, sig, test.pubKey, crypto.SHA256); err == nil {
				t.Error("Verify returned nil err, want non-nil")
			}
		})
	}
}
