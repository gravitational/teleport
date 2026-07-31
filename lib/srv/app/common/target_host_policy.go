/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"net/netip"
	"os"

	"github.com/gravitational/trace"
)

const (
	targetHostPolicyAllow = "allowed_hosts"
	targetHostPolicyDeny  = "denied_hosts"
)

// httpProxyEnvVars are the environment variables consulted by
// net/http.ProxyFromEnvironment to route outbound HTTP(S) traffic through a
// forward proxy.
var httpProxyEnvVars = []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"}

// HTTPProxyConfiguredInEnv returns the name of the first set HTTP(S) proxy
// environment variable, if any.
//
// A forward proxy hides the resolved target IP the policy enforces on (the
// dialer only sees the proxy address), so the two are mutually incompatible and
// the Application Service refuses to start when both are set. Only the variable
// name is returned, never its value, because proxy URLs may embed credentials.
func HTTPProxyConfiguredInEnv() (string, bool) {
	for _, name := range httpProxyEnvVars {
		if os.Getenv(name) != "" {
			return name, true
		}
	}
	return "", false
}

// TargetHostPolicy decides whether a resolved application target IP may be
// dialed, based on allowed or denied IP prefixes. It is a pure decision type;
// dialing and audit emission belong to TargetDialer.
type TargetHostPolicy struct {
	AllowedPrefixes []netip.Prefix
	DeniedPrefixes  []netip.Prefix
}

// Enabled returns true when the policy has any restrictions.
func (p TargetHostPolicy) Enabled() bool {
	return len(p.AllowedPrefixes) != 0 || len(p.DeniedPrefixes) != 0
}

// Check validates the target host policy.
func (p TargetHostPolicy) Check() error {
	if len(p.AllowedPrefixes) != 0 && len(p.DeniedPrefixes) != 0 {
		return trace.BadParameter("allowed target host prefixes and denied target host prefixes are mutually exclusive")
	}
	return nil
}

// mode returns the policy mode name recorded in denied audit events.
func (p TargetHostPolicy) mode() string {
	if len(p.AllowedPrefixes) != 0 {
		return targetHostPolicyAllow
	}
	return targetHostPolicyDeny
}

// blocked reports whether a resolved address is rejected by the policy: it is
// outside the allow list, or inside the deny list.
func (p TargetHostPolicy) blocked(addr netip.Addr) bool {
	switch {
	case len(p.AllowedPrefixes) != 0:
		_, ok := prefixContaining(p.AllowedPrefixes, addr)
		return !ok
	case len(p.DeniedPrefixes) != 0:
		_, ok := prefixContaining(p.DeniedPrefixes, addr)
		return ok
	default:
		return false
	}
}

// deniedPrefix returns the denied prefix that matches addr, or the zero prefix
// when the policy is not in deny mode or nothing matched. It identifies which
// deny rule rejected a target for the audit event. Allow-list denials, caused
// by the absence of a match, have no such prefix.
func (p TargetHostPolicy) deniedPrefix(addr netip.Addr) netip.Prefix {
	prefix, _ := prefixContaining(p.DeniedPrefixes, addr)
	return prefix
}

func prefixContaining(prefixes []netip.Prefix, addr netip.Addr) (netip.Prefix, bool) {
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return prefix, true
		}
	}
	return netip.Prefix{}, false
}
