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
	"context"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/tlsca"
)

const (
	targetHostPolicyAllow = "allowed_hosts"
	targetHostPolicyDeny  = "denied_hosts"

	// targetHostDialKeepAlive mirrors the net.Dialer keep-alive setting used by
	// net/http.DefaultTransport.
	targetHostDialKeepAlive = 30 * time.Second
)

// httpProxyEnvVars are the environment variables consulted by
// net/http.ProxyFromEnvironment to route outbound HTTP(S) traffic through a
// forward proxy.
var httpProxyEnvVars = []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"}

// HTTPProxyConfiguredInEnv returns the name of the first set HTTP(S) proxy
// environment variable, if any.
//
// The target host policy enforces restrictions on the resolved target IP, which
// it cannot do for HTTP and MCP traffic routed through a forward proxy: the only
// address the dialer observes in that case is the proxy's. The two
// configurations are therefore mutually incompatible and the Application Service
// refuses to start when both are set. Only the variable name is returned (not
// its value) because proxy URLs may embed credentials.
func HTTPProxyConfiguredInEnv() (string, bool) {
	for _, name := range httpProxyEnvVars {
		if os.Getenv(name) != "" {
			return name, true
		}
	}
	return "", false
}

// TargetHostResolver resolves hostnames to IP addresses.
type TargetHostResolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// TargetHostPolicy restricts application target dials to allowed or denied IP
// prefixes.
type TargetHostPolicy struct {
	AllowedPrefixes []netip.Prefix
	DeniedPrefixes  []netip.Prefix

	// Resolver is optional and exists for tests.
	Resolver TargetHostResolver
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

func (p TargetHostPolicy) resolver() TargetHostResolver {
	if p.Resolver != nil {
		return p.Resolver
	}
	return net.DefaultResolver
}

// TargetHostAuditContext contains audit metadata for denied target dials.
type TargetHostAuditContext struct {
	Emitter  apievents.Emitter
	Logger   *slog.Logger
	ServerID string
	Identity *tlsca.Identity
	App      types.Application
}

type targetHostDenial struct {
	Host          string
	Port          string
	ResolvedIPs   []netip.Addr
	Policy        string
	BlockedIP     netip.Addr
	BlockedPrefix netip.Prefix
}

// DialContext dials an application target after applying the target host
// policy to the concrete destination IP.
func (p TargetHostPolicy) DialContext(ctx context.Context, network, address string, audit TargetHostAuditContext) (net.Conn, error) {
	if err := p.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	dialer := net.Dialer{
		Timeout:   apidefaults.DefaultIOTimeout,
		KeepAlive: targetHostDialKeepAlive,
	}
	if !p.Enabled() {
		return dialer.DialContext(ctx, network, address)
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	addrs, err := p.resolve(ctx, network, host)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(addrs) == 0 {
		return nil, trace.NotFound("application target host %q did not resolve to any IP addresses", host)
	}

	allowed, denial := p.filter(host, port, addrs)
	if len(allowed) == 0 {
		if denial != nil {
			p.emitDenied(ctx, audit, *denial)
		}
		return nil, deniedTargetHostError(host)
	}

	var errs []error
	for _, addr := range allowed {
		// Dial checked IP literals to avoid a second hostname resolution. This
		// intentionally trades the default dialer's hostname-level fast fallback for
		// policy enforcement on the exact IPs resolved above.
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(addr.String(), port))
		if err == nil {
			return conn, nil
		}
		errs = append(errs, err)
	}
	return nil, trace.NewAggregate(errs...)
}

func (p TargetHostPolicy) filter(host, port string, addrs []netip.Addr) ([]netip.Addr, *targetHostDenial) {
	switch {
	case len(p.AllowedPrefixes) != 0:
		return p.filterAllowed(host, port, addrs)
	case len(p.DeniedPrefixes) != 0:
		return p.filterDenied(host, port, addrs)
	default:
		return addrs, nil
	}
}

func (p TargetHostPolicy) filterAllowed(host, port string, addrs []netip.Addr) ([]netip.Addr, *targetHostDenial) {
	var allowed []netip.Addr
	for _, addr := range addrs {
		if _, ok := prefixContaining(p.AllowedPrefixes, addr); ok {
			allowed = append(allowed, addr)
		}
	}
	if len(allowed) != 0 {
		return allowed, nil
	}

	denial := &targetHostDenial{
		Host:        host,
		Port:        port,
		ResolvedIPs: addrs,
		Policy:      targetHostPolicyAllow,
	}
	if len(addrs) != 0 {
		denial.BlockedIP = addrs[0]
	}
	return nil, denial
}

func (p TargetHostPolicy) filterDenied(host, port string, addrs []netip.Addr) ([]netip.Addr, *targetHostDenial) {
	var allowed []netip.Addr
	var firstDenial *targetHostDenial
	for _, addr := range addrs {
		prefix, denied := prefixContaining(p.DeniedPrefixes, addr)
		if !denied {
			allowed = append(allowed, addr)
			continue
		}
		if firstDenial == nil {
			firstDenial = &targetHostDenial{
				Host:          host,
				Port:          port,
				ResolvedIPs:   addrs,
				Policy:        targetHostPolicyDeny,
				BlockedIP:     addr,
				BlockedPrefix: prefix,
			}
		}
	}
	if len(allowed) != 0 {
		return allowed, nil
	}
	return nil, firstDenial
}

func (p TargetHostPolicy) resolve(ctx context.Context, network, host string) ([]netip.Addr, error) {
	if addr, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{canonicalAddr(addr)}, nil
	}

	lookupNetwork := "ip"
	switch network {
	case "tcp4", "udp4", "ip4":
		lookupNetwork = "ip4"
	case "tcp6", "udp6", "ip6":
		lookupNetwork = "ip6"
	}
	addrs, err := p.resolver().LookupNetIP(ctx, lookupNetwork, host)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for i := range addrs {
		addrs[i] = canonicalAddr(addrs[i])
	}
	return addrs, nil
}

// canonicalAddr normalizes a resolved address for policy evaluation. It strips
// any IPv6 zone identifier and unmaps IPv4-in-IPv6 addresses. Both are required
// for correct prefix matching: netip.Prefix.Contains always returns false for an
// address carrying a zone, and an IPv4-mapped IPv6 address would not match an
// IPv4 prefix. Without this normalization a target resolving to (or configured
// as) e.g. "fe80::1%eth0" or "::ffff:169.254.169.254" would silently slip past a
// deny rule.
func canonicalAddr(addr netip.Addr) netip.Addr {
	return addr.WithZone("").Unmap()
}

func prefixContaining(prefixes []netip.Prefix, addr netip.Addr) (netip.Prefix, bool) {
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return prefix, true
		}
	}
	return netip.Prefix{}, false
}

func deniedTargetHostError(host string) error {
	return trace.AccessDenied("application target host %q is not permitted by app_service target host policy", host)
}

func (p TargetHostPolicy) emitDenied(ctx context.Context, audit TargetHostAuditContext, denial targetHostDenial) {
	if audit.Emitter == nil || audit.App == nil || audit.Identity == nil {
		return
	}

	event := &apievents.AppSessionTargetDialDenied{
		Metadata: apievents.Metadata{
			Type:        events.AppSessionTargetDialDeniedEvent,
			Code:        events.AppSessionTargetDialDeniedCode,
			ClusterName: audit.Identity.RouteToApp.ClusterName,
		},
		UserMetadata:    audit.Identity.GetUserMetadata(),
		SessionMetadata: targetHostSessionMetadata(audit.Identity),
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion:   teleport.Version,
			ServerID:        audit.ServerID,
			ServerNamespace: apidefaults.Namespace,
		},
		AppMetadata:   *MakeAppMetadata(audit.App),
		TargetHost:    denial.Host,
		TargetPort:    denial.Port,
		Policy:        denial.Policy,
		ResolvedIPs:   stringifyAddrs(denial.ResolvedIPs),
		BlockedIP:     stringifyAddr(denial.BlockedIP),
		BlockedPrefix: stringifyPrefix(denial.BlockedPrefix),
	}

	if err := audit.Emitter.EmitAuditEvent(ctx, event); err != nil && audit.Logger != nil {
		audit.Logger.WarnContext(ctx, "Failed to emit app target dial denied audit event.", "error", err)
	}
}

func targetHostSessionMetadata(identity *tlsca.Identity) apievents.SessionMetadata {
	return apievents.SessionMetadata{
		SessionID:        identity.RouteToApp.SessionID,
		WithMFA:          identity.MFAVerified,
		PrivateKeyPolicy: string(identity.PrivateKeyPolicy),
	}
}

func stringifyAddrs(addrs []netip.Addr) []string {
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, stringifyAddr(addr))
	}
	return out
}

func stringifyAddr(addr netip.Addr) string {
	if !addr.IsValid() {
		return ""
	}
	return addr.String()
}

func stringifyPrefix(prefix netip.Prefix) string {
	if !prefix.IsValid() {
		return ""
	}
	return prefix.String()
}
