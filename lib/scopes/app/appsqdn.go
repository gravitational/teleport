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

package app

import (
	"crypto/sha3"
	"encoding/base32"
	"net"
	"strings"
)

const (
	// scopedPubAddrHashBytes is the number of hash bytes kept before base32
	// encoding. 20 bytes (160 bits) is collision-free for any realistic number
	// of scoped apps.
	scopedPubAddrHashBytes = 20

	// scopedSubdomainLen is the length of an encoded scope/app subdomain: a 20-byte hash
	// base32-encodes to exactly 32 characters.
	scopedSubdomainLen = 32
)

// scopeEncoder is base32 without padding.
var scopedAppEncoder = base32.StdEncoding.WithPadding(base32.NoPadding)

// scopedSubdomain encodes a scoped app's (name, scope) into a single DNS label
func generateScopedSubDomain(appName, scope string) string {
	h := sha3.Sum256([]byte(appName + "\x00" + scope))
	return strings.ToLower(scopedAppEncoder.EncodeToString(h[:scopedPubAddrHashBytes]))
}

// scopeSubdomainOk reports whether the subdomain is a valid string produced by
// ScopedSubdomain: exactly 32 lowercase base32 characters (a-z, 2-7).
func scopeSubdomainOk(subdomain string) bool {
	if len(subdomain) != scopedSubdomainLen {
		return false
	}
	for _, r := range subdomain {
		if (r < 'a' || r > 'z') && (r < '2' || r > '7') {
			return false
		}
	}
	return true
}

// ScopedSubdomain returns the scope-qualified subdomain for a scoped app.
func ScopedSubdomain(appName, scope string) string {
	return generateScopedSubDomain(appName, scope)
}

// ScopedAppPublicAddr returns the derived public address for a scoped app (Scope Qualified SubDomain):
// "<ScopedSubdomain(name, scope)>.<proxy>".
// The trailing port on the proxy is stripped and the host is lowercased so the result
// is a valid public address.
func ScopedAppPublicAddr(scope, appName, localProxyDNSName string) string {
	if host, _, err := net.SplitHostPort(localProxyDNSName); err == nil {
		localProxyDNSName = host
	}
	return generateScopedSubDomain(appName, scope) + "." + strings.ToLower(localProxyDNSName)
}

// ScopedAppPublicAddrValid reports whether publicAddr is a valid derived address
// for the scoped app (appName, scope).
func ScopedAppPublicAddrValid(scope, appName, publicAddr string) bool {
	parts := strings.Split(strings.TrimSuffix(publicAddr, "."), ".")
	if len(parts) < 2 {
		return false
	}

	if !scopeSubdomainOk(parts[0]) {
		return false
	}
	subdomain := parts[0]
	return subdomain == generateScopedSubDomain(appName, scope)
}
