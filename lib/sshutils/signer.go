package sshutils

import (
	"fmt"

	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

func NewSigner(keyBytes, certBytes []byte) (ssh.Signer, error) {
	keySigner, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse host private key, err: %v", err)
	}

	pubkey, _, _, _, err := ssh.ParseAuthorizedKey(certBytes)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse server CA certificate '%v', err: %v",
			string(certBytes), err)
	}

	cert, ok := pubkey.(*ssh.Certificate)
	if !ok {
		return nil, fmt.Errorf("expected CA certificate, got %T ", pubkey)
	}

	return ssh.NewCertSigner(cert, keySigner)
}
