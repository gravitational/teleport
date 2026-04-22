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

// Package labelhash produces a fixed-length, collision-resistant, DNS-label-safe
// encoding of a (teleport cluster, kube cluster) pair. Used as a routing key in
// SNI so that arbitrarily long Kubernetes cluster names still fit within the
// RFC 1035 63-byte DNS label limit.
package labelhash

import (
	"crypto/sha256"
	"encoding/base32"
)

// encoder is RFC 4648's "Extended Hex Alphabet" with lowercase letters and no
// padding. Its output sorts lexicographically the same as the source bytes
// and is safe for use in DNS labels.
var encoder = base32.NewEncoding("0123456789abcdefghijklmnopqrstuv").WithPadding(base32.NoPadding)

// HashSize is the byte length of the raw hash produced by Hash.
const HashSize = sha256.Size

// EncodedSize is the character length of the output of Encode.
const EncodedSize = HashSize/5*8 + (HashSize%5*8+4)/5

// Hash returns a fixed-length hash of the given (teleport cluster, kube cluster)
// pair. Inputs are individually SHA-256'd before concatenation so that the pair
// is length-separated: this prevents inputs like (a, bc) from colliding with
// (ab, c).
func Hash(teleportCluster, kubeCluster string) [HashSize]byte {
	h := sha256.New()
	var buf [HashSize]byte
	buf = sha256.Sum256([]byte(teleportCluster))
	h.Write(buf[:])
	buf = sha256.Sum256([]byte(kubeCluster))
	h.Write(buf[:])
	h.Sum(buf[:0])
	return buf
}

// Encode returns the base32hex-encoded Hash of the given pair, suitable for use
// as (part of) a DNS label.
func Encode(teleportCluster, kubeCluster string) string {
	h := Hash(teleportCluster, kubeCluster)
	return encoder.EncodeToString(h[:])
}

// Decode parses a base32hex-encoded string (as produced by Encode) back into
// the raw Hash bytes.
func Decode(encoded string) ([HashSize]byte, error) {
	var out [HashSize]byte
	_, err := encoder.Decode(out[:], []byte(encoded))
	return out, err
}
