// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package subca

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"strings"
)

// HashCertificatePublicKey hashes a certificate's public key in the
// standardized Sub CA public key representation.
//
// Equivalent to `HEX(SHA256(cert.RawSubjectPublicKeyInfo))`.
func HashCertificatePublicKey(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	return HashPublicKey(cert.RawSubjectPublicKeyInfo)
}

// HashPublicKey hashes a RawSubjectPublicKeyInfo in the
// standardized Sub CA public key representation.
//
// Equivalent to `HEX(SHA256(rawSubjectPublicKeyInfo))`.
func HashPublicKey(rawSubjectPublicKeyInfo []byte) string {
	sum := sha256.Sum256(rawSubjectPublicKeyInfo)
	return hex.EncodeToString(sum[:])
}

// NormalizePublicKey normalizes user-supplied Sub CA public keys to the same
// form as returned by [HashCertificatePublicKey] and [HashPublicKey] (ie, it
// makes it lowercase).
//
// Normalized public keys can be compared directly for equality.
func NormalizePublicKey(pk string) string {
	return strings.ToLower(pk)
}
