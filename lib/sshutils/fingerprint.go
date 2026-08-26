/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package sshutils

import (
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// Fingerprint returns SSH RFC4716 fingerprint of the key
func Fingerprint(key ssh.PublicKey) string {
	return ssh.FingerprintSHA256(key)
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

// fingerprintPrefix is the fingerprint prefix added by ssh.FingerprintSHA256.
const fingerprintPrefix = "SHA256:"

func maybeAddPrefix(fingerprint string) string {
	if !strings.HasPrefix(fingerprint, fingerprintPrefix) {
		return fingerprintPrefix + fingerprint
	}
	return fingerprint
}

// EqualFingerprints checks if two finger prints are equal.
func EqualFingerprints(a, b string) bool {
	return strings.EqualFold(maybeAddPrefix(a), maybeAddPrefix(b))
}
