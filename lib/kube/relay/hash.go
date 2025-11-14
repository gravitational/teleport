// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package relay

import (
	"crypto/sha256"
	"encoding/base32"
)

// base32hex is the "Extended Hex Alphabet" defined in RFC 4648 but with
// lowercase letters and no padding.
var base32hex = base32.NewEncoding("0123456789abcdefghijklmnopqrstuv").WithPadding(base32.NoPadding)

const (
	hashLen = sha256.Size
	// encodedHashLen is the length of the base32 encoding of [hashLen] bytes,
	// without padding.
	encodedHashLen = hashLen/5*8 + (hashLen%5*8+4)/5
)

func hashForTarget(teleportClusterName, kubeClusterName string) [hashLen]byte {
	h := sha256.New()
	var buf [hashLen]byte
	buf = sha256.Sum256([]byte(teleportClusterName))
	h.Write(buf[:])
	buf = sha256.Sum256([]byte(kubeClusterName))
	h.Write(buf[:])
	h.Sum(buf[:0])
	return buf
}

// SNILabelForKubeCluster returns the domain label used in front of [SNISuffix]
// to identify a Kubernetes cluster as the target for a passively routed Relay
// connection. It consists of [SNIPrefixForKubeCluster] ("cluster-") followed by
// the base32hex encoding of the SHA256 hash of Teleport cluster name and
// Kubernetes cluster name, for a total of 60 ASCII lowercase characters.
func SNILabelForKubeCluster(teleportClusterName, kubeClusterName string) string {
	h := hashForTarget(teleportClusterName, kubeClusterName)
	return SNIPrefixForKubeCluster + base32hex.EncodeToString(h[:])
}
