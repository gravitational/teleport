package sshutils

import (
	"fmt"

	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

func NewHostSigner(key, cert []byte) (ssh.Signer, error) {
	hostSigner, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse host private key, err: %v", err)
	}

	hostCAKey, _, _, _, err := ssh.ParseAuthorizedKey(cert)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server CA certificate '%v', err: %v", string(cert), err)
	}

	hostCert, ok := hostCAKey.(*ssh.Certificate)
	if !ok {
		return nil, fmt.Errorf("expected host CA certificate, got %T ", hostCAKey)
	}

	signer, err := ssh.NewCertSigner(hostCert, hostSigner)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate signer, err: %v", err)
	}
	return signer, nil
}
