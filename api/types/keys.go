package types

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/keys"
)

func (p *EncryptionKeyPair) EncryptOAEP(plaintext []byte) ([]byte, error) {
	pub, err := keys.ParsePublicKey(p.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hash := crypto.SHA256
	if p.Hash > 0 {
		hash = crypto.Hash(p.Hash)
	}
	switch pubKey := pub.(type) {
	case *rsa.PublicKey:
		ciphertext, err := rsa.EncryptOAEP(hash.New(), rand.Reader, pubKey, plaintext, nil)
		return ciphertext, trace.Wrap(err)
	default:
		return nil, trace.BadParameter("unsupported encryption public key type %T", pub)
	}
}
