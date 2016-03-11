package sshutils

import (
	"crypto/md5"
	"fmt"
	"strings"

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
