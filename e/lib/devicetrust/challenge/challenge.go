package challenge

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"

	"github.com/gravitational/trace"
)

const challengeLength = 32

// New creates a new Device Trust challenge with the default length.
func New() ([]byte, error) {
	b := make([]byte, challengeLength)
	if _, err := rand.Read(b); err != nil {
		return nil, trace.Wrap(err)
	}
	return b, nil
}

// Verify verifies that [sig] is a signature over [chal] by the private key
// corresponding to [pubKey].
func Verify(chal, sig []byte, pubKey crypto.PublicKey, hash crypto.Hash) error {
	hasher := hash.New()
	if _, err := hasher.Write(chal); err != nil {
		return trace.Wrap(err)
	}
	digest := hasher.Sum(nil)

	switch pub := pubKey.(type) {
	case *ecdsa.PublicKey:
		if !ecdsa.VerifyASN1(pub, digest, sig) {
			return errors.New("ecdsa verification failed")
		}
		return nil

	case *rsa.PublicKey:
		return trace.Wrap(rsa.VerifyPKCS1v15(pub, hash, digest, sig))

	default:
		return fmt.Errorf("unsupported key type: %T", pub)
	}
}
