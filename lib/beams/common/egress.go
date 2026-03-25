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

package beamscommon

import (
	"net"
	"strings"
)

// EgressHostPortAllowed reports whether the given hostPort is permitted by the
// allowed domains list from a beam spec.
//
// Matching rules:
//   - "example.com"        → any port on example.com
//   - "example.com:22"     → only port 22 on example.com
//   - "*.example.com"      → any port on any subdomain (including nested) of example.com
//   - "*.example.com:443"  → port 443 only on any subdomain (including nested) of example.com
func EgressHostPortAllowed(allowedDomains []string, hostPort string) bool {
	reqHost, reqPort, err := net.SplitHostPort(hostPort)
	if err != nil {
		return false
	}
	for _, allowed := range allowedDomains {
		if egressDomainMatches(allowed, reqHost, reqPort) {
			return true
		}
	}
	return false
}

// EgressFQDNAllowed reports whether a bare hostname (no port) matches any
// entry in the allowed domains list, ignoring port constraints.
// Used during DNS resolution where the port is not yet known.
//
// Stored allowed domains may have a trailing dot (e.g. "example.com.") since
// tsh beam allow calls fullyQualify before saving — both sides are normalized.
func EgressFQDNAllowed(allowedDomains []string, fqdn string) bool {
	// Normalize input: strip trailing dot from fully-qualified DNS names.
	fqdn = strings.TrimSuffix(fqdn, ".")
	for _, allowed := range allowedDomains {
		allowedHost := normalizeDomain(allowed)
		if h, _, err := net.SplitHostPort(allowed); err == nil {
			allowedHost = normalizeDomain(h)
		}
		if strings.HasPrefix(allowedHost, "*.") {
			suffix := allowedHost[1:] // ".example.com"
			if strings.HasSuffix(fqdn, suffix) {
				return true
			}
		} else if fqdn == allowedHost {
			return true
		}
	}
	return false
}

// normalizeDomain strips any trailing dot from a domain name so that stored
// FQDNs (e.g. "example.com.") compare equal to bare hostnames ("example.com").
func normalizeDomain(d string) string {
	return strings.TrimSuffix(d, ".")
}

func egressDomainMatches(allowed, reqHost, reqPort string) bool {
	var allowedHost, allowedPort string
	if h, p, err := net.SplitHostPort(allowed); err == nil {
		allowedHost, allowedPort = normalizeDomain(h), p
	} else {
		// No port specified — any port is allowed.
		allowedHost = normalizeDomain(allowed)
	}

	// Check port constraint first.
	if allowedPort != "" && allowedPort != reqPort {
		return false
	}

	// Match host with optional wildcard prefix.
	// "*.example.com" matches "foo.example.com" and "foo.bar.example.com".
	if strings.HasPrefix(allowedHost, "*.") {
		suffix := allowedHost[1:] // ".example.com"
		return strings.HasSuffix(reqHost, suffix)
	}
	return reqHost == allowedHost
}
