package sshutils

import (
	"crypto/md5"
	"fmt"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// Fingerprint returns SSH RFC4716 fingerprint of the key
func Fingerprint(key ssh.PublicKey) string {
	sum := md5.Sum(key.Marshal())
	parts := make([]string, len(sum))
	for i := 0; i < len(sum); i++ {
		parts[i] = fmt.Sprintf("%0.2x", sum[i])
	}
	return strings.Join(parts, ":")
}

// AuthorizedKeyFingerprint returns fingerprint from public key
// in authorized key format
func AuthorizedKeyFingerprint(publicKey []byte) (string, error) {
	key, _, _, _, err := ssh.ParseAuthorizedKey(publicKey)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return Fingerprint(key), nil
}

// PrivateKeyFingerprint returns fingerprint of the public key
// extracted from the PEM encoded private key
func PrivateKeyFingerprint(keyBytes []byte) (string, error) {
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return Fingerprint(signer.PublicKey()), nil
}
